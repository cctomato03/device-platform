package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/pkg/errs"
)

// Shell 脚本型定时任务（job_type = 3）。
//
// 设计取舍（与业界常见做法一致）：
//   - 「调度」与「执行内容」解耦：cron 只描述何时跑，脚本/命令描述跑什么；
//   - 执行内容存在数据库（sys_job.params），运维改脚本不用发版；
//   - 但执行任意命令等于把服务器交给后台用户，所以必须有三道闸：
//     1) 配置文件 scheduler.shell_enabled 总开关（默认 false，部署时显式打开）；
//     2) 解释器白名单（只允许 sh/bash/python3，不允许直接执行二进制）；
//     3) 强制超时（job.timeout_sec，超时杀进程组）+ 输出截断，避免僵死或撑爆日志。

// shellInterpreters 允许的解释器白名单。
var shellInterpreters = map[string][]string{
	"sh":      {"sh", "-c"},
	"bash":    {"bash", "-c"},
	"python3": {"python3", "-c"},
}

// shellMaxOutput 执行输出写入日志的最大字节数（超出截断）。
const shellMaxOutput = 8 << 10

// ShellJobParams Shell 任务参数（存在 sys_job.params 的 JSON）。
type ShellJobParams struct {
	// Interpreter 解释器：sh / bash / python3，默认 sh
	Interpreter string `json:"interpreter"`
	// Script 脚本内容
	Script string `json:"script"`
	// Args 追加到脚本后的参数（可选）
	Args []string `json:"args"`
}

// parseShellParams 解析并校验 Shell 任务参数。
func parseShellParams(raw string) (*ShellJobParams, error) {
	p := &ShellJobParams{}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("Shell 任务缺少参数（需要 {\"interpreter\":\"sh\",\"script\":\"...\"}）")
	}
	if err := json.Unmarshal([]byte(raw), p); err != nil {
		return nil, fmt.Errorf("Shell 任务参数不是合法 JSON: %w", err)
	}
	p.Interpreter = strings.ToLower(strings.TrimSpace(p.Interpreter))
	if p.Interpreter == "" {
		p.Interpreter = "sh"
	}
	if _, ok := shellInterpreters[p.Interpreter]; !ok {
		return nil, fmt.Errorf("不支持的脚本解释器: %s（仅支持 sh/bash/python3）", p.Interpreter)
	}
	if strings.TrimSpace(p.Script) == "" {
		return nil, fmt.Errorf("脚本内容不能为空")
	}
	return p, nil
}

// ShellEnabled 是否允许执行 Shell 脚本任务（配置文件 scheduler.shell_enabled）。
func (s *Service) ShellEnabled() bool { return s.Cfg.Scheduler.ShellEnabled }

// execShellJob 执行 Shell 脚本任务。
func (s *Service) execShellJob(ctx context.Context, job *entity.Job) (string, error) {
	if !s.ShellEnabled() {
		return "", fmt.Errorf("Shell 任务未启用：请在配置文件中设置 scheduler.shell_enabled=true")
	}
	p, err := parseShellParams(job.Params)
	if err != nil {
		return "", err
	}

	timeout := time.Duration(defaultInt(job.TimeoutSec, 60)) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv := append([]string{}, shellInterpreters[p.Interpreter]...)
	argv = append(argv, p.Script)
	argv = append(argv, p.Args...)

	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("JOB_ID=%d", job.ID), fmt.Sprintf("JOB_NAME=%s", job.Name))
	setProcessGroup(cmd) // 超时时连同子进程一起结束

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("脚本启动失败: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-runCtx.Done():
		killProcessGroup(cmd) // 超时：连同子进程一起结束，避免留下僵死任务
		err = <-done
	}
	out := strings.TrimSpace(buf.String())
	if len(out) > shellMaxOutput {
		out = out[:shellMaxOutput] + "...(输出已截断)"
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return orDash(out), fmt.Errorf("脚本执行超时（%s）", timeout)
	}
	if err != nil {
		return orDash(out), fmt.Errorf("脚本执行失败: %w", err)
	}
	return orDash(out), nil
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(无输出)"
	}
	return s
}

