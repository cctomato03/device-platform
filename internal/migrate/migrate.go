// Package migrate 极简版本化迁移执行器（内嵌 SQL，SQLite 方言）。
// 不引入额外依赖：schema_migrations 表记录已执行版本，按文件名顺序执行未应用的 *.up.sql。
package migrate

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"device-platform/migrations"
)

const tableName = "schema_migrations"

// Run 执行全部未应用的迁移。
func Run(db *gorm.DB) error { return run(db, migrations.FS) }

func run(db *gorm.DB, mfs fs.FS) error {
	if err := ensureTable(db); err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	entries, err := fs.Glob(mfs, "*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)

	for _, name := range entries {
		version := strings.SplitN(name, "_", 2)[0]
		if applied[version] {
			continue
		}
		content, err := fs.ReadFile(mfs, name)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
		}
		if err := execScript(db, string(content)); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if err := db.Exec(
			"INSERT INTO "+tableName+" (version, name, applied_at) VALUES (?, ?, ?)",
			version, name, time.Now()).Error; err != nil {
			return fmt.Errorf("记录迁移版本 %s 失败: %w", version, err)
		}
	}
	return nil
}

func ensureTable(db *gorm.DB) error {
	ddl := "CREATE TABLE IF NOT EXISTS " + tableName + " (" +
		"version TEXT NOT NULL PRIMARY KEY, " +
		"name TEXT NOT NULL DEFAULT '', " +
		"applied_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')))"
	return db.Exec(ddl).Error
}

func appliedVersions(db *gorm.DB) (map[string]bool, error) {
	var rows []struct{ Version string }
	if err := db.Raw("SELECT version FROM " + tableName).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.Version] = true
	}
	return out, nil
}

// execScript 按分号拆分语句逐条执行（驱动不支持多语句）。
func execScript(db *gorm.DB, script string) error {
	for _, stmt := range splitStatements(script) {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%w | SQL: %s", err, truncate(stmt, 300))
		}
	}
	return nil
}

// splitStatements 按顶层分号拆分 SQL 脚本。
//
// 与简单 strings.Split 相比这里做了引号感知：单引号/双引号/反引号内部的分号
// 不会切断语句（种子数据里出现过含分号的文案），同时跳过纯注释片段。
func splitStatements(script string) []string {
	var (
		out    []string
		buf    strings.Builder
		quote  byte
		escape bool
	)
	flush := func() {
		s := strings.TrimSpace(buf.String())
		buf.Reset()
		if s == "" || onlyComments(s) {
			return
		}
		out = append(out, s)
	}
	for i := 0; i < len(script); i++ {
		ch := script[i]
		if escape {
			escape = false
			buf.WriteByte(ch)
			continue
		}
		switch {
		case quote != 0:
			if ch == '\\' && quote != '`' {
				escape = true
				buf.WriteByte(ch)
				continue
			}
			if ch == quote {
				quote = 0
			}
			buf.WriteByte(ch)
		case ch == '\'' || ch == '"' || ch == '`':
			quote = ch
			buf.WriteByte(ch)
		case ch == '-' && i+1 < len(script) && script[i+1] == '-':
			// 行注释：一直吞到行尾（保留换行便于错误定位）
			for i < len(script) && script[i] != '\n' {
				i++
			}
			buf.WriteByte('\n')
		case ch == ';':
			flush()
		default:
			buf.WriteByte(ch)
		}
	}
	flush()
	return out
}

func onlyComments(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		return false
	}
	return true
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
