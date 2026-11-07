// Package migrations 内嵌 SQL 迁移脚本，编译进二进制，部署时无需额外文件。
package migrations

import "embed"

// FS 包含全部 .sql 迁移脚本。
//
//go:embed *.sql
var FS embed.FS
