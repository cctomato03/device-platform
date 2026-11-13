// Package repository 数据访问层：复杂查询集中在此，业务逻辑放在 service。
package repository

import (
	"gorm.io/gorm"

	"device-platform/internal/config"
	"device-platform/pkg/cache"
)

// Repo 数据访问入口。
type Repo struct {
	DB    *gorm.DB
	Cache *cache.Cache
	Cfg   *config.Config
}

func New(db *gorm.DB, c *cache.Cache, cfg *config.Config) *Repo {
	return &Repo{DB: db, Cache: c, Cfg: cfg}
}

// Result 归一化写操作结果：
// 数据库错误原样返回；影响行数为 0 表示目标记录不存在，返回 gorm.ErrRecordNotFound，
// 由 errs.Normalize 统一转换成 10004/404，避免"修改了不存在的记录却返回成功"。
//
// 依赖 DSN 中的 clientFoundRows=true：该参数让驱动返回"匹配行数"而不是"实际变更行数"，
// 否则把字段更新成相同值时也会被判为 0 行（误报不存在）。
func Result(res *gorm.DB) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
