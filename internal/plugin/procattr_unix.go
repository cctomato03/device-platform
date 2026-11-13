//go:build !windows

package plugin

import (
	"os/exec"
	"syscall"
)

// applyProcAttr 让子进程独立成组，便于连同它的子进程一起停止。
func applyProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcess 先 SIGTERM 整个进程组，宽限期后仍存活则 SIGKILL。
func terminateProcess(pid int) error {
	// 负号表示进程组
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}
	}
	return nil
}
