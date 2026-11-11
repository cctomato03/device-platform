package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"web-basic/internal/audit"
	"web-basic/internal/metrics"
	"web-basic/internal/model/entity"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
)

// JobQuery 任务列表查询。
type JobQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"`
}

// JobPayload 任务新增/编辑入参。
type JobPayload struct {
	Name       string `json:"name" binding:"required"`
	Handler    string `json:"handler"`
	JobType    int    `json:"jobType"`
	Target     string `json:"target"`
	HTTPMethod string `json:"httpMethod"`
	Cron       string `json:"cron" binding:"required"`
	Params     string `json:"params"`
	Concurrent int    `json:"concurrent"`
	TimeoutSec int    `json:"timeoutSec"`
	RetryTimes int    `json:"retryTimes"`
	Status     int    `json:"status"`
	Remark     string `json:"remark"`
}

// cronParser 与调度器 cron.New(WithSeconds()) 相同的解析器：秒可选。
var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// JobHandlerFunc 内置任务处理器：入参为任务参数，返回执行摘要。
type JobHandlerFunc func(ctx context.Context, params map[string]any) (string, error)

// JobHandlers 内置任务处理器注册表（sys_job.handler）。
var JobHandlers = map[string]JobHandlerFunc{}

// RegisterJobHandlers 注册需要访问依赖的内置处理器。
func (s *Service) RegisterJobHandlers() {
	JobHandlers["cleanSession"] = s.jobCleanSession
	JobHandlers["cleanTempFile"] = s.jobCleanTempFile
	JobHandlers["cleanOperLog"] = s.jobCleanOperLog
	JobHandlers["cleanLoginLog"] = s.jobCleanLoginLog
}

// ListJobs 定时任务列表。
func (s *Service) ListJobs(ctx context.Context) ([]entity.Job, error) {
	return s.Repo.ListJobs(ctx)
}

// validateJobPayload 校验任务配置：cron 表达式合法、HTTP 任务的目标地址合法（防 SSRF）。
func validateJobPayload(p *JobPayload) error {
	// 与调度器保持一致：支持「秒 分 时 日 月 周」6 段（也兼容省略秒的 5 段）
	if _, err := cronParser.Parse(strings.TrimSpace(p.Cron)); err != nil {
		return errs.InvalidErr(fmt.Errorf("cron 表达式不合法: %w", err))
	}
	if p.JobType == 2 {
		if err := checkJobTarget(p.Target); err != nil {
			return err
		}
	}
	return nil
}

// validateJobType 按任务类型做额外校验（Shell 任务需要总开关 + 参数合法）。
func (s *Service) validateJobType(p *JobPayload) error {
	if p.JobType == 3 {
		return s.validateShellJob(p)
	}
	return nil
}

// checkJobTarget 限制 HTTP 任务的目标地址：只允许 http/https，且禁止内网/环回/链路本地地址。
// 否则任何后台用户都能借定时任务探测或攻击内网服务（盲 SSRF）。
func checkJobTarget(target string) error {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return errs.InvalidErr(fmt.Errorf("target 不是合法 URL: %w", err))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errs.InvalidErr(fmt.Errorf("target 只允许 http/https"))
	}
	host := u.Hostname()
	if host == "" {
		return errs.InvalidErr(fmt.Errorf("target 缺少主机名"))
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errs.InvalidErr(fmt.Errorf("target 不允许指向内网地址"))
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return errs.InvalidErr(fmt.Errorf("target 不允许指向内网地址"))
	}
	return nil
}

// CreateJob 新增任务。
func (s *Service) CreateJob(ctx context.Context, p *JobPayload) (*entity.Job, error) {
	if err := validateJobPayload(p); err != nil {
		return nil, err
	}
	if err := s.validateJobType(p); err != nil {
		return nil, err
	}
	job := &entity.Job{
		Base:       entity.Base{ID: id.Next()},
		Name:       p.Name,
		Handler:    p.Handler,
		JobType:    defaultInt(p.JobType, 1),
		Target:     p.Target,
		HTTPMethod: p.HTTPMethod,
		Cron:       p.Cron,
		Params:     defaultStr(p.Params, "{}"),
		Concurrent: defaultInt(p.Concurrent, 1),
		TimeoutSec: defaultInt(p.TimeoutSec, 60),
		RetryTimes: p.RetryTimes,
		Status:     p.Status,
	}
	job.Remark = p.Remark
	if err := s.Repo.InsertJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// UpdateJob 编辑任务。
func (s *Service) UpdateJob(ctx context.Context, jobID uint64, p *JobPayload) error {
	job, err := s.Repo.FindJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return errs.ErrNotFound
	}
	job.Name = p.Name
	job.Handler = p.Handler
	job.JobType = defaultInt(p.JobType, job.JobType)
	job.Target = p.Target
	job.HTTPMethod = p.HTTPMethod
	job.Cron = p.Cron
	job.Params = defaultStr(p.Params, "{}")
	job.Concurrent = defaultInt(p.Concurrent, 1)
	job.TimeoutSec = defaultInt(p.TimeoutSec, 60)
	job.RetryTimes = p.RetryTimes
	job.Status = p.Status
	job.Remark = p.Remark
	if err := validateJobPayload(p); err != nil {
		return err
	}
	if err := s.validateJobType(p); err != nil {
		return err
	}
	before, _ := repository.LoadSnapshot(ctx, s.Repo.DB, &entity.Job{}, jobID)
	if err := s.Repo.SaveJob(ctx, job); err != nil {
		return err
	}
	// 记录字段级差异（审计：谁在什么时候把 cron / 目标地址改成了什么）
	audit.Note(ctx, audit.Diff(before, repository.Snapshot(job)))
	return nil
}

