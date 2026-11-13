package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
)

// ApplyResult 单项系统设置的下发结果（逐项反馈，避免整体回滚）。
type ApplyResult struct {
	Item    string `json:"item"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// DeviceSettings 设备本地设置。
type DeviceSettings struct {
	Name        string `json:"name"`
	Timezone    string `json:"timezone"`
	NTPEnabled  bool   `json:"ntpEnabled"`
	NTPServers  string `json:"ntpServers"`
	DNS         string `json:"dns"`
	SniffEnabled bool  `json:"sniffEnabled"`
	SniffIface  string `json:"sniffIface"`
	Remark      string `json:"remark"`
}

// GetDeviceSettings 读取本地设置。
func (s *Service) GetDeviceSettings(ctx context.Context) (*DeviceSettings, error) {
	kv := s.deviceSettingsBestEffort(ctx)
	return &DeviceSettings{
		Name:         kv["name"],
		Timezone:     defaultStr(kv["timezone"], s.Cfg.App.Timezone),
		NTPEnabled:   kv["ntp_enabled"] == "1",
		NTPServers:   kv["ntp_servers"],
		DNS:          kv["dns"],
		SniffEnabled: kv["sniff_enabled"] == "1",
		SniffIface:   kv["sniff_iface"],
		Remark:       kv["remark"],
	}, nil
}

// UpdateDeviceSettings 保存并下发本地设置。
//
// 设计原则：能落库的一律先落库，系统级下发失败只回报该项失败，
// 保证设备在容器/开发机上也能完整使用界面（只是提示「当前环境不支持」）。
func (s *Service) UpdateDeviceSettings(ctx context.Context, req *DeviceSettings) ([]ApplyResult, error) {
	kv := map[string]string{
		"device.name":          req.Name,
		"device.timezone":      req.Timezone,
		"device.ntp_enabled":   boolStr(req.NTPEnabled),
		"device.ntp_servers":   req.NTPServers,
		"device.dns":           req.DNS,
		"device.sniff_enabled": boolStr(req.SniffEnabled),
		"device.sniff_iface":   req.SniffIface,
		"device.remark":        req.Remark,
	}
	for k, v := range kv {
		if err := s.Repo.DB.WithContext(ctx).Exec(
			"INSERT INTO sys_device_setting (setting_key, setting_value, updated_at) VALUES (?, ?, ?) "+
				"ON CONFLICT(setting_key) DO UPDATE SET setting_value = excluded.setting_value, updated_at = excluded.updated_at",
			k, v, time.Now()).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
	}

	results := make([]ApplyResult, 0, 4)
	// 设备名
	if req.Name != "" {
		results = append(results, applyHostname(req.Name))
	}
	// 时区
	if req.Timezone != "" {
		results = append(results, applyTimezone(req.Timezone))
	}
	// NTP
	results = append(results, applyNTP(req.NTPEnabled, req.NTPServers))
	// DNS
	if strings.TrimSpace(req.DNS) != "" {
		results = append(results, applyDNS(req.DNS))
	}
	return results, nil
}

// DeviceTimeInfo 设备时间信息。
type DeviceTimeInfo struct {
	LocalTime  string `json:"localTime"`
	UTC        string `json:"utc"`
	Timezone   string `json:"timezone"`
	Unix       int64  `json:"unix"`
	NTPEnabled bool   `json:"ntpEnabled"`
	NTPSynced  bool   `json:"ntpSynced"`
	NTPServers string `json:"ntpServers"`
	Source     string `json:"source"`
}

// GetDeviceTime 读取时间与 NTP 状态。
func (s *Service) GetDeviceTime(ctx context.Context) (*DeviceTimeInfo, error) {
	now := time.Now()
	info := &DeviceTimeInfo{
		LocalTime: now.Format("2006-01-02 15:04:05"),
		UTC:       now.UTC().Format("2006-01-02 15:04:05"),
		Timezone:  now.Location().String(),
		Unix:      now.Unix(),
	}
	kv := s.deviceSettingsBestEffort(ctx)
	info.NTPEnabled = kv["ntp_enabled"] == "1"
	info.NTPServers = kv["ntp_servers"]

	if syscmd.IsLinux() && syscmd.Exists("timedatectl") {
		if res, err := syscmd.Run(ctx, 3*time.Second, "timedatectl", "show", "--no-pager"); err == nil {
			props := parseKeyValue(res.Stdout)
			if v, ok := props["Timezone"]; ok {
				info.Timezone = v
			}
			info.NTPEnabled = props["NTP"] == "yes"
			info.NTPSynced = props["NTPSynchronized"] == "yes"
			info.Source = "timedatectl"
		}
	}
	return info, nil
}

// SyncTimeNow 立即同步时间（timedatectl 或 chronyc/ntpdate 兜底）。
func (s *Service) SyncTimeNow(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("时间同步"); err != nil {
		return []ApplyResult{{Item: "ntp", OK: false, Message: err.Error()}}, nil
	}
	out := make([]ApplyResult, 0, 2)
	if syscmd.Exists("timedatectl") {
		if res, err := syscmd.Run(ctx, 15*time.Second, "timedatectl", "set-ntp", "true"); err == nil {
			out = append(out, ApplyResult{Item: "ntp", OK: true, Message: "已开启自动时间同步"})
		} else {
			out = append(out, ApplyResult{Item: "ntp", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())})
		}
	}
	if syscmd.Exists("chronyc") {
		if res, err := syscmd.Run(ctx, 15*time.Second, "chronyc", "makestep"); err == nil {
			out = append(out, ApplyResult{Item: "chrony", OK: true, Message: "已强制校时"})
		} else {
			out = append(out, ApplyResult{Item: "chrony", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())})
		}
	}
	if len(out) == 0 {
		out = append(out, ApplyResult{Item: "ntp", OK: false, Message: "系统未安装 timedatectl / chronyc，无法校时"})
	}
	return out, nil
}

// SystemLog 读取平台自身日志（系统日志页）。
func (s *Service) SystemLog(ctx context.Context, lines int) (map[string]any, error) {
	if lines <= 0 || lines > 5000 {
		lines = 500
	}
	dir := s.Cfg.Log.Dir
	files := []string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			files = append(files, e.Name())
		}
	}
	primary := filepath.Join(dir, s.Cfg.App.Name+".log")
	if _, err := os.Stat(primary); err != nil && len(files) > 0 {
		primary = filepath.Join(dir, files[0])
	}
	text, err := tailFileLines(primary, lines)
	if err != nil && !os.IsNotExist(err) {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return map[string]any{
		"dir":   dir,
		"files": files,
		"file":  filepath.Base(primary),
		"lines": strings.Split(strings.TrimRight(text, "\n"), "\n"),
	}, nil
}

// ---------- 系统下发实现 ----------

func applyHostname(name string) ApplyResult {
	if err := syscmd.RequireLinux("修改设备名"); err != nil {
		return ApplyResult{Item: "hostname", OK: false, Message: err.Error()}
	}
	if !syscmd.Exists("hostnamectl") {
		return ApplyResult{Item: "hostname", OK: false, Message: "系统缺少 hostnamectl，无法修改设备名"}
	}
	res, err := syscmd.Run(context.Background(), 10*time.Second, "hostnamectl", "set-hostname", name)
	if err != nil {
		return ApplyResult{Item: "hostname", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())}
	}
	return ApplyResult{Item: "hostname", OK: true, Message: "设备名已更新为 " + name}
}

func applyTimezone(tz string) ApplyResult {
	if err := syscmd.RequireLinux("修改时区"); err != nil {
		return ApplyResult{Item: "timezone", OK: false, Message: err.Error()}
	}
	if !syscmd.Exists("timedatectl") {
		return ApplyResult{Item: "timezone", OK: false, Message: "系统缺少 timedatectl，无法修改时区"}
	}
	res, err := syscmd.Run(context.Background(), 10*time.Second, "timedatectl", "set-timezone", tz)
	if err != nil {
		return ApplyResult{Item: "timezone", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())}
	}
	return ApplyResult{Item: "timezone", OK: true, Message: "时区已设置为 " + tz}
}

func applyNTP(enabled bool, servers string) ApplyResult {
	if err := syscmd.RequireLinux("NTP 设置"); err != nil {
		return ApplyResult{Item: "ntp", OK: false, Message: err.Error()}
	}
	// 写 chrony/ntp 配置（存在哪个就写哪个），再开启自动同步
	written := ""
	if list := strings.Split(servers, ","); len(list) > 0 {
		var sb strings.Builder
		for _, srv := range list {
			if srv = strings.TrimSpace(srv); srv != "" {
				sb.WriteString("server " + srv + " iburst\n")
			}
		}
		for _, p := range []string{"/etc/chrony/chrony.conf", "/etc/chrony.conf"} {
			if _, err := os.Stat(p); err == nil {
				body := "# 由设备管理平台写入\n" + sb.String() + "driftfile /var/lib/chrony/drift\nmakestep 1.0 3\nrtcsync\n"
				if err := os.WriteFile(p, []byte(body), 0o644); err == nil {
					written = p
				}
				break
			}
		}
	}
	if syscmd.Exists("timedatectl") {
		arg := "false"
		if enabled {
			arg = "true"
		}
		if res, err := syscmd.Run(context.Background(), 10*time.Second, "timedatectl", "set-ntp", arg); err != nil {
			return ApplyResult{Item: "ntp", OK: false, Message: firstNonEmpty(res.Combined(), err.Error())}
		}
	}
	if written != "" {
		_ = syscmd.Run2(context.Background(), "systemctl", "restart", "chrony")
		return ApplyResult{Item: "ntp", OK: true, Message: "NTP 服务器已写入 " + written}
	}
	return ApplyResult{Item: "ntp", OK: true, Message: "NTP 开关已下发"}
}

func applyDNS(servers string) ApplyResult {
	if err := syscmd.RequireLinux("DNS 配置"); err != nil {
		return ApplyResult{Item: "dns", OK: false, Message: err.Error()}
	}
	var sb strings.Builder
	for _, srv := range strings.Split(servers, ",") {
		if srv = strings.TrimSpace(srv); srv != "" {
			sb.WriteString("nameserver " + srv + "\n")
		}
	}
	if sb.Len() == 0 {
		return ApplyResult{Item: "dns", OK: false, Message: "DNS 服务器不能为空"}
	}
	body := "# 由设备管理平台写入\n" + sb.String()
	if err := os.WriteFile("/etc/resolv.conf", []byte(body), 0o644); err != nil {
		return ApplyResult{Item: "dns", OK: false, Message: "写入 /etc/resolv.conf 失败：" + err.Error()}
	}
	return ApplyResult{Item: "dns", OK: true, Message: "DNS 已更新（/etc/resolv.conf）"}
}

// ---------- 小工具 ----------

func parseKeyValue(out string) map[string]string {
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		i := strings.Index(line, "=")
		if i <= 0 {
			continue
		}
		kv[line[:i]] = strings.TrimSpace(line[i+1:])
	}
	return kv
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}


// tailFileLines 读取文件末尾 n 行。
func tailFileLines(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	const chunk = 64 * 1024
	size := stat.Size()
	offset := size
	var buf []byte
	found := 0
	for offset > 0 && found <= n {
		read := int64(chunk)
		if offset < read {
			read = offset
		}
		offset -= read
		part := make([]byte, read)
		if _, err := f.ReadAt(part, offset); err != nil {
			break
		}
		buf = append(part, buf...)
		found = strings.Count(string(buf), "\n")
	}
	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}

// DescribeSettingsDiff 生成设置变更的可读描述（写操作日志用）。
func DescribeSettingsDiff(old, new *DeviceSettings) string {
	var changes []string
	if old.Name != new.Name {
		changes = append(changes, fmt.Sprintf("设备名 %s → %s", old.Name, new.Name))
	}
	if old.Timezone != new.Timezone {
		changes = append(changes, fmt.Sprintf("时区 %s → %s", old.Timezone, new.Timezone))
	}
	if old.NTPEnabled != new.NTPEnabled {
		changes = append(changes, fmt.Sprintf("NTP %v → %v", old.NTPEnabled, new.NTPEnabled))
	}
	if old.DNS != new.DNS {
		changes = append(changes, fmt.Sprintf("DNS %s → %s", old.DNS, new.DNS))
	}
	return strings.Join(changes, "；")
}
