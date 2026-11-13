package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	yaml "go.yaml.in/yaml/v3"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// 本文件：网卡 / 网桥 / 端口聚合配置（Ubuntu netplan 落地）。
//
// 为什么用 netplan：Ubuntu 20.04/22.04 的网络由 netplan 接管，
// 直接改 /etc/network/interfaces 或 ip 命令都不持久。平台把自己负责的部分
// 写成**一个独立文件** /etc/netplan/90-devplat.yaml（序号靠后，优于系统默认 50-xxx），
// 下发即 `netplan generate && netplan apply`，并保留每次改动前的备份可回滚。
//
// 成员口规则：被桥/聚合占用的物理网卡不能再配地址（netplan 会报错），
// 渲染时统一输出 `dhcp4: false` + `optional: true`。

// netplanFile 平台托管的 netplan 文件。
const netplanFile = "/etc/netplan/90-devplat.yaml"

// errNoIfaceConfig 没有启用的网卡配置（预览接口据此返回空串而不是报错）。
var errNoIfaceConfig = errors.New("没有启用的网卡配置")

// bondModes 支持的聚合模式（netplan/内核 bonding 常用模式）。
var bondModes = map[string]string{
	"balance-rr":    "轮询（需要交换机配合）",
	"active-backup": "主备（最常用，无需交换机配置）",
	"balance-xor":   "异或哈希",
	"broadcast":     "广播冗余",
	"802.3ad":       "LACP 动态聚合（需要交换机配置 LACP）",
	"balance-tlb":   "发送负载均衡",
	"balance-alb":   "自适应负载均衡",
}

// IfaceConfigReq 网卡配置入参。
type IfaceConfigReq struct {
	ID        uint64 `json:"id,string"`
	Name      string `json:"name" binding:"required,max=32"`
	Kind      string `json:"kind" binding:"required,oneof=ethernet bridge bond"`
	Dhcp4     bool   `json:"dhcp4"`
	Addresses string `json:"addresses"`
	Gateway   string `json:"gateway"`
	MTU       int    `json:"mtu"`
	Members   string `json:"members"`
	BondMode  string `json:"bondMode"`
	STP       bool   `json:"stp"`
	Enabled   bool   `json:"enabled"`
	Remark    string `json:"remark"`
}

// IfaceConfigView 网卡配置视图（附带实时链路信息，便于对照）。
type IfaceConfigView struct {
	entity.IfaceConfig
	Up    bool     `json:"up"`
	Addrs []string `json:"addrs"`
}

// ListIfaceConfigs 网卡配置列表。
func (s *Service) ListIfaceConfigs(ctx context.Context) ([]IfaceConfigView, error) {
	var rows []entity.IfaceConfig
	if err := s.Repo.DB.WithContext(ctx).Order("kind asc, name asc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	live := map[string]struct {
		up    bool
		addrs []string
	}{}
	ifaces := s.Sys.Interfaces(ctx)
	for _, it := range ifaces {
		live[it.Name] = struct {
			up    bool
			addrs []string
		}{up: it.Up, addrs: it.Addrs}
	}
	out := make([]IfaceConfigView, 0, len(rows))
	for i := range rows {
		v := IfaceConfigView{IfaceConfig: rows[i]}
		if l, ok := live[rows[i].Name]; ok {
			v.Up = l.up
			v.Addrs = l.addrs
		}
		out = append(out, v)
	}
	return out, nil
}

