// Package database 数据库与缓存连接初始化。
package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"web-basic/internal/config"
)

// OpenMySQL 初始化 MySQL 连接池。
func OpenMySQL(cfg config.MySQL) (*gorm.DB, error) {
	level := glog.Warn
	if cfg.LogSQL {
		level = glog.Info
	}
	slow := time.Duration(cfg.SlowThresholdMS) * time.Millisecond
	if slow <= 0 {
		slow = 200 * time.Millisecond
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		SkipDefaultTransaction: true, // 单条写入无需事务，性能更好
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
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	lifetime := time.Duration(cfg.ConnMaxLifetime) * time.Minute
	if lifetime <= 0 {
		lifetime = time.Hour
	}
	sqlDB.SetConnMaxLifetime(lifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("MySQL 探活失败: %w", err)
	}
	return db, nil
}