// SetJobStatus 启停任务。
func (s *Service) SetJobStatus(ctx context.Context, jobID uint64, status int) error {
	return s.Repo.UpdateJobStatus(ctx, jobID, status)
}

// DeleteJob 删除任务。
func (s *Service) DeleteJob(ctx context.Context, jobID uint64) error {
	return s.Repo.DeleteJob(ctx, jobID)
}

// ListJobLogs 任务执行日志。
func (s *Service) ListJobLogs(ctx context.Context, q *JobQuery) ([]entity.JobLog, int64, error) {
	page, size := normalizePage(q.Page, q.PageSize)
	return s.Repo.ListJobLogs(ctx, 0, page, size)
}

// RunJob 立即执行任务（同时被调度器与接口复用），trigger 标记触发来源。
func (s *Service) RunJob(ctx context.Context, jobID uint64, trigger string) error {
	job, err := s.Repo.FindJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return errs.ErrNotFound
	}

	timeout := time.Duration(defaultInt(job.TimeoutSec, 60)) * time.Second

	// 并发控制：同一任务同一时刻只允许一个实例执行。
	// 用"租约"而不是裸 SETNX：执行超过 TTL 时后台心跳续期（长任务不会被抢跑），
	// 进程崩了则 TTL 到期自动释放（不会永久卡死）；释放时校验持有者 token，避免误删他人的锁。
	// concurrent=1 表示允许并发；0/2 或未设置表示禁止并发。
	if job.Concurrent != 1 {
		l, busy, err := s.AcquireLease(ctx, fmt.Sprintf("job:lock:%d", job.ID), defaultLeaseTTL)
		if err != nil {
			return errs.ErrCache.WithErr(err)
		}
		if busy {
			metrics.Default.ObserveCron(job.Handler, "skipped", 0)
			if trigger == "cron" || trigger == "boot" {
				return nil // 定时触发直接跳过本次，避免堆叠
			}
			return errs.ErrJobFailed.WithErr(fmt.Errorf("任务正在执行中，请稍后再试"))
		}
		defer l.Release()
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	msg, runErr := s.executeJob(runCtx, job)
	costMS := int(time.Since(start).Milliseconds())

	status := 1
	if runErr != nil {
		status = 2
	}

	entry := &entity.JobLog{
		ID:          id.Next(),
		JobID:       job.ID,
		JobName:     job.Name,
		TriggerType: triggerType(trigger),
		Status:      status,
		StartTime:   start.Format("2006-01-02 15:04:05"),
		EndTime:     time.Now().Format("2006-01-02 15:04:05"),
		CostMS:      costMS,
		Result:      logResult(msg, runErr),
		CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
	}
	_ = s.Repo.InsertJobLog(ctx, entry)
	if runErr != nil {
		metrics.Default.ObserveCron(job.Handler, "failed", time.Duration(costMS)*time.Millisecond)
	} else {
		metrics.Default.ObserveCron(job.Handler, "success", time.Duration(costMS)*time.Millisecond)
	}

	if runErr != nil {
		// 失败重试（同步、有限次数），每次重试独立计时
		for i := 0; i < job.RetryTimes && runCtx.Err() == nil; i++ {
			rStart := time.Now()
			m, e := s.executeJob(runCtx, job)
			retryEntry := &entity.JobLog{
				ID:          id.Next(),
				JobID:       job.ID,
				JobName:     job.Name,
				TriggerType: triggerType(trigger),
				Status:      1,
				StartTime:   rStart.Format("2006-01-02 15:04:05"),
				EndTime:     time.Now().Format("2006-01-02 15:04:05"),
				CostMS:      int(time.Since(rStart).Milliseconds()),
				Result:      logResult(m, e),
				CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
			}
			if e != nil {
				retryEntry.Status = 2
			}
			_ = s.Repo.InsertJobLog(ctx, retryEntry)
			if e == nil {
				return nil
			}
		}
		return runErr
	}
	return nil
}

// triggerType 触发来源：1 手动、2 定时、3 启动补跑。
func triggerType(trigger string) int {
	switch trigger {
	case "cron":
		return 2
	case "boot":
		return 3
	default:
		return 1
	}
}

