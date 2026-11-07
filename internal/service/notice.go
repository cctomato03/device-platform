package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/audit"
	"web-basic/internal/model/entity"
	"web-basic/internal/model/scope"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
)

// NoticeQuery 公告列表查询。
type NoticeQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Keyword  string `form:"keyword"` // 标题模糊
	Type     *int   `form:"type"`
	Status   *int   `form:"status"`
}

func (q *NoticeQuery) normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 200 {
		q.PageSize = 200
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
}

// NoticeReq 公告新增/修改入参。
type NoticeReq struct {
	Title   string `json:"title" binding:"required,max=200"`
	Type    int    `json:"type" binding:"oneof=1 2 3"`
	Level   int    `json:"level" binding:"oneof=1 2 3"`
	Content string `json:"content" binding:"max=20000"`
	Remark  string `json:"remark" binding:"max=500"`
}

// ListNotices 公告分页列表（租户内）。
func (s *Service) ListNotices(ctx context.Context, q *NoticeQuery, p *principal.Info) ([]entity.Notice, int64, error) {
	q.normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Notice{}).Scopes(scope.Tenant(p.TenantID))
	if q.Keyword != "" {
		tx = tx.Where("title LIKE ?", q.Keyword+"%")
	}
	if q.Type != nil {
		tx = tx.Where("type = ?", *q.Type)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Notice, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// NoticeDetail 公告详情。
func (s *Service) NoticeDetail(ctx context.Context, noticeID uint64, p *principal.Info) (*entity.Notice, error) {
	var n entity.Notice
	err := s.Repo.DB.WithContext(ctx).Scopes(scope.Tenant(p.TenantID)).Where("id = ?", noticeID).Limit(1).Find(&n).Error
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if n.ID == 0 {
		return nil, errs.ErrNotFound
	}
	return &n, nil
}

// CreateNotice 新增公告（默认草稿）。
func (s *Service) CreateNotice(ctx context.Context, req *NoticeReq, p *principal.Info) (uint64, error) {
	now := time.Now()
	n := &entity.Notice{
		Base: entity.Base{
			ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark,
		},
		Title: req.Title, Type: defaultInt(req.Type, 1), Level: defaultInt(req.Level, 1),
		Content: req.Content, Status: entity.NoticeStatusDraft, TenantID: p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(n).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return n.ID, nil
}

// UpdateNotice 修改公告（已关闭的公告不允许再改，避免"发布过的公告被静默篡改"）。
func (s *Service) UpdateNotice(ctx context.Context, noticeID uint64, req *NoticeReq, p *principal.Info) error {
	cur, err := s.NoticeDetail(ctx, noticeID, p)
	if err != nil {
		return err
	}
	if cur.Status == entity.NoticeStatusClosed {
		return errs.InvalidErr(errors.New("公告已关闭，不能再修改"))
	}
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Notice{}, noticeID, map[string]any{
		"title": req.Title, "type": req.Type, "level": req.Level, "content": req.Content,
		"remark": req.Remark, "update_by": p.UserID, "update_time": time.Now(),
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeleteNotice 删除公告（同时清理其站内信，避免用户点开 404）。
func (s *Service) DeleteNotice(ctx context.Context, noticeID uint64, p *principal.Info) error {
	if _, err := s.NoticeDetail(ctx, noticeID, p); err != nil {
		return err
	}
	return s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.Result(tx.Delete(&entity.Notice{}, noticeID)); err != nil {
			return err
		}
		return tx.Where("notice_id = ?", noticeID).Delete(&entity.Message{}).Error
	})
}

// PublishNotice 发布公告：状态置为已发布 + 扇出站内信给本租户所有启用用户。
// 重复发布会刷新发布时间并补齐缺失的站内信（幂等：用 notice_id + user_id 去重）。
func (s *Service) PublishNotice(ctx context.Context, noticeID uint64, p *principal.Info) (int, error) {
	n, err := s.NoticeDetail(ctx, noticeID, p)
	if err != nil {
		return 0, err
	}
	if n.Status == entity.NoticeStatusClosed {
		return 0, errs.InvalidErr(errors.New("公告已关闭，不能发布"))
	}
	now := time.Now()
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Model(&entity.Notice{}).Where("id = ?", noticeID).Updates(map[string]any{
		"status":       entity.NoticeStatusPublished,
		"publish_time": now,
		"publisher_id": p.UserID,
		"publisher":    p.Username,
		"update_by":    p.UserID,
		"update_time":  now,
	})); err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}

	// 扇出：本租户启用用户 → 站内信
	var userIDs []uint64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.User{}).
		Where("tenant_id = ? AND status = 1 AND deleted = 0", n.TenantID).
		Pluck("id", &userIDs).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if len(userIDs) == 0 {
		return 0, nil
	}
	var exists []uint64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
		Where("notice_id = ? AND deleted = 0", noticeID).Pluck("user_id", &exists).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	done := make(map[uint64]bool, len(exists))
	for _, uid := range exists {
		done[uid] = true
	}
	rows := make([]entity.Message, 0, len(userIDs))
	for _, uid := range userIDs {
		if done[uid] {
			continue
		}
		rows = append(rows, entity.Message{
			ID: id.Next(), UserID: uid, NoticeID: noticeID, Title: n.Title, Content: summarize(n.Content),
			Type: 2, IsRead: 0, TenantID: n.TenantID, CreateTime: now,
		})
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := s.Repo.DB.WithContext(ctx).CreateInBatches(&rows, 200).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	audit.Note(ctx, []audit.Change{{Field: "status", Before: "草稿", After: "已发布"}, {Field: "站内信推送人数", Before: "", After: strconv.Itoa(len(rows))}})
	return len(rows), nil
}

// summarize 站内信只带摘要（正文前 500 字），避免站内信表被大正文撑爆。
func summarize(s string) string {
	s = strings.TrimSpace(s)
	const limit = 500
	if len([]rune(s)) <= limit {
		return s
	}
	return string([]rune(s)[:limit]) + "..."
}
