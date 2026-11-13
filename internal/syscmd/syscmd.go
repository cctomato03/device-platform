// Package syscmd 受控的系统命令执行封装。
//
// 设备平台大量功能（时间同步、路由下发、抓包排查）都要调用操作系统命令，
// 这里统一做四件事：超时控制、输出采集、非零退出码可读化、命令存在性检查。
// 命令白名单由调用方（service 层）保证——本包不做业务判断。
package syscmd

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Result 命令执行结果。
type Result struct {
	Cmd      string `json:"cmd"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	CostMS   int64  `json:"costMs"`
}

// Combined 合并输出（排查工具展示用）。
func (r Result) Combined() string {
	out := r.Stdout
	if strings.TrimSpace(r.Stderr) != "" {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += r.Stderr
	}
	return out
}

// Run 执行命令并等待完成。
func Run(ctx context.Context, timeout time.Duration, name string, args ...string) (Result, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(cctx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{
		Cmd:    name + " " + strings.Join(args, " "),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		CostMS: time.Since(start).Milliseconds(),
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if cctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("命令执行超时（%s）", timeout)
	}
	return res, err
}

// Stream 逐行回调执行输出（前端用 SSE 实时展示 ping/traceroute 结果）。
func Stream(ctx context.Context, timeout time.Duration, name string, args []string, onLine func(string)) (Result, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(cctx, name, args...)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	var all strings.Builder
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		all.WriteString(line + "\n")
		if onLine != nil {
			onLine(line)
		}
	}
	_ = cmd.Wait()
	res := Result{
		Cmd:    name + " " + strings.Join(args, " "),
		Stdout: all.String(),
		CostMS: time.Since(start).Milliseconds(),
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if cctx.Err() == context.DeadlineExceeded && onLine != nil {
		onLine(fmt.Sprintf("--- 已到达超时时间 %s，输出结束 ---", timeout))
	}
	return res, nil
}

// Exists 命令是否存在于 PATH。
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Run2 执行并忽略输出（只关心成功与否，用于 systemctl restart 这类操作）。
func Run2(ctx context.Context, name string, args ...string) error {
	_, err := Run(ctx, 30*time.Second, name, args...)
	return err
}

// IsLinux 当前是否为 Linux（设备端为 Linux，开发机可能是 Windows/macOS）。
func IsLinux() bool { return runtime.GOOS == "linux" }

// RequireLinux 非 Linux 环境直接给出可读错误，避免前端看到奇怪的执行失败。
func RequireLinux(feature string) error {
	if !IsLinux() {
		return fmt.Errorf("%s 依赖 Linux 系统能力，当前运行环境为 %s，暂不可用", feature, runtime.GOOS)
	}
	return nil
}