// SaveIfaceConfig 新建/修改网卡配置（含校验）。
func (s *Service) SaveIfaceConfig(ctx context.Context, req *IfaceConfigReq) (*entity.IfaceConfig, error) {
	if !ifaceRe.MatchString(req.Name) {
		return nil, errs.Invalid("网卡名非法（仅允许字母、数字、点、下划线、中划线）")
	}
	for _, addr := range splitCSV(req.Addresses) {
		if err := validateCIDR(addr); err != nil {
			return nil, errs.Invalid(fmt.Sprintf("地址 %s 格式非法，例：192.168.1.10/24", addr))
		}
	}
	if req.Gateway != "" {
		if net.ParseIP(req.Gateway) == nil {
			return nil, errs.Invalid("网关地址格式非法")
		}
	}
	if req.MTU < 0 || req.MTU > 9000 {
		return nil, errs.Invalid("MTU 需在 0-9000 之间（0 表示不修改）")
	}
	members := splitCSV(req.Members)
	if req.Kind == "bridge" || req.Kind == "bond" {
		if len(members) == 0 {
			return nil, errs.Invalid("网桥/聚合必须至少指定一个成员网卡")
		}
		for _, m := range members {
			if !ifaceRe.MatchString(m) {
				return nil, errs.Invalid("成员网卡名非法：" + m)
			}
			if m == req.Name {
				return nil, errs.Invalid("成员网卡不能是自己")
			}
		}
	}
	if req.Kind == "bond" {
		if req.BondMode == "" {
			req.BondMode = "active-backup"
		}
		if _, ok := bondModes[req.BondMode]; !ok {
			return nil, errs.Invalid("不支持的聚合模式：" + req.BondMode)
		}
	}
	// 同一块成员网卡不能同时属于两个桥/聚合（否则 netplan 报错且现场排查困难）
	var others []entity.IfaceConfig
	if err := s.Repo.DB.WithContext(ctx).Where("kind IN ? AND name <> ?", []string{"bridge", "bond"}, req.Name).Find(&others).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	for _, o := range others {
		for _, m := range members {
			if containsCSV(o.Members, m) {
				return nil, errs.Invalid(fmt.Sprintf("成员网卡 %s 已被 %s（%s）占用", m, o.Name, o.Kind))
			}
		}
	}

	row := entity.IfaceConfig{
		Base:      entity.Base{ID: id.Next(), Remark: req.Remark},
		Name:      req.Name,
		Kind:      req.Kind,
		Dhcp4:     req.Dhcp4,
		Addresses: joinCSV(splitCSV(req.Addresses)),
		Gateway:   req.Gateway,
		MTU:       req.MTU,
		Members:   joinCSV(members),
		BondMode:  req.BondMode,
		STP:       req.STP,
		Enabled:   req.Enabled,
		TenantID:  s.Cfg.Tenant.DefaultID,
	}
	row.CreateTime = nowTime()

	if req.ID == 0 {
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
		return &row, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.IfaceConfig{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "kind": row.Kind, "dhcp4": row.Dhcp4, "addresses": row.Addresses,
		"gateway": row.Gateway, "mtu": row.MTU, "members": row.Members, "bond_mode": row.BondMode,
		"stp": row.STP, "enabled": row.Enabled, "remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, errs.ErrNotFound
	}
	return &row, nil
}

// DeleteIfaceConfig 删除网卡配置。
func (s *Service) DeleteIfaceConfig(ctx context.Context, cfgID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", cfgID).Delete(&entity.IfaceConfig{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// NetplanPreview 生成 netplan 片段预览（前端「下发前先看」）。
//
// 没有任何启用的配置时返回空串（页面显示「暂无配置」），不算错误。
func (s *Service) NetplanPreview(ctx context.Context) (string, error) {
	tree, err := s.buildNetplanTree(ctx)
	if err != nil {
		if errors.Is(err, errNoIfaceConfig) {
			return "", nil
		}
		return "", err
	}
	b, err := yaml.Marshal(tree)
	if err != nil {
		return "", errs.ErrInternal.WithErr(err)
	}
	return string(b), nil
}

// ApplyIfaceConfigs 下发网卡配置（写入 netplan 文件并 apply）。
func (s *Service) ApplyIfaceConfigs(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("网卡配置下发"); err != nil {
		return []ApplyResult{{Item: "netplan", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "netplan", OK: false, Message: "需要 root 权限才能修改网络配置"}}, nil
	}
	if !syscmd.Exists("netplan") {
		return []ApplyResult{{Item: "netplan", OK: false, Message: "系统未安装 netplan（Ubuntu 默认自带，请检查）"}}, nil
	}

	tree, err := s.buildNetplanTree(ctx)
	if err != nil {
		if errors.Is(err, errNoIfaceConfig) {
			return []ApplyResult{{Item: "netplan", OK: false, Message: "还没有启用的网卡配置，请先新增后下发"}}, nil
		}
		return nil, err
	}
	body, err := yaml.Marshal(tree)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	content := "# 由设备管理平台生成，请勿手工修改（改配置请用平台界面）\n" + string(body)

	// 先备份再原子写（0600：netplan 要求配置文件权限足够严格，否则 generate 会警告）
	backup := backupFile(netplanFile, filepath.Join(s.Paths.Backup, "netplan"))
	restore := func() {
		if backup != "" {
			if old, err := os.ReadFile(backup); err == nil {
				_ = writeFileAtomic(netplanFile, string(old), 0o600)
			}
		}
	}
	if err := writeFileAtomic(netplanFile, content, 0o600); err != nil {
		return []ApplyResult{{Item: "写入 " + netplanFile, OK: false, Message: err.Error()}}, nil
	}

	results := make([]ApplyResult, 0, 4)
	results = append(results, ApplyResult{Item: "写入 " + netplanFile, OK: true, Message: "已写入"})

	// netplan generate 只做语法与语义校验，不会动网卡：失败立刻回滚
	gen, genErr := syscmd.Run(ctx, 20*time.Second, "netplan", "generate")
	if genErr != nil {
		restore()
		_, _ = syscmd.Run(ctx, 20*time.Second, "netplan", "generate")
		return append(results, ApplyResult{
			Item:    "netplan generate（语法校验）",
			OK:      false,
			Message: firstNonEmpty(tail(gen.Combined(), 600), genErr.Error()) + "；已回滚到上一份配置",
		}), nil
	}
	results = append(results, ApplyResult{Item: "netplan generate（语法校验）", OK: true, Message: "校验通过"})

	apply, applyErr := syscmd.Run(ctx, 60*time.Second, "netplan", "apply")
	ok := applyErr == nil
	msg := "已生效"
	if !ok {
		msg = firstNonEmpty(tail(apply.Combined(), 600), applyErr.Error()) +
			"（如已失联，可登录控制台执行 `sudo cp " + backup + " " + netplanFile + " && sudo netplan apply` 回滚）"
	}
	results = append(results, ApplyResult{Item: "netplan apply", OK: ok, Message: msg})

	// 记录下发结果
	updates := map[string]any{"applied": ok, "update_time": nowTime()}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.IfaceConfig{}).Where("enabled = ?", true).Updates(updates).Error; err != nil {
		return results, errs.ErrDB.WithErr(err)
	}
	return results, nil
}

// buildNetplanTree 把数据库里的期望态渲染成 netplan 结构。
func (s *Service) buildNetplanTree(ctx context.Context) (map[string]any, error) {
	var rows []entity.IfaceConfig
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if len(rows) == 0 {
		return nil, errNoIfaceConfig
	}

	ethernets := map[string]any{}
	bridges := map[string]any{}
	bonds := map[string]any{}
	members := map[string]bool{}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Kind < rows[j].Kind })
	for _, r := range rows {
		switch r.Kind {
		case "bridge", "bond":
			for _, m := range splitCSV(r.Members) {
				members[m] = true
			}
		}
	}

	for _, r := range rows {
		conf := map[string]any{"dhcp4": r.Dhcp4}
		if addrs := splitCSV(r.Addresses); len(addrs) > 0 {
			conf["addresses"] = addrs
		}
		if r.Gateway != "" && !r.Dhcp4 {
			conf["routes"] = []map[string]any{{"to": "default", "via": r.Gateway}}
		}
		if r.MTU > 0 {
			conf["mtu"] = r.MTU
		}
		switch r.Kind {
		case "bridge":
			conf["interfaces"] = splitCSV(r.Members)
			conf["parameters"] = map[string]any{"stp": r.STP}
			bridges[r.Name] = conf
		case "bond":
			params := map[string]any{"mode": r.BondMode, "mii-monitor-interval": 100}
			conf["interfaces"] = splitCSV(r.Members)
			conf["parameters"] = params
			bonds[r.Name] = conf
		default:
			// 成员口不能带地址，否则 netplan generate 直接报错
			if members[r.Name] {
				conf = map[string]any{"dhcp4": false, "optional": true}
			}
			ethernets[r.Name] = conf
		}
	}

	// 桥/聚合用到的成员网卡一律渲染成「无地址 + optional」：
	// 否则 netplan 会因为成员口还带地址而拒绝生成。
	for m := range members {
		ethernets[m] = map[string]any{"dhcp4": false, "optional": true}
	}

	network := map[string]any{
		"version":  2,
		"renderer": "networkd",
	}
	if len(ethernets) > 0 {
		network["ethernets"] = ethernets
	}
	if len(bridges) > 0 {
		network["bridges"] = bridges
	}
	if len(bonds) > 0 {
		network["bonds"] = bonds
	}
	return map[string]any{"network": network}, nil
}

// containsCSV 判断逗号分隔列表中是否包含某项。
func containsCSV(list, item string) bool {
	for _, v := range splitCSV(list) {
		if v == item {
			return true
		}
	}
	return false
}

// NetplanStatus 读平台托管文件与 netplan 版本（页面展示用）。
type NetplanStatus struct {
	File      string `json:"file"`
	Exists    bool   `json:"exists"`
	Installed bool   `json:"installed"`
	Preview   string `json:"preview"`
}

// GetNetplanStatus 网卡配置页的状态信息。
func (s *Service) GetNetplanStatus(ctx context.Context) *NetplanStatus {
	st := &NetplanStatus{
		File:      netplanFile,
		Installed: syscmd.Exists("netplan"),
		Preview:   readFileIfExists(netplanFile),
	}
	if _, err := os.Stat(netplanFile); err == nil {
		st.Exists = true
	}
	if st.Preview == "" {
		if preview, err := s.NetplanPreview(ctx); err == nil {
			st.Preview = preview
		}
	}
	return st
}

// BondModes 供前端下拉使用。
func (s *Service) BondModes() map[string]string { return bondModes }