func logResult(msg string, err error) string {
	if err != nil {
		if msg == "" {
			return err.Error()
		}
		return msg + " | " + err.Error()
	}
	return msg
}

// executeJob 按任务类型分发执行。
func (s *Service) executeJob(ctx context.Context, job *entity.Job) (string, error) {
	// 类型 2：HTTP 回调
	if job.JobType == 2 {
		return s.execHTTPJob(ctx, job)
	}
	// 类型 3：Shell / Python 脚本（执行内容存在 params 里，可随时改）
	if job.JobType == 3 {
		return s.execShellJob(ctx, job)
	}
	h, ok := JobHandlers[job.Handler]
	if !ok || h == nil {
		return "", fmt.Errorf("未注册的任务处理器: %s", job.Handler)
	}
	return h(ctx, parseParams(job.Params))
}

// execHTTPJob 发起 HTTP 调用型任务。
func (s *Service) execHTTPJob(ctx context.Context, job *entity.Job) (string, error) {
	if job.Target == "" {
		return "", fmt.Errorf("HTTP 任务缺少 target")
	}
	if err := checkJobTarget(job.Target); err != nil {
		return "", err
	}
	method := strings.ToUpper(defaultStr(job.HTTPMethod, http.MethodPost))
	req, err := http.NewRequestWithContext(ctx, method, job.Target, strings.NewReader(job.Params))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: time.Duration(defaultInt(job.TimeoutSec, 60)) * time.Second,
		// 不跟随重定向：避免通过 302 绕过目标地址白名单
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := fmt.Sprintf("%s %s -> %d %s", method, job.Target, resp.StatusCode, strings.TrimSpace(string(body)))
	if resp.StatusCode >= 300 {
		return msg, fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	return msg, nil
}

/* ---------------- 内置任务处理器 ---------------- */

// jobCleanSession 清理 Redis 中过期的在线会话记录。
func (s *Service) jobCleanSession(_ context.Context, params map[string]any) (string, error) {
	ttlDays := intParam(params, "days", 7)
	limit := time.Now().AddDate(0, 0, -ttlDays).Unix()
	members := s.Cache.SetMembers(context.Background(), keyOnline)
	stale := make([]string, 0, len(members))
	for _, m := range members {
		parts := strings.Split(m, ":")
		if len(parts) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || ts < limit {
			stale = append(stale, m)
			// 同时清掉会话索引，避免退出登录时再回扫
			if len(parts) >= 2 {
				_ = s.Cache.Del(context.Background(), fmt.Sprintf(keyOnlineIdx, parseUintOrZero(parts[0]), parts[1]))
			}
		}
	}
	// 一次批量 SREM：避免每个成员一次 Redis 往返
	if err := s.Cache.SetRemove(context.Background(), keyOnline, stale...); err != nil {
		return "", err
	}
	return fmt.Sprintf("在线会话清理完成：共 %d 条，移除 %d 条", len(members), len(stale)), nil
}

// jobCleanTempFile 清理临时文件（bizType=temp 且超过保留天数）。
func (s *Service) jobCleanTempFile(ctx context.Context, params map[string]any) (string, error) {
	days := intParam(params, "days", 7)
	before := time.Now().AddDate(0, 0, -days)
	var files []entity.File
	err := s.Repo.DB.WithContext(ctx).
		Where("biz_type IN ? AND create_time < ?", []string{"temp", "tmp"}, before).Limit(500).Find(&files).Error
	if err != nil {
		return "", err
	}
	removed := 0
	for _, f := range files {
		if s.Storage != nil && f.ObjectKey != "" {
			_ = s.Storage.Delete(ctx, f.ObjectKey)
		}
		if err := s.Repo.DB.WithContext(ctx).Delete(&entity.File{}, f.ID).Error; err == nil {
			removed++
		}
	}
	return fmt.Sprintf("临时文件清理完成：保留 %d 天，删除 %d 个", days, removed), nil
}

// jobCleanOperLog 清理历史操作日志。
func (s *Service) jobCleanOperLog(ctx context.Context, params map[string]any) (string, error) {
	days := intParam(params, "days", 90)
	before := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	n, err := s.Repo.TrimOldRecords(ctx, "sys_oper_log", "oper_time", before)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("操作日志清理完成：删除 %d 条（%s 之前）", n, before), nil
}

// jobCleanLoginLog 清理历史登录日志。
func (s *Service) jobCleanLoginLog(ctx context.Context, params map[string]any) (string, error) {
	days := intParam(params, "days", 90)
	before := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	n, err := s.Repo.TrimOldRecords(ctx, "sys_login_log", "login_time", before)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("登录日志清理完成：删除 %d 条（%s 之前）", n, before), nil
}

/* ---------------- 辅助 ---------------- */

func parseParams(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// parseUintOrZero 把字符串解析为 uint64，失败返回 0（用于解析会话成员里的用户 ID）。
func parseUintOrZero(s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
func intParam(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	switch v := params[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func defaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func normalizePage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	return page, size
}
