//go:build embed

// Package webui 前端产物托管（内嵌模式：go build -tags embed）。
// 构建前需将 web/dist 复制到 internal/webui/dist。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist 返回内嵌的前端静态资源。
func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
