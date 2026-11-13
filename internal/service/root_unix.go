//go:build !windows

package service

import "os"

// isRoot 当前进程是否以 root 运行（tcpdump / 防火墙下发需要）。
func isRoot() bool { return os.Geteuid() == 0 }
