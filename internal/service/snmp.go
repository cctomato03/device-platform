package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
)

// 本文件：SNMP 服务（net-snmp snmpd）。
//
// 设备接入监控平台（Zabbix/Prometheus snmp_exporter/自建采集）普遍要求 SNMP，
// 平台负责渲染 /etc/snmp/snmpd.conf 并重启 snmpd；v2c 走 community，
// v3 走 snmpd 自带用户（由 net-snmp-create-v3-user 创建）。

const (
	snmpConf    = "/etc/snmp/snmpd.conf"
	snmpService = "snmpd"
)

// SnmpConfigReq SNMP 配置入参。
type SnmpConfigReq struct {
	Enabled     bool   `json:"enabled"`
	Version     string `json:"version" binding:"required,oneof=2c 3"`
	Community   string `json:"community"`
	V3User      string `json:"v3User"`
	V3Password  string `json:"v3Password"`
	Location    string `json:"location"`
	Contact     string `json:"contact"`
	ListenAddr  string `json:"listenAddr"`
	AllowedHost string `json:"allowedHost"`
}

// GetSnmpConfig 读取 SNMP 配置（密码不回传明文）。
func (s *Service) GetSnmpConfig(ctx context.Context) (*entity.SnmpConfig, error) {
	var row entity.SnmpConfig
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", 1).First(&row).Error; err != nil {
		return &entity.SnmpConfig{Version: "2c", Community: "public", ListenAddr: "0.0.0.0"}, nil
	}
	// 密码字段不回显，前端留空表示「不修改」
	if row.V3Password != "" {
		row.V3Password = ""
	}
	return &row, nil
}

