package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
)

// 本文件：DHCP 服务与 DNS 服务（统一由 dnsmasq 提供）。
//
// 为什么用 dnsmasq：Ubuntu 设备上它体积小、同时提供 DHCP 与 DNS、配置是纯文本，
// 非常适合「期望态落库 → 渲染配置 → 重启服务」这种平台化管理方式。
//
// 平台只写 /etc/dnsmasq.d/devplat.conf 一个文件（dnsmasq 默认 conf-dir 会加载），
// 不覆盖系统其它 dnsmasq 配置；每次下发前备份到 <home>/backup/dnsmasq/。

const (
	dnsmasqConf     = "/etc/dnsmasq.d/devplat.conf"
	dnsmasqLease    = "/var/lib/misc/dnsmasq.leases"
	dnsmasqService  = "dnsmasq"
)

// DhcpStaticBinding DHCP 静态绑定。
type DhcpStaticBinding struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// DhcpConfigReq DHCP 配置入参。
type DhcpConfigReq struct {
	Enabled        bool                `json:"enabled"`
	Iface          string              `json:"iface"`
	RangeStart     string              `json:"rangeStart"`
	RangeEnd       string              `json:"rangeEnd"`
	Netmask        string              `json:"netmask"`
	LeaseHours     int                 `json:"leaseHours"`
	Gateway        string              `json:"gateway"`
	DNS            string              `json:"dns"`
	Domain         string              `json:"domain"`
	StaticBindings []DhcpStaticBinding `json:"staticBindings"`
}

// GetDhcpConfig 读取 DHCP 配置。
func (s *Service) GetDhcpConfig(ctx context.Context) (*entity.DhcpConfig, error) {
	var row entity.DhcpConfig
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", 1).First(&row).Error; err != nil {
		// 兜底：返回默认值，避免页面空白
		return &entity.DhcpConfig{StaticBindings: "[]", LeaseHours: 12, Netmask: "255.255.255.0"}, nil
	}
	return &row, nil
}

// SaveDhcpConfig 保存 DHCP 配置（校验地址池合法性）。
func (s *Service) SaveDhcpConfig(ctx context.Context, req *DhcpConfigReq) (*entity.DhcpConfig, error) {
	if req.Enabled {
		if !ifaceRe.MatchString(req.Iface) {
			return nil, errs.Invalid("请选择提供 DHCP 服务的网卡")
		}
		start, end := net.ParseIP(req.RangeStart), net.ParseIP(req.RangeEnd)
		if start == nil || end == nil {
			return nil, errs.Invalid("地址池起止地址格式非法，例：192.168.1.100")
		}
		// 掩码允许写 255.255.255.0，也允许写 24
		mask := req.Netmask
		if net.ParseIP(mask) == nil {
			bits, err := strconv.Atoi(mask)
			if err != nil || bits <= 0 || bits > 32 {
				return nil, errs.Invalid("子网掩码格式非法，例：255.255.255.0 或 24")
			}
			mask = net.IP(net.CIDRMask(bits, 32)).String()
			req.Netmask = mask
		}
		// 起止地址必须同网段且顺序正确
		if compareIP(start, end) > 0 {
			return nil, errs.Invalid("地址池起始地址不能大于结束地址")
		}
		if req.Gateway != "" && net.ParseIP(req.Gateway) == nil {
			return nil, errs.Invalid("网关格式非法")
		}
		for _, d := range splitCSV(req.DNS) {
			if net.ParseIP(d) == nil {
				return nil, errs.Invalid("DNS 服务器格式非法：" + d)
			}
		}
		for _, b := range req.StaticBindings {
			if !isMAC(b.MAC) {
				return nil, errs.Invalid("静态绑定的 MAC 格式非法：" + b.MAC)
			}
			if net.ParseIP(b.IP) == nil {
				return nil, errs.Invalid("静态绑定的 IP 格式非法：" + b.IP)
			}
		}
	}
	if req.LeaseHours <= 0 {
		req.LeaseHours = 12
	}
	if req.Netmask == "" {
		req.Netmask = "255.255.255.0"
	}

	bindings, _ := json.Marshal(req.StaticBindings)
	row := entity.DhcpConfig{
		Enabled:        req.Enabled,
		Iface:          req.Iface,
		RangeStart:     req.RangeStart,
		RangeEnd:       req.RangeEnd,
		Netmask:        req.Netmask,
		LeaseHours:     req.LeaseHours,
		Gateway:        req.Gateway,
		DNS:            joinCSV(splitCSV(req.DNS)),
		Domain:         req.Domain,
		StaticBindings: string(bindings),
		TenantID:       s.Cfg.Tenant.DefaultID,
	}
	row.ID = 1
	row.CreateTime = nowTime()

	if err := s.Repo.DB.WithContext(ctx).Exec(
		`INSERT INTO sys_dhcp_config (id, enabled, iface, range_start, range_end, netmask, lease_hours, gateway, dns, domain, static_bindings, applied, tenant_id, create_time)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled, iface=excluded.iface, range_start=excluded.range_start,
		   range_end=excluded.range_end, netmask=excluded.netmask, lease_hours=excluded.lease_hours,
		   gateway=excluded.gateway, dns=excluded.dns, domain=excluded.domain,
		   static_bindings=excluded.static_bindings, tenant_id=excluded.tenant_id, update_time=?`,
		row.ID, row.Enabled, row.Iface, row.RangeStart, row.RangeEnd, row.Netmask, row.LeaseHours,
		row.Gateway, row.DNS, row.Domain, row.StaticBindings, false, row.TenantID, row.CreateTime, nowTime(),
	).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return s.GetDhcpConfig(ctx)
}

