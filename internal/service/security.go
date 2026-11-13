package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// ---------- 防火墙状态 ----------

// FirewallStatus 防火墙与内核过滤能力总览。
type FirewallStatus struct {
	// Manager 当前生效的防火墙管理方式：firewalld | nftables | iptables | none
	Manager string `json:"manager"`
	// Installed 是否已安装 firewalld
	FirewalldInstalled bool `json:"firewalldInstalled"`
	// Active 是否运行中
	FirewalldActive bool `json:"firewalldActive"`
	// Nftables iptables 命令是否可用
	Nftables bool `json:"nftables"`
	Iptables bool `json:"iptables"`
	// Rules 当前生效规则（只读展示）
	Rules []string `json:"rules"`
	// PolicyCount / NatCount / QosCount 平台策略数量
	PolicyCount int64 `json:"policyCount"`
	NatCount    int64 `json:"natCount"`
	QosCount    int64 `json:"qosCount"`
	// Root 是否有 root 权限（未提权时禁止下发）
	Root bool `json:"root"`
}

// GetFirewallStatus 探测防火墙状态。
func (s *Service) GetFirewallStatus(ctx context.Context) (*FirewallStatus, error) {
	st := &FirewallStatus{
		Manager:            "none",
		FirewalldInstalled: syscmd.Exists("firewall-cmd"),
		Nftables:           syscmd.Exists("nft"),
		Iptables:           syscmd.Exists("iptables"),
		Root:               isRoot(),
	}
	if st.FirewalldInstalled {
		st.Manager = "firewalld"
		res, err := syscmd.Run(ctx, 5*time.Second, "systemctl", "is-active", "firewalld")
		st.FirewalldActive = err == nil && strings.TrimSpace(res.Stdout) == "active"
		if st.FirewalldActive {
			if r, err := syscmd.Run(ctx, 8*time.Second, "firewall-cmd", "--list-all"); err == nil {
				st.Rules = splitLines(r.Stdout)
			}
		}
	} else if st.Nftables {
		st.Manager = "nftables"
		if r, err := syscmd.Run(ctx, 8*time.Second, "nft", "list", "ruleset"); err == nil {
			st.Rules = limitLines(splitLines(r.Stdout), 200)
		}
	} else if st.Iptables {
		st.Manager = "iptables"
		if r, err := syscmd.Run(ctx, 8*time.Second, "iptables", "-S"); err == nil {
			st.Rules = limitLines(splitLines(r.Stdout), 200)
		}
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.SecurePolicy{}).Count(&st.PolicyCount).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.NATRule{}).Count(&st.NatCount).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.QosRule{}).Count(&st.QosCount).Error
	return st, nil
}

// InstallFirewalld 安装/启用 firewalld（设备联网时可用）。
func (s *Service) InstallFirewalld(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("安装防火墙"); err != nil {
		return []ApplyResult{{Item: "firewalld", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "firewalld", OK: false, Message: "需要 root 权限（请用 root 或 sudo 运行平台）"}}, nil
	}
	out := make([]ApplyResult, 0, 3)
	if syscmd.Exists("apt-get") {
		res, err := syscmd.Run(ctx, 300*time.Second, "apt-get", "install", "-y", "firewalld")
		if err != nil {
			out = append(out, ApplyResult{Item: "apt install firewalld", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())})
			return out, nil
		}
		out = append(out, ApplyResult{Item: "apt install firewalld", OK: true, Message: "安装完成"})
	} else if syscmd.Exists("yum") {
		res, err := syscmd.Run(ctx, 300*time.Second, "yum", "install", "-y", "firewalld")
		if err != nil {
			out = append(out, ApplyResult{Item: "yum install firewalld", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())})
			return out, nil
		}
		out = append(out, ApplyResult{Item: "yum install firewalld", OK: true, Message: "安装完成"})
	} else {
		out = append(out, ApplyResult{Item: "firewalld", OK: false, Message: "系统没有 apt-get/yum，请手动安装 firewalld"})
	}
	if syscmd.Exists("systemctl") {
		if res, err := syscmd.Run(ctx, 30*time.Second, "systemctl", "enable", "--now", "firewalld"); err != nil {
			out = append(out, ApplyResult{Item: "systemctl enable --now firewalld", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())})
		} else {
			out = append(out, ApplyResult{Item: "systemctl enable --now firewalld", OK: true, Message: "服务已启动"})
		}
	}
	return out, nil
}

