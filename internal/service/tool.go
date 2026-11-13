package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
)

// ToolReq 排查工具请求。
type ToolReq struct {
	// Action ping | traceroute | netstat | tcpdump
	Action string `json:"action"`
	// Target 目标地址（ping/traceroute）或抓包过滤表达式（tcpdump）
	Target string `json:"target"`
	// Count 次数/包数
	Count int `json:"count"`
	// Iface 网卡（tcpdump 使用）
	Iface string `json:"iface"`
	// Port 端口（netstat 过滤，可选）
	Port int `json:"port"`
	// Proto 协议（tcpdump 过滤，可选）
	Proto string `json:"proto"`
}

// ToolResult 排查结果。
type ToolResult struct {
	Action   string `json:"action"`
	Command  string `json:"command"`
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
	CostMS   int64  `json:"costMs"`
	Error    string `json:"error,omitempty"`
}

// 目标地址白名单：只允许主机名/IP/IPv6 字符，禁止任何可以注入参数的内容。
var targetRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._:-]{0,252})$`)

// 网卡名白名单。
var ifaceRe = regexp.MustCompile(`^[A-Za-z0-9._@-]{1,32}$`)

// RunTool 执行排查命令（同步返回结果，最长 60 秒）。
func (s *Service) RunTool(ctx context.Context, req *ToolReq) (*ToolResult, error) {
	if !s.Cfg.System.ToolsEnabled {
		return nil, errs.Invalid("设备排查工具已在配置中关闭（system.tools_enabled）")
	}
	name, args, err := s.buildToolCommand(req)
	if err != nil {
		return nil, err
	}
	timeout := 30 * time.Second
	if req.Action == "traceroute" {
		timeout = 60 * time.Second
	}
	res, runErr := syscmd.Run(ctx, timeout, name, args...)
	out := res.Combined()
	if runErr != nil && res.ExitCode == 0 && out == "" {
		out = runErr.Error()
	}
	tr := &ToolResult{
		Action:   req.Action,
		Command:  res.Cmd,
		Output:   out,
		ExitCode: res.ExitCode,
		CostMS:   res.CostMS,
	}
	if runErr != nil {
		tr.Error = runErr.Error()
	}
	return tr, nil
}

// StreamTool 流式执行（SSE 实时输出，ping/traceroute 体验更好）。
func (s *Service) StreamTool(ctx context.Context, req *ToolReq, w io.Writer, flush func()) error {
	if !s.Cfg.System.ToolsEnabled {
		return errs.Invalid("设备排查工具已在配置中关闭（system.tools_enabled）")
	}
	name, args, err := s.buildToolCommand(req)
	if err != nil {
		return err
	}
	timeout := 60 * time.Second
	if req.Action == "traceroute" {
		timeout = 90 * time.Second
	}
	send := func(line string) {
		b, _ := json.Marshal(map[string]string{"line": line})
		_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
		if flush != nil {
			flush()
		}
	}
	send(fmt.Sprintf("执行：%s %s", name, strings.Join(args, " ")))
	res, _ := syscmd.Stream(ctx, timeout, name, args, func(line string) {
		send(line)
	})
	send(fmt.Sprintf("--- 结束（exit=%d，耗时 %d ms）", res.ExitCode, res.CostMS))
	_, _ = w.Write([]byte("event: done\ndata: {}\n\n"))
	if flush != nil {
		flush()
	}
	return nil
}

// buildToolCommand 依据白名单构造命令与参数。
func (s *Service) buildToolCommand(req *ToolReq) (string, []string, error) {
	allow := s.Cfg.System.Tools
	switch req.Action {
	case "ping":
		if !allow.Ping {
			return "", nil, errs.Invalid("ping 工具已关闭")
		}
		target, err := validateTarget(req.Target)
		if err != nil {
			return "", nil, err
		}
		count := clamp(req.Count, 1, 20, 4)
		if syscmd.IsLinux() {
			return "ping", []string{"-c", fmt.Sprint(count), "-W", "2", target}, nil
		}
		return "ping", []string{"-n", fmt.Sprint(count), target}, nil

	case "traceroute":
		if !allow.Traceroute {
			return "", nil, errs.Invalid("traceroute 工具已关闭")
		}
		target, err := validateTarget(req.Target)
		if err != nil {
			return "", nil, err
		}
		if syscmd.Exists("traceroute") {
			return "traceroute", []string{"-m", "20", "-w", "2", "-q", "1", target}, nil
		}
		if syscmd.Exists("tracepath") {
			return "tracepath", []string{"-m", "20", target}, nil
		}
		return "", nil, errs.Invalid("系统未安装 traceroute/tracepath")

	case "netstat":
		if !allow.Netstat {
			return "", nil, errs.Invalid("netstat 工具已关闭")
		}
		if syscmd.Exists("ss") {
			args := []string{"-tunlp"}
			if req.Port > 0 {
				args = append(args, "sport", "=", fmt.Sprint(req.Port))
			}
			return "ss", args, nil
		}
		return "netstat", []string{"-tunlp"}, nil

	case "tcpdump":
		if !allow.Tcpdump {
			return "", nil, errs.Invalid("tcpdump 工具已关闭")
		}
		if !syscmd.Exists("tcpdump") {
			return "", nil, errs.Invalid("系统未安装 tcpdump（apt install tcpdump）")
		}
		iface := strings.TrimSpace(req.Iface)
		if iface == "" {
			return "", nil, errs.Invalid("请选择要抓包的网卡")
		}
		if !ifaceRe.MatchString(iface) {
			return "", nil, errs.Invalid("网卡名非法")
		}
		count := clamp(req.Count, 1, 200, 20)
		args := []string{"-i", iface, "-nn", "-c", fmt.Sprint(count)}
		// 过滤条件做严格白名单：仅允许 host/port/proto 关键字的简单表达式
		if f := strings.TrimSpace(req.Target); f != "" {
			if !safeFilterRe.MatchString(f) {
				return "", nil, errs.Invalid("抓包过滤表达式包含非法字符（仅支持 host/port/tcp/udp/icmp/and/or/not 与地址端口）")
			}
			args = append(args, strings.Fields(f)...)
		}
		return "tcpdump", args, nil
	}
	return "", nil, errs.Invalid("不支持的排查动作：" + req.Action)
}

// safeFilterRe tcpdump 过滤表达式白名单。
var safeFilterRe = regexp.MustCompile(`^[A-Za-z0-9 .:/_\-]{1,120}$`)

func validateTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errs.Invalid("请填写目标地址")
	}
	if len(target) > 253 || !targetRe.MatchString(target) {
		return "", errs.Invalid("目标地址非法（仅支持域名或 IP）")
	}
	return target, nil
}

func clamp(v, min, max, def int) int {
	if v <= 0 {
		v = def
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

// ToolAvailability 工具可用性（前端据此置灰按钮）。
type ToolAvailability struct {
	Enabled    bool `json:"enabled"`
	Ping       bool `json:"ping"`
	Traceroute bool `json:"traceroute"`
	Tcpdump    bool `json:"tcpdump"`
	Netstat    bool `json:"netstat"`
	// Root 是否具备 root 权限（tcpdump 需要）
	Root bool `json:"root"`
}

// ToolMeta 返回工具可用性。
func (s *Service) ToolMeta() ToolAvailability {
	sys := s.Cfg.System
	return ToolAvailability{
		Enabled:    sys.ToolsEnabled,
		Ping:       sys.Tools.Ping && syscmd.Exists("ping"),
		Traceroute: sys.Tools.Traceroute && (syscmd.Exists("traceroute") || syscmd.Exists("tracepath")),
		Tcpdump:    sys.Tools.Tcpdump && syscmd.Exists("tcpdump"),
		Netstat:    sys.Tools.Netstat && (syscmd.Exists("ss") || syscmd.Exists("netstat")),
		Root:       isRoot(),
	}
}
