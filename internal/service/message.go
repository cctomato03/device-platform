package service

import (
	"context"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/principal"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// MessageQuery 我的站内信查询。
type MessageQuery struct {
	Page     int  `form:"page"`
	PageSize int  `form:"pageSize"`
	IsRead   *int `form:"isRead"`
	Type     *int `form:"type"`
}

func (q *MessageQuery) normalize() {
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

// ListMyMessages 我的站内信分页（只返回当前登录用户的）。
func (s *Service) ListMyMessages(ctx context.Context, q *MessageQuery, p *principal.Info) ([]entity.Message, int64, error) {
	q.normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).Where("user_id = ?", p.UserID)
	if q.IsRead != nil {
		tx = tx.Where("is_read = ?", *q.IsRead)
	}
	if q.Type != nil {
		tx = tx.Where("type = ?", *q.Type)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Message, 0, q.PageSize)
	if err := tx.Order("is_read ASC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// UnreadMessageCount 未读数量（顶栏红点用）。
func (s *Service) UnreadMessageCount(ctx context.Context, p *principal.Info) (int64, error) {
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
		Where("user_id = ? AND is_read = 0", p.UserID).Count(&n).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return n, nil
}

// MarkMessageRead 标记单条已读（只能操作自己的消息）。
func (s *Service) MarkMessageRead(ctx context.Context, messageID uint64, p *principal.Info) error {
	now := time.Now()
	res := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
		Where("id = ? AND user_id = ?", messageID, p.UserID).
		Updates(map[string]any{"is_read": 1, "read_time": now})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// MarkAllMessagesRead 全部标记已读。
func (s *Service) MarkAllMessagesRead(ctx context.Context, p *principal.Info) (int64, error) {
	now := time.Now()
	res := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
		Where("user_id = ? AND is_read = 0", p.UserID).
		Updates(map[string]any{"is_read": 1, "read_time": now})
	if res.Error != nil {
		return 0, errs.ErrDB.WithErr(res.Error)
	}
	return res.RowsAffected, nil
}

// DeleteMyMessage 删除我的站内信（软删除，只能删自己的）。
func (s *Service) DeleteMyMessage(ctx context.Context, messageID uint64, p *principal.Info) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ? AND user_id = ?", messageID, p.UserID).Delete(&entity.Message{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// SendSystemMessage 给指定用户发送系统消息（供业务层复用，例如任务失败告警）。
func (s *Service) SendSystemMessage(ctx context.Context, userID uint64, tenantID uint64, title, content string) error {
	msg := &entity.Message{
		ID: id.Next(), UserID: userID, Title: title, Content: summarize(content),
		Type: 1, IsRead: 0, TenantID: tenantID, CreateTime: time.Now(),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(msg).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}