// DnsRecord DNS 本地解析记录。
type DnsRecord struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// DnsServerReq DNS 服务配置入参。
type DnsServerReq struct {
	Enabled    bool        `json:"enabled"`
	ListenAddr string      `json:"listenAddr"`
	Upstreams  string      `json:"upstreams"`
	Records    []DnsRecord `json:"records"`
	CacheSize  int         `json:"cacheSize"`
}

// GetDnsServerConfig 读取 DNS 服务配置。
func (s *Service) GetDnsServerConfig(ctx context.Context) (*entity.DnsServerConfig, error) {
	var row entity.DnsServerConfig
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", 1).First(&row).Error; err != nil {
		return &entity.DnsServerConfig{ListenAddr: "0.0.0.0", Upstreams: "223.5.5.5", Records: "[]", CacheSize: 1000}, nil
	}
	return &row, nil
}

// SaveDnsServerConfig 保存 DNS 服务配置。
func (s *Service) SaveDnsServerConfig(ctx context.Context, req *DnsServerReq) (*entity.DnsServerConfig, error) {
	if req.Enabled {
		if req.ListenAddr == "" {
			req.ListenAddr = "0.0.0.0"
		}
		if net.ParseIP(req.ListenAddr) == nil {
			return nil, errs.Invalid("监听地址格式非法（0.0.0.0 表示所有网卡）")
		}
		if len(splitCSV(req.Upstreams)) == 0 {
			return nil, errs.Invalid("请至少配置一个上游 DNS")
		}
		for _, up := range splitCSV(req.Upstreams) {
			if net.ParseIP(up) == nil {
				return nil, errs.Invalid("上游 DNS 格式非法：" + up)
			}
		}
		for _, r := range req.Records {
			if strings.TrimSpace(r.Name) == "" || net.ParseIP(r.IP) == nil {
				return nil, errs.Invalid("本地解析记录需同时填写域名与合法 IP")
			}
		}
	}
	if req.CacheSize <= 0 {
		req.CacheSize = 1000
	}
	records, _ := json.Marshal(req.Records)
	row := entity.DnsServerConfig{
		Enabled:    req.Enabled,
		ListenAddr: defaultStr(req.ListenAddr, "0.0.0.0"),
		Upstreams:  joinCSV(splitCSV(req.Upstreams)),
		Records:    string(records),
		CacheSize:  req.CacheSize,
		TenantID:   s.Cfg.Tenant.DefaultID,
	}
	row.ID = 1
	if err := s.Repo.DB.WithContext(ctx).Exec(
		`INSERT INTO sys_dns_server_config (id, enabled, listen_addr, upstreams, records, cache_size, applied, tenant_id, create_time)
		 VALUES (?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled, listen_addr=excluded.listen_addr,
		   upstreams=excluded.upstreams, records=excluded.records, cache_size=excluded.cache_size,
		   tenant_id=excluded.tenant_id, update_time=?`,
		row.ID, row.Enabled, row.ListenAddr, row.Upstreams, row.Records, row.CacheSize, false, row.TenantID, nowTime(), nowTime(),
	).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return s.GetDnsServerConfig(ctx)
}

// DnsmasqStatus dnsmasq 运行状态与租约（DHCP/DNS 两个页面共用）。
type DnsmasqStatus struct {
	Installed   bool              `json:"installed"`
	Active      bool              `json:"active"`
	Version     string            `json:"version"`
	ConfFile    string            `json:"confFile"`
	ConfPreview string            `json:"confPreview"`
	Leases      []DnsmasqLease    `json:"leases"`
	Notes       []string          `json:"notes"`
}

