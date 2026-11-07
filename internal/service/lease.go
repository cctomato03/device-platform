// 分布式租约（Distributed Lease）：SET NX 抢锁 + 后台心跳续期 + 持有者校验后释放。
//
// 与"抢一次锁、结束时删掉"相比，租约解决两个多副本部署下的经典问题：
//  1. 任务执行时间超过锁 TTL 时，锁被自动释放 → 另一个副本重复执行（心跳续期消除这个窗口）；
//  2. 锁过期后被副本 B 抢到，副本 A 执行完把 B 的锁删掉 → 心跳 + token 校验后释放避免误删。
package service

import (
	"context"
	"sync"
	"time"

	"web-basic/pkg/id"
)

// 租约默认参数：TTL 60s，心跳间隔 20s（TTL 的 1/3，容忍两次心跳抖动）。
const (
	defaultLeaseTTL      = 60 * time.Second
	defaultLeaseInterval = 20 * time.Second
)

// Lease 一把带心跳的分布式租约。
type Lease struct {
	svc      *Service
	key      string
	token    string
	ttl      time.Duration
	stop     chan struct{}
	stopOnce sync.Once
}

// AcquireLease 抢占租约；busy=true 表示已被其他持有者占用。
func (s *Service) AcquireLease(ctx context.Context, key string, ttl time.Duration) (*Lease, bool, error) {
	if ttl <= 0 {
		ttl = defaultLeaseTTL
	}
	token := id.NewString()
	ok, err := s.Cache.AcquireLock(ctx, key, token, ttl)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, true, nil
	}
	l := &Lease{svc: s, key: key, token: token, ttl: ttl, stop: make(chan struct{})}
	l.startHeartbeat()
	return l, false, nil
}

// startHeartbeat 后台续期，直到 Release 被调用。
func (l *Lease) startHeartbeat() {
	interval := l.ttl / 3
	if interval <= 0 {
		interval = defaultLeaseInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-l.stop:
				return
			case <-ticker.C:
				// 心跳必须脱离请求上下文：请求结束后 ctx 会被 cancel，续期不应随之失效
				ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 5*time.Second)
				l.svc.Cache.ExtendLock(ctx, l.key, l.token, l.ttl)
				cancel()
			}
		}
	}()
}

// Release 停止心跳并释放租约（幂等，可安全 defer）。
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		close(l.stop)
	})
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 5*time.Second)
	defer cancel()
	l.svc.Cache.ReleaseLock(ctx, l.key, l.token)
}