// ---------- 安全策略 ----------

// PolicyReq 安全策略入参。
type PolicyReq struct {
	ID        uint64 `json:"id,string"`
	Name      string `json:"name" binding:"required,max=64"`
	Direction string `json:"direction" binding:"required,oneof=in out forward"`
	Action    string `json:"action" binding:"required,oneof=accept drop reject"`
	Protocol  string `json:"protocol" binding:"required,oneof=tcp udp icmp any"`
	SrcAddr   string `json:"srcAddr"`
	DstAddr   string `json:"dstAddr"`
	Port      int    `json:"port"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
	Remark    string `json:"remark"`
}

// ListPolicies 策略列表（按优先级升序）。
func (s *Service) ListPolicies(ctx context.Context) ([]entity.SecurePolicy, error) {
	var rows []entity.SecurePolicy
	if err := s.Repo.DB.WithContext(ctx).Order("priority asc, create_time desc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// SavePolicy 新建/修改策略。
//
// 返回值第二项是**冲突告警**（不阻断保存）：优先级重复、重复规则、动作冲突、
// 被前序规则覆盖等，前端以黄色提示展示，让用户保存前就知道风险。
func (s *Service) SavePolicy(ctx context.Context, req *PolicyReq) (*entity.SecurePolicy, []string, error) {
	if req.SrcAddr != "" {
		if err := validateCIDR(req.SrcAddr); err != nil {
			return nil, nil, errs.Invalid("源地址非法：" + err.Error())
		}
	}
	if req.DstAddr != "" {
		if err := validateCIDR(req.DstAddr); err != nil {
			return nil, nil, errs.Invalid("目的地址非法：" + err.Error())
		}
	}
	if req.Port < 0 || req.Port > 65535 {
		return nil, nil, errs.Invalid("端口需在 0-65535 之间")
	}
	warnings := s.policyConflicts(ctx, req)
	row := entity.SecurePolicy{
		Base: entity.Base{ID: id.Next(), Remark: req.Remark}, Name: req.Name, Direction: req.Direction,
		Action: req.Action, Protocol: req.Protocol, SrcAddr: req.SrcAddr, DstAddr: req.DstAddr,
		Port: req.Port, Priority: defaultInt(req.Priority, 100), Enabled: req.Enabled,
		TenantID: s.Cfg.Tenant.DefaultID,
	}
	if req.ID == 0 {
		row.CreateTime = nowTime()
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, nil, errs.ErrDB.WithErr(err)
		}
		return &row, warnings, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.SecurePolicy{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "direction": row.Direction, "action": row.Action, "protocol": row.Protocol,
		"src_addr": row.SrcAddr, "dst_addr": row.DstAddr, "port": row.Port, "priority": row.Priority,
		"enabled": row.Enabled, "remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil, errs.ErrNotFound
	}
	return &row, warnings, nil
}

// DeletePolicy 删除策略。
func (s *Service) DeletePolicy(ctx context.Context, policyID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", policyID).Delete(&entity.SecurePolicy{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ApplyPolicies 把策略下发为 iptables 规则（幂等重建平台专属链）。
func (s *Service) ApplyPolicies(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("安全策略下发"); err != nil {
		return []ApplyResult{{Item: "policy", OK: false, Message: err.Error()}}, nil
	}
	if !syscmd.Exists("iptables") {
		return []ApplyResult{{Item: "policy", OK: false, Message: "系统缺少 iptables，无法下发策略"}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "policy", OK: false, Message: "需要 root 权限才能下发防火墙规则"}}, nil
	}
	var rows []entity.SecurePolicy
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Order("priority asc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	results := make([]ApplyResult, 0, len(rows)+2)
	// 重建平台专属链，避免与系统规则冲突
	for _, chain := range []string{"DEVPLAT_IN", "DEVPLAT_OUT", "DEVPLAT_FWD"} {
		_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-N", chain)
		_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-F", chain)
	}
	hookOK := true
	for parent, chain := range map[string]string{"INPUT": "DEVPLAT_IN", "OUTPUT": "DEVPLAT_OUT", "FORWARD": "DEVPLAT_FWD"} {
		if res, err := syscmd.Run(ctx, 5*time.Second, "iptables", "-C", parent, "-j", chain); err != nil {
			_ = res
			if _, err := syscmd.Run(ctx, 5*time.Second, "iptables", "-I", parent, "1", "-j", chain); err != nil {
				hookOK = false
			}
		}
	}
	results = append(results, ApplyResult{Item: "hook chains", OK: hookOK, Message: boolMsg(hookOK, "平台链已挂载到 INPUT/OUTPUT/FORWARD", "挂载平台链失败")})
	// 策略级风险提示（不影响下发，但要让人看见）
	for _, w := range s.applyPolicyWarnings(ctx) {
		results = append(results, ApplyResult{Item: "策略冲突提醒", OK: true, Message: w})
	}

	for _, r := range rows {
		chain := map[string]string{"in": "DEVPLAT_IN", "out": "DEVPLAT_OUT", "forward": "DEVPLAT_FWD"}[r.Direction]
		args := []string{"-A", chain}
		if r.Protocol != "any" {
			args = append(args, "-p", r.Protocol)
		}
		if r.SrcAddr != "" {
			args = append(args, "-s", r.SrcAddr)
		}
		if r.DstAddr != "" {
			args = append(args, "-d", r.DstAddr)
		}
		if r.Port > 0 && (r.Protocol == "tcp" || r.Protocol == "udp") {
			args = append(args, "--dport", strconv.Itoa(r.Port))
		}
		args = append(args, "-j", map[string]string{"accept": "ACCEPT", "drop": "DROP", "reject": "REJECT"}[r.Action])
		res, err := syscmd.Run(ctx, 8*time.Second, "iptables", args...)
		ok := err == nil
		msg := "已下发"
		if !ok {
			msg = firstNonEmpty(res.Combined(), err.Error())
		}
		results = append(results, ApplyResult{Item: r.Name, OK: ok, Message: msg})
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.SecurePolicy{}).Where("id = ?", r.ID).Update("applied", ok).Error
	}
	return results, nil
}

// ---------- NAT ----------

// NatReq NAT 策略入参。
type NatReq struct {
	ID       uint64 `json:"id,string"`
	Name     string `json:"name" binding:"required,max=64"`
	Type     string `json:"type" binding:"required,oneof=snat dnat"`
	Protocol string `json:"protocol" binding:"required,oneof=tcp udp any"`
	SrcIface string `json:"srcIface"`
	SrcAddr  string `json:"srcAddr"`
	DstAddr  string `json:"dstAddr"`
	DstPort  int    `json:"dstPort"`
	ToAddr   string `json:"toAddr"`
	ToPort   int    `json:"toPort"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

// ListNATRules NAT 列表。
func (s *Service) ListNATRules(ctx context.Context) ([]entity.NATRule, error) {
	var rows []entity.NATRule
	if err := s.Repo.DB.WithContext(ctx).Order("create_time desc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// SaveNATRule 新建/修改 NAT。
func (s *Service) SaveNATRule(ctx context.Context, req *NatReq) (*entity.NATRule, []string, error) {
	if req.Type == "dnat" && strings.TrimSpace(req.ToAddr) == "" {
		return nil, nil, errs.Invalid("DNAT 必须填写目标地址（转发到）")
	}
	warnings := s.natConflicts(ctx, req)
	row := entity.NATRule{
		Base: entity.Base{ID: id.Next(), Remark: req.Remark}, Name: req.Name, Type: req.Type,
		Protocol: req.Protocol, SrcIface: req.SrcIface, SrcAddr: req.SrcAddr, DstAddr: req.DstAddr,
		DstPort: req.DstPort, ToAddr: req.ToAddr, ToPort: req.ToPort, Enabled: req.Enabled,
		TenantID: s.Cfg.Tenant.DefaultID,
	}
	if req.ID == 0 {
		row.CreateTime = nowTime()
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, nil, errs.ErrDB.WithErr(err)
		}
		return &row, warnings, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.NATRule{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "rule_type": row.Type, "protocol": row.Protocol, "src_iface": row.SrcIface,
		"src_addr": row.SrcAddr, "dst_addr": row.DstAddr, "dst_port": row.DstPort,
		"to_addr": row.ToAddr, "to_port": row.ToPort, "enabled": row.Enabled,
		"remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil, errs.ErrNotFound
	}
	return &row, warnings, nil
}

// DeleteNATRule 删除 NAT。
func (s *Service) DeleteNATRule(ctx context.Context, natID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", natID).Delete(&entity.NATRule{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ApplyNATRules 下发 NAT（iptables nat 表）。
func (s *Service) ApplyNATRules(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("NAT 下发"); err != nil {
		return []ApplyResult{{Item: "nat", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() || !syscmd.Exists("iptables") {
		return []ApplyResult{{Item: "nat", OK: false, Message: "需要 root 权限与 iptables"}}, nil
	}
	var rows []entity.NATRule
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-t", "nat", "-N", "DEVPLAT_NAT")
	_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-t", "nat", "-F", "DEVPLAT_NAT")
	_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-t", "nat", "-C", "POSTROUTING", "-j", "DEVPLAT_NAT")
	_, _ = syscmd.Run(ctx, 5*time.Second, "iptables", "-t", "nat", "-I", "POSTROUTING", "1", "-j", "DEVPLAT_NAT")

	results := make([]ApplyResult, 0, len(rows))
	for _, r := range rows {
		args := []string{"-t", "nat", "-A", "DEVPLAT_NAT"}
		if r.Type == "dnat" {
			if r.Protocol != "any" {
				args = append(args, "-p", r.Protocol)
			}
			if r.DstAddr != "" {
				args = append(args, "-d", r.DstAddr)
			}
			if r.DstPort > 0 {
				args = append(args, "--dport", strconv.Itoa(r.DstPort))
			}
			target := r.ToAddr
			if r.ToPort > 0 {
				target = fmt.Sprintf("%s:%d", r.ToAddr, r.ToPort)
			}
			args = append(args, "-j", "DNAT", "--to-destination", target)
		} else {
			if r.SrcAddr != "" {
				args = append(args, "-s", r.SrcAddr)
			}
			if r.SrcIface != "" {
				args = append(args, "-o", r.SrcIface)
			}
			args = append(args, "-j", "SNAT", "--to-source", r.ToAddr)
		}
		res, err := syscmd.Run(ctx, 8*time.Second, "iptables", args...)
		ok := err == nil
		msg := "已下发"
		if !ok {
			msg = firstNonEmpty(res.Combined(), err.Error())
		}
		results = append(results, ApplyResult{Item: r.Name, OK: ok, Message: msg})
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.NATRule{}).Where("id = ?", r.ID).Update("applied", ok).Error
	}
	return results, nil
}

// ---------- QoS ----------

// QosReq QoS 规则入参。
type QosReq struct {
	ID        uint64 `json:"id,string"`
	Name      string `json:"name" binding:"required,max=64"`
	Iface     string `json:"iface" binding:"required,max=32"`
	Direction string `json:"direction" binding:"required,oneof=egress ingress"`
	RateKbps  int    `json:"rateKbps" binding:"required,min=8"`
	BurstKb   int    `json:"burstKb"`
	Target    string `json:"target"`
	Enabled   bool   `json:"enabled"`
	Remark    string `json:"remark"`
}

// ListQosRules QoS 列表。
func (s *Service) ListQosRules(ctx context.Context) ([]entity.QosRule, error) {
	var rows []entity.QosRule
	if err := s.Repo.DB.WithContext(ctx).Order("create_time desc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// SaveQosRule 新建/修改 QoS。
func (s *Service) SaveQosRule(ctx context.Context, req *QosReq) (*entity.QosRule, []string, error) {
	if !ifaceRe.MatchString(req.Iface) {
		return nil, nil, errs.Invalid("网卡名非法")
	}
	warnings := s.qosConflicts(ctx, req)
	row := entity.QosRule{
		Base: entity.Base{ID: id.Next(), Remark: req.Remark}, Name: req.Name, Iface: req.Iface,
		Direction: req.Direction, RateKbps: req.RateKbps, BurstKb: req.BurstKb, Target: req.Target,
		Enabled: req.Enabled, TenantID: s.Cfg.Tenant.DefaultID,
	}
	if req.ID == 0 {
		row.CreateTime = nowTime()
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, nil, errs.ErrDB.WithErr(err)
		}
		return &row, warnings, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.QosRule{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "iface": row.Iface, "direction": row.Direction, "rate_kbps": row.RateKbps,
		"burst_kb": row.BurstKb, "target": row.Target, "enabled": row.Enabled,
		"remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil, errs.ErrNotFound
	}
	return &row, warnings, nil
}

// DeleteQosRule 删除 QoS。
func (s *Service) DeleteQosRule(ctx context.Context, qosID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", qosID).Delete(&entity.QosRule{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ApplyQosRules 下发限速（tc htb，出方向生效；入方向用 ingress police 简化实现）。
func (s *Service) ApplyQosRules(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("QoS 限速下发"); err != nil {
		return []ApplyResult{{Item: "qos", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() || !syscmd.Exists("tc") {
		return []ApplyResult{{Item: "qos", OK: false, Message: "需要 root 权限与 iproute2(tc)"}}, nil
	}
	var rows []entity.QosRule
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	results := make([]ApplyResult, 0, len(rows))
	// 清理平台创建的根 qdisc（幂等）
	ifaces := map[string]bool{}
	for _, r := range rows {
		ifaces[r.Iface] = true
	}
	for iface := range ifaces {
		_, _ = syscmd.Run(ctx, 5*time.Second, "tc", "qdisc", "del", "dev", iface, "root")
	}
	for _, r := range rows {
		rate := fmt.Sprintf("%dkbit", r.RateKbps)
		burst := "32k"
		if r.BurstKb > 0 {
			burst = fmt.Sprintf("%dk", r.BurstKb)
		}
		var (
			res syscmd.Result
			err error
		)
		if r.Direction == "egress" {
			_, _ = syscmd.Run(ctx, 5*time.Second, "tc", "qdisc", "add", "dev", r.Iface, "root", "handle", "1:", "htb", "default", "10")
			_, _ = syscmd.Run(ctx, 5*time.Second, "tc", "class", "add", "dev", r.Iface, "parent", "1:", "classid", "1:10", "htb", "rate", rate, "burst", burst)
			res, err = syscmd.Run(ctx, 5*time.Second, "tc", "qdisc", "add", "dev", r.Iface, "parent", "1:10", "handle", "10:", "fq_codel")
		} else {
			// 入向：用 ingress + police
			_, _ = syscmd.Run(ctx, 5*time.Second, "tc", "qdisc", "add", "dev", r.Iface, "handle", "ffff:", "ingress")
			res, err = syscmd.Run(ctx, 5*time.Second, "tc", "filter", "add", "dev", r.Iface, "parent", "ffff:",
				"protocol", "all", "prio", "1", "u32", "match", "u32", "0", "0",
				"police", "rate", rate, "burst", burst, "drop", "flowid", ":1")
		}
		ok := err == nil
		msg := "已下发"
		if !ok {
			msg = firstNonEmpty(res.Combined(), err.Error())
		}
		results = append(results, ApplyResult{Item: r.Name, OK: ok, Message: msg})
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.QosRule{}).Where("id = ?", r.ID).Update("applied", ok).Error
	}
	return results, nil
}

// ---------- 小工具 ----------

func splitLines(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func limitLines(lines []string, max int) []string {
	if len(lines) > max {
		return lines[:max]
	}
	return lines
}

func boolMsg(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}
