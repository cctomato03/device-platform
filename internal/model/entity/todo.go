package entity

import "time"

// Todo 待办事项 sys_todo。
//
// 面向「每个用户」，因此按 user_id 归属；业务系统可通过开放接口推送，
// biz_key 作为幂等键保证重复推送不会重复建单。
type Todo struct {
	Base
	TenantID     uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	UserID       uint64     `gorm:"column:user_id" json:"userId,string"`
	Title        string     `gorm:"column:title" json:"title"`
	Content      string     `gorm:"column:content" json:"content"`
	Priority     int        `gorm:"column:priority" json:"priority"`
	Status       int        `gorm:"column:status" json:"status"`
	DueTime      *time.Time `gorm:"column:due_time" json:"dueTime"`
	CompleteTime *time.Time `gorm:"column:complete_time" json:"completeTime"`
	LinkURL      string     `gorm:"column:link_url" json:"linkUrl"`
	BizType      string     `gorm:"column:biz_type" json:"bizType"`
	BizID        string     `gorm:"column:biz_id" json:"bizId"`
	BizKey       *string    `gorm:"column:biz_key" json:"bizKey"`
	Source       int        `gorm:"column:source" json:"source"`
}

// TableName 表名。
func (Todo) TableName() string { return "sys_todo" }

// NoticeRead 通知已读记录 sys_notice_read。
type NoticeRead struct {
	NoticeID uint64    `gorm:"column:notice_id;primaryKey" json:"noticeId,string"`
	UserID   uint64    `gorm:"column:user_id;primaryKey" json:"userId,string"`
	ReadTime time.Time `gorm:"column:read_time" json:"readTime"`
}

// TableName 表名。
func (NoticeRead) TableName() string { return "sys_notice_read" }
