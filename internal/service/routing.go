package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// 本文件：动态路由 BGP / OSPF（底层 FRR）。
//
// 选型说明：设备上做 BGP/OSPF，工业现场几乎都用 FRR（Quagga 的继任者，Ubuntu 源里就有）。
// 平台负责：装 FRR → 打开对应守护进程 → 按界面配置渲染 /etc/frr/frr.conf → 重启 →
// 用 vtysh 拉取邻居状态回显。协议细节（策略、路由映射）保持 FRR 原生能力，平台不重造轮子。

const (
	frrConf    = "/etc/frr/frr.conf"
	frrDaemons = "/etc/frr/daemons"
	frrService = "frr"
)

// BgpNeighbor BGP 邻居。
type BgpNeighbor struct {
	Addr     string `json:"addr"`
	RemoteAs int    `json:"remoteAs"`
}

// RoutingConfigReq 动态路由配置入参。
type RoutingConfigReq struct {
	Protocol  string        `json:"protocol" binding:"required,oneof=bgp ospf"`
	Enabled   bool          `json:"enabled"`
	ASN       int           `json:"asn"`
	RouterID  string        `json:"routerId"`
	Neighbors []BgpNeighbor `json:"neighbors"`
	Networks  string        `json:"networks"`
	Area      string        `json:"area"`
	Remark    string        `json:"remark"`
}

// ListRoutingConfigs 读取 BGP/OSPF 两份配置（不存在时返回默认结构，保证界面可用）。
func (s *Service) ListRoutingConfigs(ctx context.Context) ([]entity.RoutingConfig, error) {
	var rows []entity.RoutingConfig
	if err := s.Repo.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	byProto := map[string]entity.RoutingConfig{}
	for _, r := range rows {
		byProto[r.Protocol] = r
	}
	out := make([]entity.RoutingConfig, 0, 2)
	for _, proto := range []string{"bgp", "ospf"} {
		if r, ok := byProto[proto]; ok {
			out = append(out, r)
			continue
		}
		out = append(out, entity.RoutingConfig{Protocol: proto, Neighbors: "[]", Area: "0", Networks: ""})
	}
	return out, nil
}

// SaveRoutingConfig 保存某协议的配置。
func (s *Service) SaveRoutingConfig(ctx context.Context, req *RoutingConfigReq) (*entity.RoutingConfig, error) {
	if req.Enabled {
		switch req.Protocol {
		case "bgp":
			if req.ASN < 1 || req.ASN > 4294967295 {
				return nil, errs.Invalid("BGP AS 号需在 1-4294967295 之间")
			}
			for _, n := range req.Neighbors {
				if n.Addr == "" || n.RemoteAs <= 0 {
					return nil, errs.Invalid("BGP 邻居需同时填写地址与对端 AS 号")
				}
				if err := validateCIDR(n.Addr); err != nil {
					return nil, errs.Invalid("BGP 邻居地址非法：" + n.Addr)
				}
			}
		case "ospf":
			if strings.TrimSpace(req.RouterID) == "" {
				return nil, errs.Invalid("OSPF 必须填写 router-id（如 1.1.1.1）")
			}
			if len(splitCSV(req.Networks)) == 0 {
				return nil, errs.Invalid("OSPF 至少配置一个要启用的网段（如 192.168.1.0/24）")
			}
		}
		if len(splitCSV(req.Networks)) > 0 {
			for _, n := range splitCSV(req.Networks) {
				if err := validateCIDR(n); err != nil {
					return nil, errs.Invalid("网段格式非法：" + n)
				}
			}
		}
	}
	neighbors, _ := json.Marshal(req.Neighbors)
	row := entity.RoutingConfig{
		Base:      entity.Base{ID: id.Next(), Remark: req.Remark},
		Protocol:  req.Protocol,
		Enabled:   req.Enabled,
		ASN:       req.ASN,
		RouterID:  req.RouterID,
		Neighbors: string(neighbors),
		Networks:  joinCSV(splitCSV(req.Networks)),
		Area:      defaultStr(req.Area, "0"),
		TenantID:  s.Cfg.Tenant.DefaultID,
	}
	if err := s.Repo.DB.WithContext(ctx).Exec(
		`INSERT INTO sys_routing_config (id, protocol, enabled, asn, router_id, neighbors, networks, area, applied, tenant_id, create_time)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(protocol) DO UPDATE SET enabled=excluded.enabled, asn=excluded.asn, router_id=excluded.router_id,
		   neighbors=excluded.neighbors, networks=excluded.networks, area=excluded.area,
		   tenant_id=excluded.tenant_id, update_time=?`,
		row.ID, row.Protocol, row.Enabled, row.ASN, row.RouterID, row.Neighbors, row.Networks, row.Area, false, row.TenantID, nowTime(), nowTime(),
	).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &row, nil
}

