package entity

import "time"

// Notice 通知公告 sys_notice
type Notice struct {
	Base
	Title       string     `gorm:"column:title" json:"title"`
	Type        int        `gorm:"column:type" json:"type"`   // 1 通知 2 公告 3 维护
	Level       int        `gorm:"column:level" json:"level"` // 1 普通 2 重要 3 紧急
	Content     string     `gorm:"column:content" json:"content"`
	Status      int        `gorm:"column:status" json:"status"` // 0 草稿 1 已发布 2 已关闭
	PublisherID uint64     `gorm:"column:publisher_id" json:"publisherId,string"`
	Publisher   string     `gorm:"column:publisher" json:"publisher"`
	PublishTime *time.Time `gorm:"column:publish_time" json:"publishTime"`
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Notice) TableName() string { return "sys_notice" }

// 公告状态
const (
	NoticeStatusDraft     = 0
	NoticeStatusPublished = 1
	NoticeStatusClosed    = 2
)

// Message 站内信 sys_message
type Message struct {
	ID         uint64     `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	UserID     uint64     `gorm:"column:user_id" json:"userId,string"`
	NoticeID   uint64     `gorm:"column:notice_id" json:"noticeId,string"`
	Title      string     `gorm:"column:title" json:"title"`
	Content    string     `gorm:"column:content" json:"content"`
	Type       int        `gorm:"column:type" json:"type"` // 1 系统 2 公告 3 任务
	BizType    string     `gorm:"column:biz_type" json:"bizType"`
	BizKey     *string    `gorm:"column:biz_key" json:"bizKey"`
	IsRead     int        `gorm:"column:is_read" json:"isRead"`
	ReadTime   *time.Time `gorm:"column:read_time" json:"readTime"`
	TenantID   uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CreateTime time.Time  `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	Deleted    int64      `gorm:"column:deleted;softDelete:milli" json:"-"`
	Remark     string     `gorm:"column:remark" json:"remark"`
}

func (Message) TableName() string { return "sys_message" }
