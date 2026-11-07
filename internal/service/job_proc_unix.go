//go:build !windows

package service

import (
	"os/exec"
	"syscall"
)

// setProcessGroup 让脚本在独立进程组里运行，方便超时后连同子进程一起结束。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup 杀掉整个进程组（负号 PID）。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
