//go:build windows

package service

import "os/exec"

// setProcessGroup Windows 无进程组概念，退化为单进程管理。
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup 直接结束进程（Windows 下不级联子进程）。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