// InstallFRR 安装 FRR 并打开 bgpd/ospfd 守护进程。
func (s *Service) InstallFRR(ctx context.Context) ([]ApplyResult, error) {
	results := s.installPackages(ctx, "frr", "frr-pythontools")
	if len(results) == 0 || !results[0].OK {
		return results, nil
	}
	// 打开守护进程（默认 bgpd=no / ospfd=no）
	if body := readFileIfExists(frrDaemons); body != "" {
		updated := body
		for _, d := range []string{"bgpd", "ospfd"} {
			updated = strings.ReplaceAll(updated, d+"=no", d+"=yes")
		}
		if updated != body {
			backup := backupFile(frrDaemons, s.Paths.Backup+"/frr")
			if err := writeFileAtomic(frrDaemons, updated, 0o644); err != nil {
				results = append(results, ApplyResult{Item: "启用 bgpd/ospfd", OK: false, Message: err.Error()})
				return results, nil
			}
			results = append(results, ApplyResult{Item: "启用 bgpd/ospfd", OK: true, Message: "已写入 " + frrDaemons + "（备份：" + defaultStr(backup, "无") + "）"})
		} else {
			results = append(results, ApplyResult{Item: "启用 bgpd/ospfd", OK: true, Message: "守护进程已启用"})
		}
	}
	if err := restartService(ctx, frrService); err != nil {
		results = append(results, ApplyResult{Item: "重启 frr", OK: false, Message: err.Error()})
		return results, nil
	}
	active := serviceReady(ctx, frrService)
	results = append(results, ApplyResult{Item: "重启 frr", OK: active, Message: boolMsg(active, "FRR 已启动", "FRR 未运行，请查看 journalctl -u frr -n 50")})
	return results, nil
}

// ApplyRoutingConfigs 渲染 frr.conf 并下发。
func (s *Service) ApplyRoutingConfigs(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("动态路由下发"); err != nil {
		return []ApplyResult{{Item: "frr", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "frr", OK: false, Message: "需要 root 权限"}}, nil
	}
	if !syscmd.Exists("vtysh") {
		return []ApplyResult{{Item: "frr", OK: false, Message: "系统未安装 FRR，请先点「安装依赖」（apt-get install -y frr）"}}, nil
	}

	var rows []entity.RoutingConfig
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}

	hostname, _ := os.Hostname()
	var b strings.Builder
	b.WriteString("frr defaults traditional\n")
	b.WriteString("hostname " + defaultStr(hostname, "device") + "\n")
	b.WriteString("!\n")

	for _, r := range rows {
		switch r.Protocol {
		case "bgp":
			fmt.Fprintf(&b, "router bgp %d\n", r.ASN)
			if r.RouterID != "" {
				fmt.Fprintf(&b, " bgp router-id %s\n", r.RouterID)
			}
			var neighbors []BgpNeighbor
			_ = json.Unmarshal([]byte(r.Neighbors), &neighbors)
			for _, n := range neighbors {
				fmt.Fprintf(&b, " neighbor %s remote-as %d\n", n.Addr, n.RemoteAs)
			}
			networks := splitCSV(r.Networks)
			if len(networks) > 0 {
				b.WriteString(" address-family ipv4 unicast\n")
				for _, n := range networks {
					fmt.Fprintf(&b, "  network %s\n", n)
				}
				b.WriteString(" exit-address-family\n")
			}
			b.WriteString("!\n")
		case "ospf":
			b.WriteString("router ospf\n")
			if r.RouterID != "" {
				fmt.Fprintf(&b, " ospf router-id %s\n", r.RouterID)
			}
			area := defaultStr(r.Area, "0")
			for _, n := range splitCSV(r.Networks) {
				fmt.Fprintf(&b, " network %s area %s\n", n, area)
			}
			b.WriteString("!\n")
		}
	}
	b.WriteString("line vty\n")

	backup := backupFile(frrConf, s.Paths.Backup+"/frr")
	if err := writeFileAtomic(frrConf, b.String(), 0o640); err != nil {
		return []ApplyResult{{Item: "写入 " + frrConf, OK: false, Message: err.Error()}}, nil
	}
	// FRR 要求配置文件属主为 frr:frr
	if u, err := user.Lookup("frr"); err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		_ = os.Chown(frrConf, uid, gid)
	}

	results := []ApplyResult{
		{Item: "写入 " + frrConf, OK: true, Message: "已写入（备份：" + defaultStr(backup, "无原文件") + "）"},
	}

	// 守护进程开关：apt 装完后 /etc/frr/daemons 里 bgpd/ospfd 默认是 no，
	// 不打开的话 frr.conf 里的 router 块根本不会被加载（现场极易踩坑）。
	if res := s.ensureFrrdDaemons(ctx); res != nil {
		results = append(results, *res)
		if !res.OK {
			return results, nil
		}
	}
	if err := restartService(ctx, frrService); err != nil {
		results = append(results, ApplyResult{Item: "重启 frr", OK: false, Message: err.Error()})
		return results, nil
	}
	active := serviceReady(ctx, frrService)
	results = append(results, ApplyResult{Item: "重启 frr", OK: active, Message: boolMsg(active, "配置已生效", "FRR 未运行，请查看 journalctl -u frr -n 50")})

	// 回读确认
	if res, err := syscmd.Run(ctx, 10*time.Second, "vtysh", "-c", "show running-config"); err == nil {
		preview := truncateText(res.Stdout, 400)
		results = append(results, ApplyResult{Item: "配置回读", OK: true, Message: preview})
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.RoutingConfig{}).Where("enabled = ?", true).Update("applied", active).Error
	return results, nil
}