// JobMetaResp 任务元信息（前端据此渲染表单，不需要把处理器列表写死在前端）。
type JobMetaResp struct {
	// Types 可选任务类型
	Types []JobTypeOption `json:"types"`
	// Handlers 已注册的内置处理器
	Handlers []JobHandlerOption `json:"handlers"`
	// ShellEnabled 是否允许 Shell 脚本任务
	ShellEnabled bool `json:"shellEnabled"`
	// Interpreters 允许的脚本解释器
	Interpreters []string `json:"interpreters"`
}

// JobTypeOption 任务类型选项。
type JobTypeOption struct {
	Value       int    `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// JobHandlerOption 内置处理器选项。
type JobHandlerOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// jobHandlerLabels 内置处理器的展示名称与说明（执行逻辑在 JobHandlers 注册表里）。
var jobHandlerLabels = map[string]JobHandlerOption{
	"cleanSession":  {Value: "cleanSession", Label: "清理过期登录会话", Description: "清理 Redis 中的过期会话与刷新令牌"},
	"cleanTempFile": {Value: "cleanTempFile", Label: "清理临时文件", Description: "删除存储目录里的临时上传文件"},
	"cleanOperLog":  {Value: "cleanOperLog", Label: "清理操作日志", Description: "按保留天数清理操作日志"},
	"cleanLoginLog": {Value: "cleanLoginLog", Label: "清理登录日志", Description: "按保留天数清理登录日志"},
}

// JobMeta 返回任务类型 / 内置处理器清单，供前端动态渲染。
func (s *Service) JobMeta() *JobMetaResp {
	resp := &JobMetaResp{ShellEnabled: s.ShellEnabled()}
	resp.Types = []JobTypeOption{
		{Value: 1, Label: "内置处理器", Description: "执行代码中已注册的处理器，适合与本服务强耦合的维护任务", Enabled: true},
		{Value: 2, Label: "HTTP 调用", Description: "定时请求一个外部地址，适合触发第三方系统的任务", Enabled: true},
		{
			Value:       3,
			Label:       "Shell 脚本",
			Description: "在服务器上执行 shell/python 脚本，适合自定义数据处理与运维动作",
			Enabled:     s.ShellEnabled(),
		},
	}
	resp.Interpreters = []string{"sh", "bash", "python3"}

	// 输出顺序稳定：先按内置顺序，再补注册表里没有登记名称的处理器
	order := []string{"cleanSession", "cleanTempFile", "cleanOperLog", "cleanLoginLog"}
	seen := map[string]bool{}
	for _, name := range order {
		if _, ok := JobHandlers[name]; !ok {
			continue
		}
		opt := jobHandlerLabels[name]
		opt.Value = name
		if opt.Label == "" {
			opt.Label = name
		}
		resp.Handlers = append(resp.Handlers, opt)
		seen[name] = true
	}
	for name := range JobHandlers {
		if seen[name] {
			continue
		}
		opt := jobHandlerLabels[name]
		opt.Value = name
		if opt.Label == "" {
			opt.Label = name
		}
		resp.Handlers = append(resp.Handlers, opt)
	}
	return resp
}

// validateShellJob 校验 Shell 任务是否可用（未开启时直接拒绝创建）。
func (s *Service) validateShellJob(payload *JobPayload) error {
	if !s.ShellEnabled() {
		return errs.Invalid("Shell 脚本任务未启用：请先在配置文件中打开 scheduler.shell_enabled 并重启服务")
	}
	if _, err := parseShellParams(defaultStr(payload.Params, "{}")); err != nil {
		return errs.Invalid(err.Error())
	}
	return nil
}
