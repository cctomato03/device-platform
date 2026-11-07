//go:build !embed

// Package webui 前端产物托管。默认从磁盘读取 web/dist（开发与前后端分离部署）。
// 构建时使用 `go build -tags embed` 切换为内嵌模式，产物打包进单一可执行文件。
package webui

import (
	"io/fs"
	"os"
)

// Dist 返回前端静态资源文件系统。
func Dist() (fs.FS, error) {
	dir := os.Getenv("WEB_DIST_DIR")
	if dir == "" {
		dir = "web/dist"
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, os.ErrNotExist
	}
	return os.DirFS(dir), nil
}