// DnsmasqLease 一条租约记录。
type DnsmasqLease struct {
	ExpireAt string `json:"expireAt"`
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	ClientID string `json:"clientId"`
}

// GetDnsmasqStatus 状态查询。
func (s *Service) GetDnsmasqStatus(ctx context.Context) *DnsmasqStatus {
	st := &DnsmasqStatus{
		Installed:   syscmd.Exists("dnsmasq"),
		Active:      serviceReady(ctx, dnsmasqService),
		ConfFile:    dnsmasqConf,
		ConfPreview: readFileIfExists(dnsmasqConf),
	}
	if st.Installed {
		if res, err := syscmd.Run(ctx, 5*time.Second, "dnsmasq", "--version"); err == nil {
			first := strings.SplitN(strings.TrimSpace(res.Stdout), "\n", 2)[0]
			st.Version = first
		}
	}
	// 租约
	if body := readFileIfExists(dnsmasqLease); body != "" {
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			f := strings.Fields(line)
			if len(f) < 4 {
				continue
			}
			exp := "永久"
			if ts, err := strconv.ParseInt(f[0], 10, 64); err == nil && ts > 0 {
				exp = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
			}
			st.Leases = append(st.Leases, DnsmasqLease{
				ExpireAt: exp, MAC: f[1], IP: f[2], Hostname: strings.Trim(f[3], "*"), ClientID: strings.Join(f[4:], " "),
			})
		}
	}
	// 提示：systemd-resolved 会占用 127.0.0.53:53
	if serviceReady(ctx, "systemd-resolved") {
		st.Notes = append(st.Notes, "系统启用了 systemd-resolved（占用 127.0.0.53:53）。若 DNS 监听 0.0.0.0，请把上游或监听地址改成具体网卡 IP，避免端口冲突。")
	}
	if st.Installed && !st.Active {
		st.Notes = append(st.Notes, "dnsmasq 未运行：保存配置后点「下发」即可启动。")
	}
	return st
}

// InstallDnsmasq 安装 dnsmasq 依赖。
func (s *Service) InstallDnsmasq(ctx context.Context) ([]ApplyResult, error) {
	return s.installPackages(ctx, "dnsmasq"), nil
}

// ApplyDhcpConfig 下发 DHCP 配置（渲染 dnsmasq 配置并重启服务）。
func (s *Service) ApplyDhcpConfig(ctx context.Context) ([]ApplyResult, error) {
	return s.applyDnsmasq(ctx, "dhcp")
}

// ApplyDnsServerConfig 下发 DNS 服务配置。
func (s *Service) ApplyDnsServerConfig(ctx context.Context) ([]ApplyResult, error) {
	return s.applyDnsmasq(ctx, "dns")
}

