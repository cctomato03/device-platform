package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/internal/sysinfo"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// NetworkStatus 网络状态总览。
type NetworkStatus struct {
	Interfaces []sysinfo.IfaceInfo `json:"interfaces"`
	Routes     []string            `json:"routes"`
	DNS        []string            `json:"dns"`
	Hostname   string              `json:"hostname"`
	Gateway    string              `json:"gateway"`
}

// GetNetworkStatus 汇总网卡、路由、DNS。
func (s *Service) GetNetworkStatus(ctx context.Context) (*NetworkStatus, error) {
	snap, err := s.Sys.Collect(ctx)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	st := &NetworkStatus{
		Interfaces: snap.NetIfaces,
		Hostname:   snap.Host.Hostname,
	}
	st.DNS = readDNS()
	st.Routes, st.Gateway = readRoutes(ctx)
	return st, nil
}

// ListInterfaces 网卡列表（网络接口配置页）。
func (s *Service) ListInterfaces(ctx context.Context) ([]sysinfo.IfaceInfo, error) {
	return s.Sys.Interfaces(ctx), nil
}

// ---------- 静态路由 ----------

// StaticRouteReq 静态路由入参。
type StaticRouteReq struct {
	ID          uint64 `json:"id,string"`
	Destination string `json:"destination" binding:"required"`
	Gateway     string `json:"gateway"`
	Device      string `json:"device"`
	Metric      int    `json:"metric"`
	Table       string `json:"table"`
	Enabled     bool   `json:"enabled"`
	Remark      string `json:"remark"`
}

// ListStaticRoutes 路由列表。
func (s *Service) ListStaticRoutes(ctx context.Context) ([]entity.StaticRoute, error) {
	var rows []entity.StaticRoute
	if err := s.Repo.DB.WithContext(ctx).Order("create_time desc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// SaveStaticRoute 新建或修改静态路由。
func (s *Service) SaveStaticRoute(ctx context.Context, req *StaticRouteReq) (*entity.StaticRoute, error) {
	if err := validateCIDR(req.Destination); err != nil {
		return nil, err
	}
	table := defaultStr(req.Table, "main")
	row := entity.StaticRoute{
		Base:        entity.Base{ID: id.Next(), Remark: req.Remark},
		Destination: req.Destination,
		Gateway:     req.Gateway,
		Device:      req.Device,
		Metric:      req.Metric,
		Table:       table,
		Enabled:     req.Enabled,
		TenantID:    s.Cfg.Tenant.DefaultID,
	}
	row.CreateTime = nowTime()

	if req.ID == 0 {
		row.ID = id.Next()
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
	} else {
		row.ID = req.ID
		res := s.Repo.DB.WithContext(ctx).Model(&entity.StaticRoute{}).Where("id = ?", req.ID).Updates(map[string]any{
			"destination": row.Destination, "gateway": row.Gateway, "device": row.Device,
			"metric": row.Metric, "table_name": row.Table, "enabled": row.Enabled,
			"remark": row.Remark, "update_time": nowTime(),
		})
		if res.Error != nil {
			return nil, errs.ErrDB.WithErr(res.Error)
		}
		if res.RowsAffected == 0 {
			return nil, errs.ErrNotFound
		}
	}
	return &row, nil
}

// DeleteStaticRoute 删除路由（连带撤销内核里的同款路由）。
func (s *Service) DeleteStaticRoute(ctx context.Context, routeID uint64) error {
	var row entity.StaticRoute
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", routeID).First(&row).Error; err != nil {
		return errs.ErrNotFound
	}
	_ = s.applyRoute(ctx, &row, false)
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", routeID).Delete(&entity.StaticRoute{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ApplyStaticRoutes 把期望状态下发到内核。
func (s *Service) ApplyStaticRoutes(ctx context.Context) ([]ApplyResult, error) {
	var rows []entity.StaticRoute
	if err := s.Repo.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	results := make([]ApplyResult, 0, len(rows))
	for i := range rows {
		applied := rows[i].Enabled
		res := applyResultForRoute(&rows[i], s.applyRoute(ctx, &rows[i], applied))
		results = append(results, res)
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StaticRoute{}).Where("id = ?", rows[i].ID).
			Update("applied", applied && res.OK).Error
	}
	return results, nil
}

func applyResultForRoute(r *entity.StaticRoute, err error) ApplyResult {
	item := "route " + r.Destination
	if err != nil {
		return ApplyResult{Item: item, OK: false, Message: err.Error()}
	}
	return ApplyResult{Item: item, OK: true, Message: "已下发"}
}

// applyRoute 用 ip route 下发/撤销路由。
func (s *Service) applyRoute(ctx context.Context, r *entity.StaticRoute, add bool) error {
	if err := syscmd.RequireLinux("静态路由下发"); err != nil {
		return err
	}
	if !syscmd.Exists("ip") {
		return fmt.Errorf("系统缺少 ip 命令（iproute2）")
	}
	args := []string{"route"}
	if add {
		args = append(args, "replace", r.Destination)
		if r.Gateway != "" {
			args = append(args, "via", r.Gateway)
		}
		if r.Device != "" {
			args = append(args, "dev", r.Device)
		}
		if r.Metric > 0 {
			args = append(args, "metric", strconv.Itoa(r.Metric))
		}
		if r.Table != "" && r.Table != "main" {
			args = append(args, "table", r.Table)
		}
	} else {
		args = append(args, "del", r.Destination)
		if r.Table != "" && r.Table != "main" {
			args = append(args, "table", r.Table)
		}
	}
	res, err := syscmd.Run(ctx, 10*time.Second, "ip", args...)
	if err != nil {
		return fmt.Errorf("%s", firstNonEmpty(res.Combined(), err.Error()))
	}
	return nil
}

// ---------- DNS ----------

// DNSConfig DNS 配置。
type DNSConfig struct {
	Servers []string `json:"servers"`
	Search  []string `json:"search"`
	Source  string   `json:"source"`
}

// GetDNSConfig 读取当前 DNS。
func (s *Service) GetDNSConfig(ctx context.Context) *DNSConfig {
	cfg := &DNSConfig{Source: "/etc/resolv.conf"}
	for _, line := range readResolvConf() {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			cfg.Servers = append(cfg.Servers, fields[1])
		case "search":
			cfg.Search = append(cfg.Search, fields[1:]...)
		}
	}
	return cfg
}

