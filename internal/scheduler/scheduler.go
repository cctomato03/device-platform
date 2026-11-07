// Package scheduler 基于 robfig/cron 的轻量任务调度器。
//
// 特性：
//   - 秒级 cron 表达式（6 段：秒 分 时 日 月 周）
//   - 任务存储在 sys_job 表，支持热重载（管理端增删改后调用 Reload）
//   - 同一任务不并发重入（SkipIfStillRunning）
//   - 执行结果写入 sys_job_log，失败按 retry_times 重试
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"web-basic/internal/logger"
	"web-basic/internal/service"
)

// Scheduler 任务调度器。
type Scheduler struct {
	svc     *service.Service
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[uint64]cron.EntryID
}

// New 创建调度器（未启动）。
func New(svc *service.Service) *Scheduler {
	c := cron.New(
		cron.WithSeconds(),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DiscardLogger),
			cron.Recover(cron.DiscardLogger),
		),
	)
	return &Scheduler{
		svc:     svc,
		cron:    c,
		entries: make(map[uint64]cron.EntryID),
	}
}

// Start 加载任务并开始调度；服务关闭时调用 Stop。
func (s *Scheduler) Start(ctx context.Context) {
	if err := s.Reload(ctx); err != nil {
		logger.S().Errorw("加载定时任务失败", "err", err)
	}
	s.cron.Start()
	logger.S().Infof("定时任务调度器已启动，已注册 %d 个任务", len(s.entries))
}

// Stop 停止调度器（等待运行中的任务结束）。
func (s *Scheduler) Stop() {
	if s.cron == nil {
		return
	}
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}
}

// Reload 重新装载全部任务（新增/删除/启停后调用）。
func (s *Scheduler) Reload(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for jobID, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
	}

	jobs, err := s.svc.ListJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		j := job
		if j.Status != 1 {
			continue
		}
		spec := j.Cron
		if spec == "" {
			continue
		}
		entryID, err := s.cron.AddFunc(spec, func() {
			runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if err := s.svc.RunJob(runCtx, j.ID, "cron"); err != nil {
				logger.S().Warnw("定时任务执行失败", "job", j.Name, "err", err)
			}
		})
		if err != nil {
			logger.S().Warnw("任务 cron 表达式无效", "job", j.Name, "cron", spec, "err", err)
			continue
		}
		s.entries[j.ID] = entryID
	}
	return nil
}

// NextRun 返回任务下次执行时间（未注册返回零值）。
func (s *Scheduler) NextRun(jobID uint64) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	entryID, ok := s.entries[jobID]
	if !ok {
		return time.Time{}
	}
	for _, entry := range s.cron.Entries() {
		if entry.ID == entryID {
			return entry.Next
		}
	}
	return time.Time{}
}