// SaveSnmpConfig 保存 SNMP 配置。
func (s *Service) SaveSnmpConfig(ctx context.Context, req *SnmpConfigReq) (*entity.SnmpConfig, error) {
	if req.Enabled {
		if req.ListenAddr == "" {
			req.ListenAddr = "0.0.0.0"
		}
		if net.ParseIP(req.ListenAddr) == nil {
			return nil, errs.Invalid("监听地址格式非法（0.0.0.0 表示所有网卡）")
		}
		if req.Version == "2c" && strings.TrimSpace(req.Community) == "" {
			return nil, errs.Invalid("SNMP v2c 必须填写 community（建议不要用默认 public）")
		}
		if req.Version == "3" && strings.TrimSpace(req.V3User) == "" {
			return nil, errs.Invalid("SNMP v3 必须填写用户名")
		}
		if req.AllowedHost != "" && net.ParseIP(req.AllowedHost) == nil {
			return nil, errs.Invalid("允许来源需为单个 IP（如 10.0.0.10），留空表示全部允许")
		}
	}
	// 密码留空时保留原值
	var old entity.SnmpConfig
	_ = s.Repo.DB.WithContext(ctx).Where("id = ?", 1).First(&old).Error
	password := req.V3Password
	if password == "" {
		password = old.V3Password
	}

	row := entity.SnmpConfig{
		Enabled:     req.Enabled,
		Version:     req.Version,
		Community:   defaultStr(req.Community, "public"),
		V3User:      req.V3User,
		V3Password:  password,
		Location:    req.Location,
		Contact:     req.Contact,
		ListenAddr:  defaultStr(req.ListenAddr, "0.0.0.0"),
		AllowedHost: req.AllowedHost,
		TenantID:    s.Cfg.Tenant.DefaultID,
	}
	row.ID = 1
	if err := s.Repo.DB.WithContext(ctx).Exec(
		`INSERT INTO sys_snmp_config (id, enabled, version, community, v3_user, v3_password, location, contact, listen_addr, allowed_host, applied, tenant_id, create_time)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled, version=excluded.version, community=excluded.community,
		   v3_user=excluded.v3_user, v3_password=excluded.v3_password, location=excluded.location,
		   contact=excluded.contact, listen_addr=excluded.listen_addr, allowed_host=excluded.allowed_host,
		   tenant_id=excluded.tenant_id, update_time=?`,
		row.ID, row.Enabled, row.Version, row.Community, row.V3User, row.V3Password, row.Location,
		row.Contact, row.ListenAddr, row.AllowedHost, false, row.TenantID, nowTime(), nowTime(),
	).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	cfg, err := s.GetSnmpConfig(ctx)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// InstallSNMP 安装 net-snmp。
func (s *Service) InstallSNMP(ctx context.Context) ([]ApplyResult, error) {
	return s.installPackages(ctx, "snmpd", "snmp"), nil
}

// ApplySnmpConfig 渲染 snmpd.conf 并重启服务。
func (s *Service) ApplySnmpConfig(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("SNMP 下发"); err != nil {
		return []ApplyResult{{Item: "snmpd", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() {
		return []ApplyResult{{Item: "snmpd", OK: false, Message: "需要 root 权限才能修改 snmpd 配置并重启服务"}}, nil
	}
	if !syscmd.Exists("snmpd") {
		return []ApplyResult{{Item: "snmpd", OK: false, Message: "系统未安装 snmpd，请先点「安装依赖」（apt-get install -y snmpd snmp）"}}, nil
	}

	var cfg entity.SnmpConfig
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", 1).First(&cfg).Error; err != nil {
		return []ApplyResult{{Item: "snmpd", OK: false, Message: "请先保存 SNMP 配置"}}, nil
	}
	if !cfg.Enabled {
		if err := stopService(ctx, snmpService); err != nil {
			return []ApplyResult{{Item: "停止 snmpd", OK: false, Message: err.Error()}}, nil
		}
		return []ApplyResult{{Item: "停止 snmpd", OK: true, Message: "SNMP 已关闭，服务已停止"}}, nil
	}

	var b strings.Builder
	b.WriteString("# 由设备管理平台生成，请勿手工修改（改配置请用平台界面）\n")
	b.WriteString(fmt.Sprintf("agentAddress udp:%s:161\n", cfg.ListenAddr))
	if cfg.Version == "2c" {
		if cfg.AllowedHost != "" {
			b.WriteString(fmt.Sprintf("rocommunity %s %s\n", cfg.Community, cfg.AllowedHost))
		} else {
			b.WriteString(fmt.Sprintf("rocommunity %s\n", cfg.Community))
		}
	} else {
		// v3：用户由 net-snmp-create-v3-user 维护在 /var/lib/snmp/snmpd.conf，
		// 这里只需要 rouser 授权 + 关闭 v1/v2c
		if cfg.V3User != "" {
			b.WriteString(fmt.Sprintf("rouser %s auth\n", cfg.V3User))
		}
		b.WriteString("disableAuthorization no\n")
	}
	b.WriteString("sysLocation " + defaultStr(cfg.Location, "device") + "\n")
	b.WriteString("sysContact " + defaultStr(cfg.Contact, "admin@example.com") + "\n")
	b.WriteString("sysServices 72\n")
	b.WriteString("dontLogTCPWrappersConnects yes\n")
	b.WriteString("view systemonly included .1.3.6.1.2.1.1\n")
	b.WriteString("view systemonly included .1.3.6.1.2.1.25\n")
	b.WriteString("view systemonly included .1.3.6.1.4.1.2021\n")
	b.WriteString("view all included .1 80\n")

	backup := backupFile(snmpConf, s.Paths.Backup+"/snmp")
	if err := writeFileAtomic(snmpConf, b.String(), 0o600); err != nil {
		return []ApplyResult{{Item: "写入 " + snmpConf, OK: false, Message: err.Error()}}, nil
	}
	results := []ApplyResult{
		{Item: "写入 " + snmpConf, OK: true, Message: "已写入（备份：" + defaultStr(backup, "无原文件") + "）"},
	}

	// v3 用户创建：需要先停服务（net-snmp-create-v3-user 会读取运行中的用户库）
	if cfg.Version == "3" && cfg.V3User != "" && cfg.V3Password != "" {
		_ = stopService(ctx, snmpService)
		res, err := syscmd.Run(ctx, 20*time.Second, "net-snmp-create-v3-user", "-ro",
			"-A", cfg.V3Password, "-a", "SHA", "-X", cfg.V3Password, "-x", "AES", cfg.V3User)
		ok := err == nil
		msg := "v3 用户已创建/更新"
		if !ok {
			msg = firstNonEmpty(res.Combined(), err.Error())
		}
		results = append(results, ApplyResult{Item: "创建 SNMPv3 用户 " + cfg.V3User, OK: ok, Message: msg})
	}

	if err := restartService(ctx, snmpService); err != nil {
		results = append(results, ApplyResult{Item: "重启 snmpd", OK: false, Message: err.Error()})
		return results, nil
	}
	active := serviceReady(ctx, snmpService)
	results = append(results, ApplyResult{Item: "重启 snmpd", OK: active, Message: boolMsg(active, "已生效", "snmpd 未运行，请查看 journalctl -u snmpd -n 50")})
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.SnmpConfig{}).Where("id = ?", 1).Update("applied", active).Error
	return results, nil
}

// SnmpStatus SNMP 运行状态。
type SnmpStatus struct {
	Installed   bool     `json:"installed"`
	Active      bool     `json:"active"`
	Version     string   `json:"version"`
	ConfPreview string   `json:"confPreview"`
	Notes       []string `json:"notes"`
	// WalkHint 给用户的验证命令
	WalkHint string `json:"walkHint"`
}

// GetSnmpStatus 查询 SNMP 状态。
func (s *Service) GetSnmpStatus(ctx context.Context) *SnmpStatus {
	st := &SnmpStatus{
		Installed:   syscmd.Exists("snmpd"),
		Active:      serviceReady(ctx, snmpService),
		ConfPreview: readFileIfExists(snmpConf),
	}
	if st.Installed {
		if res, err := syscmd.Run(ctx, 5*time.Second, "snmpd", "-v"); err == nil {
			st.Version = strings.TrimSpace(res.Stdout + res.Stderr)
		}
	}
	cfg, _ := s.GetSnmpConfig(ctx)
	if cfg != nil && cfg.Version == "2c" {
		st.WalkHint = fmt.Sprintf("snmpwalk -v2c -c %s <设备IP> system", cfg.Community)
	} else if cfg != nil {
		st.WalkHint = fmt.Sprintf("snmpwalk -v3 -l authPriv -u %s -a SHA -A '<密码>' -x AES -X '<密码>' <设备IP> system", cfg.V3User)
	}
	if !st.Installed {
		st.Notes = append(st.Notes, "未检测到 snmpd：点「安装依赖」安装 net-snmp。")
	}
	return st
}
