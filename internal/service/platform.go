package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/plugin"
	"device-platform/internal/sysinfo"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// ---------- 概览 ----------

// HostOverview 设备概览（首页/系统概览卡片）。
type HostOverview struct {
	Summary        sysinfo.Summary   `json:"summary"`
	Host           sysinfo.HostInfo  `json:"host"`
	CPU            sysinfo.CPUInfo   `json:"cpu"`
	Memory         sysinfo.MemoryInfo `json:"memory"`
	Disks          []sysinfo.DiskInfo `json:"disks"`
	Net            sysinfo.NetInfo   `json:"net"`
	Load           sysinfo.LoadInfo  `json:"load"`
	Temperature    []float64         `json:"temperature"`
	Device         map[string]string `json:"device"`
	ActiveAlerts   int64             `json:"activeAlerts"`
	PluginTotal    int64             `json:"pluginTotal"`
	PluginRunning  int64             `json:"pluginRunning"`
	Platform       PlatformInfo      `json:"platform"`
}

// PlatformInfo 平台自身信息（关于页面）。
type PlatformInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Home      string `json:"home"`
	DBFile    string `json:"dbFile"`
	LogDir    string `json:"logDir"`
	PluginDir string `json:"pluginDir"`
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	UptimeSec int64  `json:"uptimeSec"`
}

// HostOverview 采集一次完整概览。
func (s *Service) HostOverview(ctx context.Context) (*HostOverview, error) {
	snap, err := s.Sys.Collect(ctx)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	ov := &HostOverview{
		Summary:     sysinfo.Summarize(snap),
		Host:        snap.Host,
		CPU:         snap.CPU,
		Memory:      snap.Memory,
		Disks:       snap.Disks,
		Net:         snap.Net,
		Load:        snap.Load,
		Temperature: snap.Temperature,
		Device:      s.deviceSettingsBestEffort(ctx),
		Platform:    s.platformInfo(),
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.AlertRecord{}).Where("status = 1").Count(&ov.ActiveAlerts).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Plugin{}).Count(&ov.PluginTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Plugin{}).Where("status = ?", plugin.StatusRunning).Count(&ov.PluginRunning).Error
	return ov, nil
}

// platformInfo 平台自身信息。
func (s *Service) platformInfo() PlatformInfo {
	return PlatformInfo{
		Name:      s.Cfg.App.Name,
		Version:   BuildVersion(),
		Home:      s.Paths.Home,
		DBFile:    s.Cfg.Database.Path,
		LogDir:    s.Cfg.Log.Dir,
		PluginDir: s.Cfg.Plugin.Dir,
		GoVersion: goVersion(),
		OS:        goOS(),
		Arch:      goArch(),
		UptimeSec: int64(time.Since(startedAt).Seconds()),
	}
}

// HostSnapshot 完整快照（硬件状态页）。
func (s *Service) HostSnapshot(ctx context.Context) (*sysinfo.Snapshot, error) {
	snap, err := s.Sys.Collect(ctx)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return snap, nil
}

