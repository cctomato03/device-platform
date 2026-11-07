// Package cache Redis 缓存封装：统一 key 前缀、JSON 序列化、空值防穿透。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 带命名空间的 Redis 客户端封装。
type Cache struct {
	rdb    *redis.Client
	prefix string
}

func New(rdb *redis.Client, prefix string) *Cache {
	return &Cache{rdb: rdb, prefix: prefix}
}

// Client 暴露原始客户端（限流、分布式锁等场景使用）。
func (c *Cache) Client() *redis.Client { return c.rdb }

func (c *Cache) key(k string) string { return c.prefix + k }

// Ping 探活。
func (c *Cache) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Get 读取字符串，未命中返回 ("", false)。
func (c *Cache) Get(ctx context.Context, k string) (string, bool) {
	v, err := c.rdb.Get(ctx, c.key(k)).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

// Set 写入字符串。
func (c *Cache) Set(ctx context.Context, k, v string, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.key(k), v, ttl).Err()
}

// SetNX 仅当 key 不存在时写入，用于幂等/防重。
func (c *Cache) SetNX(ctx context.Context, k, v string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, c.key(k), v, ttl).Result()
}

// Del 删除一个或多个 key。
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	real := make([]string, 0, len(keys))
	for _, k := range keys {
		real = append(real, c.key(k))
	}
	return c.rdb.Del(ctx, real...).Err()
}

// Incr 自增并设置过期时间（首次写入时设置）。
func (c *Cache) Incr(ctx context.Context, k string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, c.key(k))
	pipe.Expire(ctx, c.key(k), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// GetInt 读取整数计数。
func (c *Cache) GetInt(ctx context.Context, k string) int64 {
	n, err := c.rdb.Get(ctx, c.key(k)).Int64()
	if err != nil {
		return 0
	}
	return n
}

// Exists 判断 key 是否存在。
func (c *Cache) Exists(ctx context.Context, k string) bool {
	n, err := c.rdb.Exists(ctx, c.key(k)).Result()
	return err == nil && n > 0
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

// Remember 缓存未命中时调用 loader 回源（带空值缓存防穿透）。
func (c *Cache) Remember(ctx context.Context, k string, ttl time.Duration, loader func() (string, error)) (string, error) {
	if v, ok := c.Get(ctx, k); ok {
		if v == emptyPlaceholder {
			return "", nil
		}
		return v, nil
	}
	v, err := loader()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	if v == "" {
		_ = c.Set(ctx, k, emptyPlaceholder, 60*time.Second)
		return "", nil
	}
	_ = c.Set(ctx, k, v, jitter(ttl))
	return v, nil
}

const emptyPlaceholder = "\x00nil"

// Lua 脚本：只有持有者 token 匹配时才续期/删除，避免"锁过期后被他人持有，却被前一个持有者误删"。
var (
	scriptExtend = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
  return redis.call('pexpire', KEYS[1], ARGV[2])
end
return 0`)
	scriptRelease = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
  return redis.call('del', KEYS[1])
end
return 0`)
)

// AcquireLock 抢占租约（SET NX + TTL）。返回 (是否抢到, error)。
func (c *Cache) AcquireLock(ctx context.Context, k, token string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, c.key(k), token, ttl).Result()
}

// ExtendLock 续期租约：仅当仍由 token 持有时生效（心跳续期用）。
func (c *Cache) ExtendLock(ctx context.Context, k, token string, ttl time.Duration) bool {
	n, err := scriptExtend.Run(ctx, c.rdb, []string{c.key(k)}, token, ttl.Milliseconds()).Int64()
	return err == nil && n == 1
}

// ReleaseLock 释放租约：仅当仍由 token 持有时删除（避免误删他人的锁）。
func (c *Cache) ReleaseLock(ctx context.Context, k, token string) bool {
	n, err := scriptRelease.Run(ctx, c.rdb, []string{c.key(k)}, token).Int64()
	return err == nil && n == 1
}

// SetAdd 集合添加元素。
func (c *Cache) SetAdd(ctx context.Context, k string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	args := make([]any, 0, len(members))
	for _, m := range members {
		args = append(args, m)
	}
	return c.rdb.SAdd(ctx, c.key(k), args...).Err()
}

// SetMembers 集合元素列表。
func (c *Cache) SetMembers(ctx context.Context, k string) []string {
	return c.rdb.SMembers(ctx, c.key(k)).Val()
}

// SetRemove 集合移除元素。
func (c *Cache) SetRemove(ctx context.Context, k string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	args := make([]any, 0, len(members))
	for _, m := range members {
		args = append(args, m)
	}
	return c.rdb.SRem(ctx, c.key(k), args...).Err()
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
