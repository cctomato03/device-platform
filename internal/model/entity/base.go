// Package entity 数据库实体定义（与 migrations 中的建表语句严格对应）。
package entity

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// Base 公共审计字段。软删除使用 deleted 时间戳（0 表示未删除），
// 与唯一索引组合可避免"软删除后唯一键冲突"问题。
type Base struct {
	ID         uint64                `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	CreateBy   uint64                `gorm:"column:create_by" json:"createBy,string"`
	CreateTime time.Time             `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateBy   uint64                `gorm:"column:update_by" json:"updateBy,string"`
	UpdateTime *time.Time            `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	Deleted    soft_delete.DeletedAt `gorm:"column:deleted;softDelete:milli" json:"-"`
	Remark     string                `gorm:"column:remark" json:"remark"`
}

// BaseLite 精简审计字段（部分表无 create_by/update_by 列时使用）。
type BaseLite struct {
	ID         uint64                `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	CreateTime time.Time             `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime *time.Time            `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
	Deleted    soft_delete.DeletedAt `gorm:"column:deleted;softDelete:milli" json:"-"`
	Remark     string                `gorm:"column:remark" json:"remark"`
}

// PageQuery 通用分页查询参数。
type PageQuery struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	OrderBy  string `form:"orderBy" json:"orderBy"`
	Order    string `form:"order" json:"order"`
}

// Normalize 规范化分页参数（默认 1/10，最大 200）。
func (q *PageQuery) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 200 {
		q.PageSize = 200
	}
}

// Offset 计算偏移量。
func (q *PageQuery) Offset() int { return (q.Page - 1) * q.PageSize }
