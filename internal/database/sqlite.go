// Package database 提供嵌入式 SQLite 连接初始化。
//
// 采用纯 Go 驱动（github.com/glebarez/sqlite → modernc.org/sqlite），
// 无需 cgo，可直接交叉编译到 linux/arm64、linux/amd64 并塞进设备镜像。
package database

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"device-platform/internal/config"
)

// OpenSQLite 初始化 SQLite 连接池。
//
// 关键 DSN 参数：
//   - _pragma=journal_mode(WAL)  读写并发不互相阻塞（设备上常并发读写日志）
//   - _pragma=busy_timeout(..)   锁等待，避免瞬时 SQLITE_BUSY
//   - _pragma=synchronous(NORMAL) WAL 下兼顾安全与写性能
//   - _timezone / _time_format   统一时间读写格式，避免时间字段解析错位
//   - _txlock=immediate          写事务立即取写锁，减少死锁重试
func OpenSQLite(cfg config.Database, timezone string) (*gorm.DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("database.path 不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	level := glog.Warn
	if cfg.LogSQL {
		level = glog.Info
	}
	slow := time.Duration(cfg.SlowThresholdMS) * time.Millisecond
	if slow <= 0 {
		slow = 200 * time.Millisecond
	}

	busy := cfg.BusyTimeoutMS
	if busy <= 0 {
		busy = 10000
	}
	journal := "WAL"
	if !cfg.WAL {
		journal = "DELETE"
	}

	dsn := "file:" + filepath.ToSlash(cfg.Path) + "?" + url.Values{
		"_pragma":       {"journal_mode(" + journal + ")", "busy_timeout(" + fmt.Sprint(busy) + ")", "synchronous(NORMAL)", "foreign_keys(1)"},
		"_timezone":     {timezone},
		"_time_format":  {"sqlite"},
		"_txlock":       {"immediate"},
		"_busy_timeout": {fmt.Sprint(busy)},
	}.Encode()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 表名不加复数：sys_user 而非 sys_users
		},
		NowFunc: func() time.Time { return time.Now().Local() },
		Logger: glog.New(log.New(os.Stdout, "[GORM] ", log.LstdFlags), glog.Config{
			SlowThreshold:             slow,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	// SQLite 单文件写入，连接数过多反而增加锁竞争；
	// 单连接即可满足设备场景，同时保证内存表/临时表在同一连接内可见。
	maxOpen := cfg.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 1
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxOpen)
	sqlDB.SetConnMaxLifetime(0)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("SQLite 探活失败: %w", err)
	}
	return db, nil
}