// applyDnsmasq 把 DHCP / DNS 两份配置一起渲染成一个 dnsmasq 配置文件并生效。
//
// 为什么合并渲染：dnsmasq 是同一个进程，两个页面各自改一半会导致配置互相覆盖，
// 所以每次下发都按「当前库里的完整期望态」重建整个文件。
func (s *Service) applyDnsmasq(ctx context.Context, trigger string) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("dnsmasq 下发"); err != nil {
		return []ApplyResult{{Item: "dnsmasq", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "dnsmasq", OK: false, Message: "需要 root 权限（或 CAP_NET_BIND_SERVICE）才能监听 53/67 端口"}}, nil
	}
	if !syscmd.Exists("dnsmasq") {
		return []ApplyResult{{Item: "dnsmasq", OK: false, Message: "系统未安装 dnsmasq，请先点「安装依赖」（apt-get install -y dnsmasq）"}}, nil
	}

	dhcp, _ := s.GetDhcpConfig(ctx)
	dnsCfg, _ := s.GetDnsServerConfig(ctx)

	var conf strings.Builder
	conf.WriteString("# 由设备管理平台生成，请勿手工修改（改配置请用平台界面）\n")
	conf.WriteString("# 生成时间：" + nowTime().Format("2006-01-02 15:04:05") + "，触发：" + trigger + "\n\n")

	// ---------- DNS ----------
	conf.WriteString("# ---------- DNS ----------\n")
	if dnsCfg.Enabled {
		listen := defaultStr(dnsCfg.ListenAddr, "0.0.0.0")
		conf.WriteString("port=53\n")
		conf.WriteString("listen-address=" + listen + "\n")
		// 关键：不指定 0.0.0.0 时必须加 bind-dynamic，否则 dnsmasq 默认还会去绑通配地址，
		// 与 systemd-resolved（127.0.0.53/127.0.0.54:53）冲突导致起不来。
		if listen != "0.0.0.0" && listen != "::" {
			conf.WriteString("bind-dynamic\n")
		}
		ups := splitCSV(dnsCfg.Upstreams)
		if len(ups) > 0 {
			conf.WriteString("no-resolv\n")
			for _, up := range ups {
				conf.WriteString("server=" + up + "\n")
			}
		}
		conf.WriteString(fmt.Sprintf("cache-size=%d\n", defaultInt(dnsCfg.CacheSize, 1000)))
		var records []DnsRecord
		_ = json.Unmarshal([]byte(dnsCfg.Records), &records)
		for _, r := range records {
			conf.WriteString(fmt.Sprintf("address=/%s/%s\n", strings.TrimSpace(r.Name), strings.TrimSpace(r.IP)))
		}
	} else {
		conf.WriteString("# DNS 服务未启用\n")
	}

	// ---------- DHCP ----------
	conf.WriteString("\n# ---------- DHCP ----------\n")
	if dhcp.Enabled {
		conf.WriteString("interface=" + dhcp.Iface + "\n")
		lease := fmt.Sprintf("%dh", defaultInt(dhcp.LeaseHours, 12))
		conf.WriteString(fmt.Sprintf("dhcp-range=%s,%s,%s,%s\n", dhcp.RangeStart, dhcp.RangeEnd, dhcp.Netmask, lease))
		if dhcp.Gateway != "" {
			conf.WriteString("dhcp-option=3," + dhcp.Gateway + "\n")
		}
		if dnsList := splitCSV(dhcp.DNS); len(dnsList) > 0 {
			conf.WriteString("dhcp-option=6," + strings.Join(dnsList, ",") + "\n")
		}
		if dhcp.Domain != "" {
			conf.WriteString("domain=" + dhcp.Domain + "\n")
		}
		var bindings []DhcpStaticBinding
		_ = json.Unmarshal([]byte(dhcp.StaticBindings), &bindings)
		for _, b := range bindings {
			line := "dhcp-host=" + b.MAC + "," + b.IP
			if b.Hostname != "" {
				line += "," + b.Hostname
			}
			conf.WriteString(line + "\n")
		}
		conf.WriteString("dhcp-authoritative\n")
	} else {
		conf.WriteString("# DHCP 服务未启用\n")
	}

	backup := backupFile(dnsmasqConf, s.Paths.Backup+"/dnsmasq")
	if err := writeFileAtomic(dnsmasqConf, conf.String(), 0o644); err != nil {
		return []ApplyResult{{Item: "写入 " + dnsmasqConf, OK: false, Message: err.Error()}}, nil
	}
	results := []ApplyResult{
		{Item: "写入 " + dnsmasqConf, OK: true, Message: "已写入（备份：" + defaultStr(backup, "无原文件") + "）"},
	}

	// 两个都关掉就停服务，避免继续对外提供 DHCP/DNS
	if !dhcp.Enabled && !dnsCfg.Enabled {
		if err := stopService(ctx, dnsmasqService); err != nil {
			results = append(results, ApplyResult{Item: "停止 dnsmasq", OK: false, Message: err.Error()})
		} else {
			results = append(results, ApplyResult{Item: "停止 dnsmasq", OK: true, Message: "DHCP/DNS 均已关闭，服务已停止"})
		}
		return results, nil
	}

	if err := restartService(ctx, dnsmasqService); err != nil {
		results = append(results, ApplyResult{Item: "重启 dnsmasq", OK: false, Message: err.Error() + serviceLogHint(ctx, dnsmasqService)})
		return results, nil
	}
	active := serviceReady(ctx, dnsmasqService)
	results = append(results, ApplyResult{
		Item: "重启 dnsmasq", OK: active,
		Message: boolMsg(active, "服务已启动", "服务未处于运行状态，请查看 journalctl -u dnsmasq -n 50"),
	})

	// 记录下发状态
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.DhcpConfig{}).Where("id = ?", 1).Update("applied", active).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.DnsServerConfig{}).Where("id = ?", 1).Update("applied", active).Error
	return results, nil
}

// isMAC 校验 MAC 地址格式。
func isMAC(s string) bool {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 6 {
		return false
	}
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		for _, c := range p {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

// compareIP 比较两个 IP（仅支持同族）。
func compareIP(a, b net.IP) int {
	a4, b4 := a.To4(), b.To4()
	if a4 != nil && b4 != nil {
		for i := 0; i < 4; i++ {
			if a4[i] != b4[i] {
				if a4[i] < b4[i] {
					return -1
				}
				return 1
			}
		}
		return 0
	}
	// IPv6：按字节比较
	a16, b16 := a.To16(), b.To16()
	if a16 == nil || b16 == nil {
		return 0
	}
	for i := 0; i < 16; i++ {
		if a16[i] != b16[i] {
			if a16[i] < b16[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
