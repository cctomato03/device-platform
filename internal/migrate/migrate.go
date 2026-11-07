// Package migrate 极简版本化迁移执行器（内嵌 SQL，兼容 MySQL 5.7/8.0）。
// 不引入额外依赖：schema_migrations 表记录已执行版本，按文件名顺序执行未应用的 *.up.sql。
package migrate

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/migrations"
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
		"version VARCHAR(64) NOT NULL COMMENT '版本号'," +
		"name VARCHAR(255) NOT NULL DEFAULT '' COMMENT '文件名'," +
		"applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '执行时间'," +
		"PRIMARY KEY (version)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='迁移版本记录'"
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

// execScript 按分号拆分语句逐条执行（驱动默认不支持多语句）。
func execScript(db *gorm.DB, script string) error {
	for _, stmt := range splitStatements(script) {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%w | SQL: %s", err, truncate(stmt, 200))
		}
	}
	return nil
}

func splitStatements(script string) []string {
	raw := strings.Split(script, ";")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// 仅包含注释的空语句跳过
		if onlyComments(s) {
			continue
		}
		out = append(out, s)
	}
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