// UpdateDNSConfig 修改 DNS（写入 resolv.conf，下发失败返回可读原因）。
func (s *Service) UpdateDNSConfig(ctx context.Context, servers []string) ([]ApplyResult, error) {
	if len(servers) == 0 {
		return nil, errs.Invalid("至少填写一个 DNS 服务器")
	}
	joined := strings.Join(servers, ",")
	res := applyDNS(joined)
	// 同步写库，重启后仍能展示
	_ = s.Repo.DB.WithContext(ctx).Exec(
		"INSERT INTO sys_device_setting (setting_key, setting_value, updated_at) VALUES ('device.dns', ?, ?) "+
			"ON CONFLICT(setting_key) DO UPDATE SET setting_value = excluded.setting_value, updated_at = excluded.updated_at",
		joined, time.Now()).Error
	return []ApplyResult{res}, nil
}

// ---------- 内部工具 ----------

// validateCIDR 校验网段写法（支持单地址与 CIDR）。
func validateCIDR(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errs.Invalid("网段不能为空，例：10.0.0.0/8")
	}
	if net.ParseIP(v) != nil {
		return nil
	}
	if _, _, err := net.ParseCIDR(v); err != nil {
		return errs.Invalid("网段格式非法，例：10.0.0.0/8 或 192.168.1.1")
	}
	return nil
}

func readResolvConf() []string {
	body, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func readDNS() []string {
	out := []string{}
	for _, line := range readResolvConf() {
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				out = append(out, fields[1])
			}
		}
	}
	return out
}

func readRoutes(ctx context.Context) ([]string, string) {
	if !syscmd.IsLinux() || !syscmd.Exists("ip") {
		return nil, ""
	}
	res, err := syscmd.Run(ctx, 5*time.Second, "ip", "route", "show")
	if err != nil {
		return nil, ""
	}
	lines := []string{}
	gateway := ""
	for _, line := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
		if gateway == "" && strings.HasPrefix(line, "default via ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				gateway = fields[2]
			}
		}
	}
	return lines, gateway
}
