// Package cache 进程内缓存（设备端零外部依赖方案）。
//
// 平台原先依赖 Redis 承担「会话 / 在线用户 / 验证码 / 限流 / 权限缓存 / 租约」，
// 设备是单机单进程部署，这里用内存实现同样的语义：
//   - 字符串读写与 TTL（Set/Get/SetNX/Incr/GetInt）
//   - JSON 读写（SetJSON/GetJSON）与空值防穿透（Remember）
//   - 集合（SetAdd/SetMembers/SetRemove）——用于在线会话
//   - 租约锁（AcquireLock/ExtendLock/ReleaseLock）——用于单机任务的互斥
//
// 代价：进程重启后会话与缓存全部失效（用户需重新登录），对设备场景可接受。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"
)

// Cache 带命名空间前缀的内存缓存。
type Cache struct {
	mu     sync.RWMutex
	prefix string
	items  map[string]*entry
	stopCh chan struct{}
	once   sync.Once
}

type entry struct {
	// val 字符串值；集合类型时为空
	val string
	// set 集合成员（nil 表示该 key 不是集合）
	set map[string]struct{}
	// exp 过期时间，零值表示永不过期
	exp time.Time
}

func (e *entry) expired(now time.Time) bool {
	return !e.exp.IsZero() && now.After(e.exp)
}

// New 创建内存缓存实例，prefix 仅用于日志与调试可读性。
func New(prefix string) *Cache {
	c := &Cache{
		prefix: prefix,
		items:  make(map[string]*entry, 128),
		stopCh: make(chan struct{}),
	}
	go c.janitor()
	return c
}

// Stop 停止后台清理协程。
func (c *Cache) Stop() {
	c.once.Do(func() { close(c.stopCh) })
}

// janitor 周期性清理过期键，避免长期运行内存增长。
func (c *Cache) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case now := <-t.C:
			c.mu.Lock()
			for k, e := range c.items {
				if e.expired(now) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *Cache) key(k string) string { return c.prefix + k }

// Ping 探活（内存实现恒为可用）。
func (c *Cache) Ping(ctx context.Context) error { return nil }

// Get 读取字符串，未命中返回 ("", false)。
func (c *Cache) Get(ctx context.Context, k string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[c.key(k)]
	if !ok || e.expired(time.Now()) || e.set != nil {
		return "", false
	}
	return e.val, true
}

// Set 写入字符串（ttl<=0 表示永不过期）。
func (c *Cache) Set(ctx context.Context, k, v string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := &entry{val: v}
	if ttl > 0 {
		e.exp = time.Now().Add(ttl)
	}
	c.items[c.key(k)] = e
	return nil
}

// SetNX 仅当 key 不存在时写入（幂等/防重）。
func (c *Cache) SetNX(ctx context.Context, k, v string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(k)
	if e, ok := c.items[key]; ok && !e.expired(time.Now()) {
		return false, nil
	}
	e := &entry{val: v}
	if ttl > 0 {
		e.exp = time.Now().Add(ttl)
	}
	c.items[key] = e
	return true, nil
}

// Del 删除一个或多个 key。
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.items, c.key(k))
	}
	return nil
}

// Incr 自增并设置过期时间（首次写入时设置），用于滑动窗口限流。
func (c *Cache) Incr(ctx context.Context, k string, ttl time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(k)
	e, ok := c.items[key]
	now := time.Now()
	if !ok || e.expired(now) || e.set != nil {
		e = &entry{}
		if ttl > 0 {
			e.exp = now.Add(ttl)
		}
		c.items[key] = e
	}
	n, _ := strconv.ParseInt(e.val, 10, 64)
	n++
	e.val = strconv.FormatInt(n, 10)
	return n, nil
}

// GetInt 读取整数计数。
func (c *Cache) GetInt(ctx context.Context, k string) int64 {
	if v, ok := c.Get(ctx, k); ok {
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

// Exists 判断 key 是否存在。
func (c *Cache) Exists(ctx context.Context, k string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[c.key(k)]
	return ok && !e.expired(time.Now())
}

// SetJSON 写入 JSON。
func (c *Cache) SetJSON(ctx context.Context, k string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Set(ctx, k, string(b), ttl)
}

// GetJSON 读取并反序列化，未命中返回 false。
func (c *Cache) GetJSON(ctx context.Context, k string, out any) bool {
	s, ok := c.Get(ctx, k)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(s), out) == nil
}

const emptyPlaceholder = "\x00nil"

// Remember 缓存未命中时回源 loader（带空值缓存防穿透）。
func (c *Cache) Remember(ctx context.Context, k string, ttl time.Duration, loader func() (string, error)) (string, error) {
	if v, ok := c.Get(ctx, k); ok {
		if v == emptyPlaceholder {
			return "", nil
		}
		return v, nil
	}
	v, err := loader()
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if v == "" {
		_ = c.Set(ctx, k, emptyPlaceholder, time.Minute)
		return "", nil
	}
	_ = c.Set(ctx, k, v, jitter(ttl))
	return v, nil
}

// ErrNotFound 回源未命中语义错误。
var ErrNotFound = errors.New("cache: not found")

// AcquireLock 抢占租约（等价 SET NX + TTL）。
func (c *Cache) AcquireLock(ctx context.Context, k, token string, ttl time.Duration) (bool, error) {
	return c.SetNX(ctx, k, token, ttl)
}

// ExtendLock 续期租约：仅当仍由 token 持有时生效。
func (c *Cache) ExtendLock(ctx context.Context, k, token string, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(k)
	e, ok := c.items[key]
	if !ok || e.expired(time.Now()) || e.val != token {
		return false
	}
	e.exp = time.Now().Add(ttl)
	return true
}

// ReleaseLock 释放租约：仅当仍由 token 持有时删除。
func (c *Cache) ReleaseLock(ctx context.Context, k, token string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(k)
	e, ok := c.items[key]
	if !ok || e.val != token {
		return false
	}
	delete(c.items, key)
	return true
}

// SetAdd 集合添加成员。
func (c *Cache) SetAdd(ctx context.Context, k string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(k)
	e, ok := c.items[key]
	if !ok || e.expired(time.Now()) || e.set == nil {
		e = &entry{set: make(map[string]struct{}, len(members))}
		c.items[key] = e
	}
	for _, m := range members {
		e.set[m] = struct{}{}
	}
	return nil
}

// SetMembers 集合成员列表。
func (c *Cache) SetMembers(ctx context.Context, k string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[c.key(k)]
	if !ok || e.expired(time.Now()) || e.set == nil {
		return nil
	}
	out := make([]string, 0, len(e.set))
	for m := range e.set {
		out = append(out, m)
	}
	return out
}

// SetRemove 集合移除成员。
func (c *Cache) SetRemove(ctx context.Context, k string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[c.key(k)]
	if !ok || e.set == nil {
		return nil
	}
	for _, m := range members {
		delete(e.set, m)
	}
	if len(e.set) == 0 {
		delete(c.items, c.key(k))
	}
	return nil
}

// Stats 返回缓存规模（调试与监控用）。
func (c *Cache) Stats() (keys int, approxBytes int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys = len(c.items)
	for k, e := range c.items {
		approxBytes += len(k) + len(e.val)
		for m := range e.set {
			approxBytes += len(m)
		}
	}
	return keys, approxBytes
}

// jitter 给 TTL 加 ±10% 抖动，避免缓存同时到期引发雪崩。
func jitter(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	delta := int64(ttl) / 10
	if delta == 0 {
		return ttl
	}
	return ttl + time.Duration((int64(time.Now().UnixNano())%delta)-(delta/2))
}
