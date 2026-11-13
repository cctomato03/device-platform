//go:build windows

package service

// isRoot Windows 下无 root 概念，按管理员以外的普通用户处理。
func isRoot() bool { return false }
