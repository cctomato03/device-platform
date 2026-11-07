package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/model/scope"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
)

// 工作中心：待办 / 我的消息 / 我的通知，以及供业务系统对接的入站推送。
//
// 设计要点（与专业系统的做法一致）：
//  1. 「通知/消息/待办」面向每个用户，因此挂在独立目录（工作中心），而不是系统设置；
//  2. 业务系统对接统一走 Push 入站接口：显式指定接收人 + biz_type/biz_key 幂等键，
//     重复推送不会重复建单，重试安全；
//  3. 所有查询强制带 tenant_id + user_id，用户只能看到自己的数据（不依赖前端传参）；
//  4. 待办保留 biz_type/biz_id/link_url，点击可回到业务系统对应单据，形成「待办 → 业务」闭环。

// 待办状态
const (
	TodoStatusPending   = 1
	TodoStatusCompleted = 2
	TodoStatusCanceled  = 3
)

// 待办来源
const (
	TodoSourceManual = 1 // 后台手工
	TodoSourceSystem = 2 // 系统内部推送
	TodoSourceOpen   = 3 // 开放接口（业务系统）
)

// TodoQuery 我的待办查询。
type TodoQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Status   *int   `form:"status"`
	Keyword  string `form:"keyword"`
}

func (q *TodoQuery) normalize() {
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

// TodoReq 待办新增入参（后台手工 / 开放接口共用）。
type TodoReq struct {
	// UserID 接收人（开放接口必须；后台默认当前用户）
	UserID uint64 `json:"userId,string"`
	Title  string `json:"title" binding:"required,max=200"`
	// Content 详情
	Content string `json:"content" binding:"max=2000"`
	// Priority 1 低 2 普通 3 高 4 紧急
	Priority int `json:"priority" binding:"omitempty,oneof=1 2 3 4"`
	// DueTime 截止时间（RFC3339 或 2006-01-02 15:04:05）
	DueTime string `json:"dueTime" binding:"max=32"`
	// LinkURL 关联业务地址
	LinkURL string `json:"linkUrl" binding:"max=500"`
	// BizType/BizID/BizKey 业务标识与幂等键
	BizType string `json:"bizType" binding:"max=32"`
	BizID   string `json:"bizId" binding:"max=64"`
	BizKey  string `json:"bizKey" binding:"max=128"`
}

// parseDueTime 解析截止时间，空值表示无截止。
func parseDueTime(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, errs.Invalid("截止时间格式不正确（应为 2026-01-31 或 2026-01-31 18:00:00）")
}

// ListMyTodos 我的待办列表。
func (s *Service) ListMyTodos(ctx context.Context, q *TodoQuery, p *principal.Info) ([]entity.Todo, int64, error) {
	q.normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Todo{}).
		Where("tenant_id = ? AND user_id = ?", p.TenantID, p.UserID)
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.Keyword != "" {
		tx = tx.Where("title LIKE ?", q.Keyword+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Todo, 0, q.PageSize)
	if err := tx.Order("status ASC, priority DESC, due_time IS NULL, due_time ASC, id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// TodoSummary 待办统计（首页/红点用）。
type TodoSummary struct {
	Pending   int64 `json:"pending"`
	Overdue   int64 `json:"overdue"`
	Completed int64 `json:"completed"`
}

// MyTodoSummary 我的待办统计。
func (s *Service) MyTodoSummary(ctx context.Context, p *principal.Info) (*TodoSummary, error) {
	var out TodoSummary
	base := s.Repo.DB.WithContext(ctx).Model(&entity.Todo{}).
		Where("tenant_id = ? AND user_id = ?", p.TenantID, p.UserID)
	if err := base.Session(&gorm.Session{}).Where("status = ?", TodoStatusPending).Count(&out.Pending).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if err := base.Session(&gorm.Session{}).
		Where("status = ? AND due_time IS NOT NULL AND due_time < ?", TodoStatusPending, time.Now()).
		Count(&out.Overdue).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if err := base.Session(&gorm.Session{}).Where("status = ?", TodoStatusCompleted).Count(&out.Completed).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &out, nil
}

// WorkbenchSummary 工作中心汇总（顶栏红点 + 工作台卡片）。
type WorkbenchSummary struct {
	UnreadMessages int64 `json:"unreadMessages"`
	PendingTodos   int64 `json:"pendingTodos"`
	OverdueTodos   int64 `json:"overdueTodos"`
	UnreadNotices  int64 `json:"unreadNotices"`
}

// MyWorkbenchSummary 汇总「未读消息 / 待办 / 逾期 / 未读通知」。
func (s *Service) MyWorkbenchSummary(ctx context.Context, p *principal.Info) (*WorkbenchSummary, error) {
	out := &WorkbenchSummary{}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
		Where("tenant_id = ? AND user_id = ? AND is_read = 0", p.TenantID, p.UserID).
		Count(&out.UnreadMessages).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	todo, err := s.MyTodoSummary(ctx, p)
	if err != nil {
		return nil, err
	}
	out.PendingTodos = todo.Pending
	out.OverdueTodos = todo.Overdue

	// 未读通知：已发布公告中，当前用户还没点开过的数量
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Notice{}).
		Where("tenant_id = ? AND status = ?", p.TenantID, entity.NoticeStatusPublished).
		Where("id NOT IN (?)", s.Repo.DB.WithContext(ctx).Model(&entity.NoticeRead{}).
			Select("notice_id").Where("user_id = ?", p.UserID)).
		Count(&out.UnreadNotices).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return out, nil
}

// CreateTodo 创建待办（source 区分来源：后台手工/系统/开放接口）。
func (s *Service) CreateTodo(ctx context.Context, req *TodoReq, p *principal.Info, source int) (uint64, error) {
	target := req.UserID
	if target == 0 {
		target = p.UserID // 后台手工给自己建待办
	}
	due, err := parseDueTime(req.DueTime)
	if err != nil {
		return 0, err
	}

	// 幂等：同一租户下 biz_key 重复时直接返回已有记录，业务系统重试安全
	bizKey := strings.TrimSpace(req.BizKey)
	if bizKey != "" {
		var exists entity.Todo
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.Todo{}).
			Where("tenant_id = ? AND biz_key = ?", p.TenantID, bizKey).
			Limit(1).Find(&exists).Error; err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
		if exists.ID != 0 {
			return exists.ID, nil
		}
	}

	now := time.Now()
	todo := &entity.Todo{
		Base: entity.Base{
			ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now,
		},
		TenantID: p.TenantID,
		UserID:   target,
		Title:    strings.TrimSpace(req.Title),
		Content:  strings.TrimSpace(req.Content),
		Priority: defaultInt(req.Priority, 2),
		Status:   TodoStatusPending,
		DueTime:  due,
		LinkURL:  strings.TrimSpace(req.LinkURL),
		BizType:  strings.TrimSpace(req.BizType),
		BizID:    strings.TrimSpace(req.BizID),
		Source:   source,
	}
	if bizKey != "" {
		todo.BizKey = &bizKey
	}
	if err := s.Repo.DB.WithContext(ctx).Create(todo).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return todo.ID, nil
}

// CompleteTodo 完成/取消自己的待办（只能操作自己的数据）。
func (s *Service) UpdateMyTodoStatus(ctx context.Context, todoID uint64, status int, p *principal.Info) error {
	if status != TodoStatusCompleted && status != TodoStatusPending && status != TodoStatusCanceled {
		return errs.Invalid("待办状态不合法")
	}
	fields := map[string]any{
		"status":      status,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}
	if status == TodoStatusCompleted {
		fields["complete_time"] = time.Now()
	} else {
		fields["complete_time"] = nil
	}
	res := s.Repo.DB.WithContext(ctx).Model(&entity.Todo{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", todoID, p.TenantID, p.UserID).
		Updates(fields)
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// DeleteMyTodo 删除自己的待办。
func (s *Service) DeleteMyTodo(ctx context.Context, todoID uint64, p *principal.Info) error {
	res := s.Repo.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", todoID, p.TenantID, p.UserID).
		Delete(&entity.Todo{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ListMyNotices 我收到的通知（已发布公告 + 我的已读状态）。
type MyNoticeItem struct {
	entity.Notice
	IsRead bool `json:"isRead"`
}

// ListMyNotices 通知中心列表：只返回已发布公告。
func (s *Service) ListMyNotices(ctx context.Context, q *TodoQuery, p *principal.Info) ([]MyNoticeItem, int64, error) {
	q.normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Notice{}).Scopes(scope.Tenant(p.TenantID)).
		Where("status = ?", entity.NoticeStatusPublished)
	if q.Keyword != "" {
		tx = tx.Where("title LIKE ?", q.Keyword+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	var rows []entity.Notice
	if err := tx.Order("publish_time DESC, id DESC").Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}

	readSet := map[uint64]bool{}
	if len(rows) > 0 {
		ids := make([]uint64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		var readIDs []uint64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.NoticeRead{}).
			Where("user_id = ? AND notice_id IN ?", p.UserID, ids).
			Pluck("notice_id", &readIDs).Error; err != nil {
			return nil, 0, errs.ErrDB.WithErr(err)
		}
		for _, rid := range readIDs {
			readSet[rid] = true
		}
	}

	out := make([]MyNoticeItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, MyNoticeItem{Notice: r, IsRead: readSet[r.ID]})
	}
	return out, total, nil
}

// MarkNoticeRead 标记某条通知已读（幂等）。
func (s *Service) MarkNoticeRead(ctx context.Context, noticeID uint64, p *principal.Info) error {
	n, err := s.NoticeDetail(ctx, noticeID, p)
	if err != nil {
		return err
	}
	if n.Status != entity.NoticeStatusPublished {
		return errs.Invalid("公告未发布，无法标记已读")
	}
	rec := &entity.NoticeRead{NoticeID: noticeID, UserID: p.UserID, ReadTime: time.Now()}
	if err := s.Repo.DB.WithContext(ctx).
		Where("notice_id = ? AND user_id = ?", noticeID, p.UserID).
		FirstOrCreate(rec).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// PushMessageReq 推送站内信（后台手工发起 / 开放接口共用）。
type PushMessageReq struct {
	// UserIDs 接收人列表；为空表示全员（仅后台手工发起时允许）
	// 前端与业务系统统一用字符串传雪花 ID（JS number 只有 53 位精度），这里两种写法都接受
	UserIDs SnowflakeIDs `json:"userIds"`
	Title   string       `json:"title" binding:"required,max=200"`
	Content string       `json:"content" binding:"max=20000"`
	// Type 1 系统 2 公告 3 任务
	Type    int    `json:"type" binding:"omitempty,oneof=1 2 3"`
	BizType string `json:"bizType" binding:"max=32"`
	BizKey  string `json:"bizKey" binding:"max=128"`
}

// SnowflakeIDs 雪花 ID 列表：兼容 ["123"] 与 [123] 两种 JSON 写法。
type SnowflakeIDs []uint64

// UnmarshalJSON 同时接受字符串与数字形式的 ID。
func (s *SnowflakeIDs) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make([]uint64, 0, len(raw))
	for _, item := range raw {
		text := strings.Trim(strings.TrimSpace(string(item)), `"`)
		if text == "" || text == "null" {
			continue
		}
		v, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return errs.Invalid("接收人 ID 不合法: " + text)
		}
		out = append(out, v)
	}
	*s = out
	return nil
}

// PushMessages 给指定用户（或全员）推送站内信，返回成功条数。
//
// 幂等：提供 biz_key 时按「租户 + biz_key + 接收人」去重，业务系统重试不会重复打扰用户。
func (s *Service) PushMessages(ctx context.Context, req *PushMessageReq, p *principal.Info, source int) (int, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return 0, errs.Invalid("标题不能为空")
	}

	targets := []uint64(req.UserIDs)
	if len(targets) == 0 {
		if source == TodoSourceOpen {
			return 0, errs.Invalid("开放接口必须指定接收人 userIds")
		}
		var ids []uint64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.User{}).
			Where("tenant_id = ? AND status = 1", p.TenantID).
			Pluck("id", &ids).Error; err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
		targets = ids
	}
	if len(targets) == 0 {
		return 0, errs.Invalid("没有可推送的接收人")
	}
	if len(targets) > 2000 {
		return 0, errs.Invalid("单次推送接收人不能超过 2000 人")
	}

	bizKey := strings.TrimSpace(req.BizKey)
	now := time.Now()
	created := 0
	for _, uid := range targets {
		if bizKey != "" {
			var exists entity.Message
			if err := s.Repo.DB.WithContext(ctx).Model(&entity.Message{}).
				Where("tenant_id = ? AND user_id = ? AND biz_key = ?", p.TenantID, uid, bizKey).
				Limit(1).Find(&exists).Error; err != nil {
				return created, errs.ErrDB.WithErr(err)
			}
			if exists.ID != 0 {
				continue // 已推送过，跳过
			}
		}
		msg := &entity.Message{
			ID:         id.Next(),
			UserID:     uid,
			Title:      title,
			Content:    strings.TrimSpace(req.Content),
			Type:       defaultInt(req.Type, 1),
			BizType:    strings.TrimSpace(req.BizType),
			TenantID:   p.TenantID,
			CreateTime: now,
		}
		if bizKey != "" {
			msg.BizKey = &bizKey
		}
		if err := s.Repo.DB.WithContext(ctx).Create(msg).Error; err != nil {
			return created, errs.ErrDB.WithErr(err)
		}
		created++
	}
	return created, nil
}