// RoutingStatus 动态路由运行状态。
type RoutingStatus struct {
	Installed    bool     `json:"installed"`
	Active       bool     `json:"active"`
	BgpSummary   []string `json:"bgpSummary"`
	OspfNeighbor []string `json:"ospfNeighbor"`
	RouteTable   []string `json:"routeTable"`
	ConfPreview  string   `json:"confPreview"`
}

// GetRoutingStatus 拉取 FRR 状态（邻居与路由表）。
func (s *Service) GetRoutingStatus(ctx context.Context) *RoutingStatus {
	st := &RoutingStatus{
		Installed:   syscmd.Exists("vtysh"),
		Active:      serviceReady(ctx, frrService),
		ConfPreview: readFileIfExists(frrConf),
	}
	if !st.Installed {
		return st
	}
	if res, err := syscmd.Run(ctx, 10*time.Second, "vtysh", "-c", "show bgp summary"); err == nil {
		st.BgpSummary = splitLines(res.Stdout)
	}
	if res, err := syscmd.Run(ctx, 10*time.Second, "vtysh", "-c", "show ip ospf neighbor"); err == nil {
		st.OspfNeighbor = splitLines(res.Stdout)
	}
	if res, err := syscmd.Run(ctx, 10*time.Second, "vtysh", "-c", "show ip route"); err == nil {
		st.RouteTable = limitLines(splitLines(res.Stdout), 80)
	}
	return st
}

// ensureFrrdDaemons 打开 /etc/frr/daemons 中的 bgpd / ospfd。
//
// 返回 nil 表示无需改动（已经打开或没有该文件）。
func (s *Service) ensureFrrdDaemons(ctx context.Context) *ApplyResult {
	body := readFileIfExists(frrDaemons)
	if body == "" {
		return nil
	}
	updated := body
	changed := false
	for _, d := range []string{"bgpd", "ospfd"} {
		if strings.Contains(updated, d+"=no") {
			updated = strings.ReplaceAll(updated, d+"=no", d+"=yes")
			changed = true
		}
	}
	if !changed {
		return &ApplyResult{Item: "守护进程 bgpd/ospfd", OK: true, Message: "已处于启用状态"}
	}
	backup := backupFile(frrDaemons, s.Paths.Backup+"/frr")
	if err := writeFileAtomic(frrDaemons, updated, 0o644); err != nil {
		return &ApplyResult{Item: "启用守护进程 bgpd/ospfd", OK: false, Message: err.Error()}
	}
	return &ApplyResult{
		Item: "启用守护进程 bgpd/ospfd", OK: true,
		Message: "已写入 " + frrDaemons + "（备份：" + defaultStr(backup, "无") + "）",
	}
}

// truncateText 截断长文本（配置回读只回显关键片段）。
func truncateText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
