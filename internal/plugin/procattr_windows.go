//go:build windows

package plugin

import (
	"os/exec"
	"strconv"
)

// applyProcAttr Windows 下无需进程组设置。
func applyProcAttr(cmd *exec.Cmd) {}

// terminateProcess Windows 下用 taskkill 连子进程一起结束。
func terminateProcess(pid int) error {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := kill.Run(); err != nil {
		return err
	}
	return nil
}
