package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"device-platform/internal/syscmd"
)

// 本文件：系统依赖包与服务控制的统一封装。
//
// 设备平台很多能力（dnsmasq 提供 DHCP/DNS、FRR 提供 BGP/OSPF、net-snmp 提供 SNMP）
// 依赖系统包，这里把「装包 / 启停服务 / 查状态」收敛成三个函数，
// 保证不同模块给用户的提示口径一致，并且都能在非 Linux/非 root 环境下给出可读原因。

// serviceReady 检查 systemd 服务是否处于 active。
func serviceReady(ctx context.Context, name string) bool {
	if !syscmd.IsLinux() || !syscmd.Exists("systemctl") {
		return false
	}
	res, err := syscmd.Run(ctx, 5*time.Second, "systemctl", "is-active", name)
	return err == nil && strings.TrimSpace(res.Stdout) == "active"
}

// restartService 重启服务并等待片刻确认结果。
func restartService(ctx context.Context, name string) error {
	res, err := syscmd.Run(ctx, 30*time.Second, "systemctl", "restart", name)
	if err != nil {
		return fmt.Errorf("%s", firstNonEmpty(res.Combined(), err.Error()))
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

// stopService 停止服务（关闭功能时调用）。
func stopService(ctx context.Context, name string) error {
	if !syscmd.IsLinux() || !syscmd.Exists("systemctl") {
		return nil
	}
	res, err := syscmd.Run(ctx, 30*time.Second, "systemctl", "stop", name)
	if err != nil {
		return fmt.Errorf("%s", firstNonEmpty(res.Combined(), err.Error()))
	}
	return nil
}

// installPackages 安装系统包（apt 优先，yum/dnf 兜底）。
//
// 返回逐项结果，便于前端展示「装到哪一步失败了」。
func (s *Service) installPackages(ctx context.Context, pkgs ...string) []ApplyResult {
	label := "install " + strings.Join(pkgs, " ")
	if err := syscmd.RequireLinux("安装系统包"); err != nil {
		return []ApplyResult{{Item: label, OK: false, Message: err.Error()}}
	}
	if !isRoot() {
		return []ApplyResult{{Item: label, OK: false, Message: "需要 root 权限（可用 install.sh --user root 重装服务）"}}
	}
	switch {
	case syscmd.Exists("apt-get"):
		res, err := syscmd.Run(ctx, 600*time.Second, "apt-get", append([]string{"install", "-y"}, pkgs...)...)
		if err != nil {
			return []ApplyResult{{Item: label, OK: false, Message: firstNonEmpty(tail(res.Combined(), 400), err.Error())}}
		}
		return []ApplyResult{{Item: label, OK: true, Message: "安装完成"}}
	case syscmd.Exists("dnf"):
		res, err := syscmd.Run(ctx, 600*time.Second, "dnf", append([]string{"install", "-y"}, pkgs...)...)
		if err != nil {
			return []ApplyResult{{Item: label, OK: false, Message: firstNonEmpty(tail(res.Combined(), 400), err.Error())}}
		}
		return []ApplyResult{{Item: label, OK: true, Message: "安装完成"}}
	case syscmd.Exists("yum"):
		res, err := syscmd.Run(ctx, 600*time.Second, "yum", append([]string{"install", "-y"}, pkgs...)...)
		if err != nil {
			return []ApplyResult{{Item: label, OK: false, Message: firstNonEmpty(tail(res.Combined(), 400), err.Error())}}
		}
		return []ApplyResult{{Item: label, OK: true, Message: "安装完成"}}
	}
	return []ApplyResult{{Item: label, OK: false, Message: "系统缺少 apt-get/dnf/yum，请手动安装：" + strings.Join(pkgs, ", ")}}
}

// tail 取字符串末尾 n 个字符（安装日志通常很长，只回显关键部分）。
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

// serviceLogHint 服务启动失败时补一段 journalctl 尾部，方便直接看到根因
// （例如 dnsmasq 的「port 53: Address already in use」）。
func serviceLogHint(ctx context.Context, name string) string {
	if !syscmd.IsLinux() || !syscmd.Exists("journalctl") {
		return ""
	}
	res, err := syscmd.Run(ctx, 8*time.Second, "journalctl", "-u", name, "-n", "8", "--no-pager")
	if err != nil {
		return ""
	}
	lines := splitLines(res.Stdout)
	if len(lines) == 0 {
		return ""
	}
	return "\n最近日志：\n" + strings.Join(limitLines(lines, 8), "\n")
}

// backupFile 写入前备份配置文件，返回备份路径（不存在时返回空串）。
func backupFile(src, backupDir string) string {
	return fileBackup(src, backupDir)
}