// MetricHistory 指标历史（趋势图）。
func (s *Service) MetricHistory(ctx context.Context, minutes int) ([]entity.MetricSample, error) {
	if minutes <= 0 || minutes > 7*24*60 {
		minutes = 60
	}
	since := minutesAgo(-minutes)
	var rows []entity.MetricSample
	if err := s.Repo.DB.WithContext(ctx).
		Where("sampled_at >= ?", since.Format("2006-01-02 15:04:05")).
		Order("sampled_at asc").Limit(2000).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// ---------- 告警规则 ----------

// AlertRuleReq 告警规则入参。
type AlertRuleReq struct {
	ID          uint64  `json:"id,string"`
	Name        string  `json:"name" binding:"required,max=64"`
	Metric      string  `json:"metric" binding:"required,oneof=cpu mem disk net_in net_out"`
	Operator    string  `json:"operator" binding:"required,oneof=> >= < <="`
	Threshold   float64 `json:"threshold"`
	DurationSec int     `json:"durationSec"`
	Level       int     `json:"level"`
	Enabled     bool    `json:"enabled"`
	SilenceSec  int     `json:"silenceSec"`
	Remark      string  `json:"remark"`
}

var metricLabels = map[string]string{
	"cpu": "CPU 使用率", "mem": "内存使用率", "disk": "磁盘使用率",
	"net_in": "入向带宽", "net_out": "出向带宽",
}

// ListAlertRules 规则列表。
func (s *Service) ListAlertRules(ctx context.Context) ([]entity.AlertRule, error) {
	var rows []entity.AlertRule
	if err := s.Repo.DB.WithContext(ctx).Order("create_time desc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// CreateAlertRule 新建规则。
func (s *Service) CreateAlertRule(ctx context.Context, req *AlertRuleReq) (*entity.AlertRule, error) {
	row := entity.AlertRule{
		Name: req.Name, Metric: req.Metric, Operator: req.Operator,
		Threshold: req.Threshold, DurationSec: defaultInt(req.DurationSec, 60), Level: defaultInt(req.Level, 2),
		Enabled: req.Enabled, SilenceSec: defaultInt(req.SilenceSec, 600),
		TenantID: s.Cfg.Tenant.DefaultID,
	}
	row.Base = entity.Base{ID: id.Next(), Remark: req.Remark}
	row.CreateTime = nowTime()
	if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &row, nil
}

// UpdateAlertRule 修改规则。
func (s *Service) UpdateAlertRule(ctx context.Context, req *AlertRuleReq) error {
	if req.ID == 0 {
		return errs.Invalid("缺少规则 ID")
	}
	res := s.Repo.DB.WithContext(ctx).Model(&entity.AlertRule{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": req.Name, "metric": req.Metric, "operator": req.Operator, "threshold": req.Threshold,
		"duration_sec": defaultInt(req.DurationSec, 60), "level": defaultInt(req.Level, 2),
		"enabled": req.Enabled, "silence_sec": defaultInt(req.SilenceSec, 600),
		"update_time": nowTime(), "remark": req.Remark,
	})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.Invalid("告警规则不存在或未发生变化")
	}
	return nil
}

// DeleteAlertRule 删除规则。
func (s *Service) DeleteAlertRule(ctx context.Context, ruleID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", ruleID).Delete(&entity.AlertRule{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ListAlertRecords 告警记录。
func (s *Service) ListAlertRecords(ctx context.Context, status int, page, pageSize int) ([]entity.AlertRecord, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := s.Repo.DB.WithContext(ctx).Model(&entity.AlertRecord{})
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	var rows []entity.AlertRecord
	if err := q.Order("fired_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return rows, total, nil
}

// AckAlertRecord 确认告警。
func (s *Service) AckAlertRecord(ctx context.Context, recordID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Model(&entity.AlertRecord{}).Where("id = ?", recordID).
		Updates(map[string]any{"status": 3, "update_time": nowTime()})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ---------- 流量日志 ----------

// ListTrafficLogs 流量日志。
func (s *Service) ListTrafficLogs(ctx context.Context, iface string, page, pageSize int) ([]entity.TrafficLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 50
	}
	q := s.Repo.DB.WithContext(ctx).Model(&entity.TrafficLog{})
	if iface != "" {
		q = q.Where("iface = ?", iface)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	var rows []entity.TrafficLog
	if err := q.Order("logged_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return rows, total, nil
}

// ListAccessLogs 访问日志。
func (s *Service) ListAccessLogs(ctx context.Context, ip, path string, status int, page, pageSize int) ([]entity.AccessLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 20
	}
	q := s.Repo.DB.WithContext(ctx).Model(&entity.AccessLog{})
	if ip != "" {
		q = q.Where("ip LIKE ?", "%"+ip+"%")
	}
	if path != "" {
		q = q.Where("path LIKE ?", "%"+path+"%")
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	var rows []entity.AccessLog
	if err := q.Order("access_time desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return rows, total, nil
}

// ---------- 平台任务 ----------

type platformTasks struct {
	cancel context.CancelFunc
	mu     sync.Mutex
	states map[uint64]*alertState
}

type alertState struct {
	firedAt    int64 // 首次越限时间（unix 秒）
	lastNotify int64 // 上次生成记录时间
	firing     bool
}

var tasks = &platformTasks{states: make(map[uint64]*alertState)}

// StartPlatformTasks 启动后台任务：指标采集、流量采样、告警判定、数据清理、插件自启。
func (s *Service) StartPlatformTasks(ctx context.Context) {
	tctx, cancel := context.WithCancel(ctx)
	tasks.mu.Lock()
	tasks.cancel = cancel
	tasks.mu.Unlock()

	// 插件状态校准 + 自启
	go func() {
		if s.Plugins == nil {
			return
		}
		s.Plugins.Reconcile(tctx)
		s.Plugins.AutoStartAll(tctx)
	}()

	interval := s.Cfg.System.MonitorIntervalSec
	if interval <= 0 {
		interval = 10
	}
	go func() {
		ticker := tickerOfSeconds(interval)
		defer ticker.Stop()
		for {
			select {
			case <-tctx.Done():
				return
			case <-ticker.C:
				s.collectOnce(tctx)
			}
		}
	}()
	// 每小时做一次保留期清理
	go func() {
		ticker := tickerOfSeconds(3600)
		defer ticker.Stop()
		for {
			select {
			case <-tctx.Done():
				return
			case <-ticker.C:
				s.cleanupExpired(tctx)
			}
		}
	}()
	// 先同步跑一次，保证首页一打开就有数据
	s.collectOnce(tctx)
}

// StopPlatformTasks 停止后台任务并停掉所有插件进程。
func (s *Service) StopPlatformTasks() {
	tasks.mu.Lock()
	cancel := tasks.cancel
	tasks.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if s.Plugins != nil {
		s.Plugins.Shutdown()
	}
}

// collectOnce 采集一次指标并判定告警。
func (s *Service) collectOnce(ctx context.Context) {
	snap, err := s.Sys.Collect(ctx)
	if err != nil {
		return
	}
	sum := sysinfo.Summarize(snap)
	stamp := nowTime().Format("2006-01-02 15:04:05")

	sample := entity.MetricSample{
		ID: id.Next(), SampledAt: stamp,
		CPUPercent: sum.CPUPercent, MemPercent: sum.MemPercent, DiskPercent: sum.DiskPercent,
		NetInBps: sum.NetInBps, NetOutBps: sum.NetOutBps,
	}
	_ = s.Repo.DB.WithContext(ctx).Create(&sample).Error

	traffic := entity.TrafficLog{
		ID: id.Next(), Iface: "total",
		BytesIn: snap.Net.BytesRecv, BytesOut: snap.Net.BytesSent,
		PacketsIn: snap.Net.PacketsRecv, PacketsOut: snap.Net.PacketsSent,
		InBps: sum.NetInBps, OutBps: sum.NetOutBps, LoggedAt: stamp,
	}
	_ = s.Repo.DB.WithContext(ctx).Create(&traffic).Error

	s.evaluateAlerts(ctx, sum)
}

// evaluateAlerts 判定告警：持续越限超过 duration 才生成记录，恢复时自动关闭。
func (s *Service) evaluateAlerts(ctx context.Context, sum sysinfo.Summary) {
	var rules []entity.AlertRule
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return
	}
	nowSec := nowTime().Unix()
	tasks.mu.Lock()
	defer tasks.mu.Unlock()

	for _, r := range rules {
		val := metricValue(sum, r.Metric)
		hit := compare(val, r.Operator, r.Threshold)
		st, ok := tasks.states[r.ID]
		if !ok {
			st = &alertState{}
			tasks.states[r.ID] = st
		}
		if hit {
			if st.firedAt == 0 {
				st.firedAt = nowSec
			}
			// 达到持续时间且不在静默期 → 落一条告警记录
			if nowSec-st.firedAt >= int64(defaultInt(r.DurationSec, 60)) &&
				(st.lastNotify == 0 || nowSec-st.lastNotify >= int64(defaultInt(r.SilenceSec, 600))) {
				rec := entity.AlertRecord{
					Base:    entity.Base{ID: id.Next()},
					RuleID:  r.ID,
					RuleName: r.Name,
					Metric:  r.Metric,
					Level:   r.Level,
					Value:   val,
					Threshold: r.Threshold,
					Message: fmt.Sprintf("%s 当前 %.2f %s 阈值 %.2f", metricLabel(r.Metric), val, r.Operator, r.Threshold),
					Status:  1,
					FiredAt: nowTime().Format("2006-01-02 15:04:05"),
					TenantID: r.TenantID,
				}
				rec.CreateTime = nowTime()
				_ = s.Repo.DB.WithContext(ctx).Create(&rec).Error
				st.lastNotify = nowSec
				st.firing = true
			}
		} else {
			if st.firing {
				_ = s.Repo.DB.WithContext(ctx).Model(&entity.AlertRecord{}).
					Where("rule_id = ? AND status = 1", r.ID).
					Updates(map[string]any{"status": 2, "recover_at": nowTime().Format("2006-01-02 15:04:05")}).Error
			}
			st.firedAt = 0
			st.firing = false
		}
	}
}

// cleanupExpired 清理过期指标、流量日志与访问日志。
func (s *Service) cleanupExpired(ctx context.Context) {
	retainHours := s.Cfg.System.MetricRetainHours
	if retainHours <= 0 {
		retainHours = 168
	}
	cut := minutesAgo(-retainHours * 60).Format("2006-01-02 15:04:05")
	_ = s.Repo.DB.WithContext(ctx).Where("sampled_at < ?", cut).Delete(&entity.MetricSample{}).Error
	_ = s.Repo.DB.WithContext(ctx).Where("logged_at < ?", cut).Delete(&entity.TrafficLog{}).Error

	days := s.Cfg.System.LogRetainDays
	if days <= 0 {
		days = 30
	}
	logCut := minutesAgo(-days * 24 * 60).Format("2006-01-02 15:04:05")
	_ = s.Repo.DB.WithContext(ctx).Where("access_time < ?", logCut).Delete(&entity.AccessLog{}).Error
}

// ---------- 内置插件（随平台分发的插件包） ----------

// BundledPlugin 可安装的内置插件。
type BundledPlugin struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Installed   bool   `json:"installed"`
	Path        string `json:"-"`
}

// ListBundledPlugins 扫描 plugins-bundled 目录，列出平台自带但未安装的插件。
func (s *Service) ListBundledPlugins(ctx context.Context) ([]BundledPlugin, error) {
	entries, err := os.ReadDir(s.Paths.Bundled)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errs.ErrInternal.WithErr(err)
	}
	installed := map[string]bool{}
	if s.Plugins != nil {
		rows, _ := s.Plugins.List(ctx)
		for _, p := range rows {
			installed[p.Key] = true
		}
	}
	out := make([]BundledPlugin, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.Paths.Bundled, e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		man, err := plugin.ParseManifest(raw)
		if err != nil {
			continue
		}
		out = append(out, BundledPlugin{
			Key: man.ID, Name: man.Name, Version: man.Version, Description: man.Description,
			Category: man.Category, Installed: installed[man.ID], Path: dir,
		})
	}
	return out, nil
}

// InstallBundledPlugin 安装内置插件。
func (s *Service) InstallBundledPlugin(ctx context.Context, key string) (*entity.Plugin, error) {
	dir := filepath.Join(s.Paths.Bundled, key)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return nil, errs.Invalid("内置插件不存在：" + key)
	}
	if s.Plugins == nil {
		return nil, errs.ErrInternal.WithErr(fmt.Errorf("插件管理器未初始化"))
	}
	return s.Plugins.InstallBundled(ctx, dir)
}

// deviceSettingsBestEffort 读取设备设置（失败时返回空 map，不影响概览展示）。
func (s *Service) deviceSettingsBestEffort(ctx context.Context) map[string]string {
	var rows []struct {
		SettingKey   string `gorm:"column:setting_key"`
		SettingValue string `gorm:"column:setting_value"`
	}
	out := map[string]string{}
	if err := s.Repo.DB.WithContext(ctx).Table("sys_device_setting").Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[strings.TrimPrefix(r.SettingKey, "device.")] = r.SettingValue
	}
	return out
}

func metricValue(sum sysinfo.Summary, metric string) float64 {
	switch metric {
	case "cpu":
		return sum.CPUPercent
	case "mem":
		return sum.MemPercent
	case "disk":
		return sum.DiskPercent
	case "net_in":
		return sum.NetInBps / 8 / 1024 / 1024 // 转 MB/s，便于配置阈值
	case "net_out":
		return sum.NetOutBps / 8 / 1024 / 1024
	}
	return 0
}

func metricLabel(metric string) string {
	if v, ok := metricLabels[metric]; ok {
		return v
	}
	return metric
}

func compare(v float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return v > threshold
	case ">=":
		return v >= threshold
	case "<":
		return v < threshold
	case "<=":
		return v <= threshold
	}
	return false
}

// MarshalConfig 工具：把 map 序列化为 JSON 字符串。
func MarshalConfig(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
