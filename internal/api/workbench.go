package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// ============================ 工作中心（我的待办 / 消息 / 通知） ============================

// MyWorkbenchSummary 工作中心汇总：未读消息 / 待办 / 逾期 / 未读通知。
//
//	@Summary 工作中心汇总
//	@Tags 工作中心
//	@Success 200 {object} response.Body{data=service.WorkbenchSummary}
//	@Router /my/summary [get]
func (h *Handler) MyWorkbenchSummary(c *gin.Context) {
	out, err := h.Svc.MyWorkbenchSummary(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, out)
}

// ListMyTodos 我的待办列表。
//
//	@Summary 我的待办
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/todos [get]
func (h *Handler) ListMyTodos(c *gin.Context) {
	var q service.TodoQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMyTodos(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// MyTodoSummary 我的待办统计。
//
//	@Summary 待办统计
//	@Tags 工作中心
//	@Success 200 {object} response.Body{data=service.TodoSummary}
//	@Router /my/todos/summary [get]
func (h *Handler) MyTodoSummary(c *gin.Context) {
	out, err := h.Svc.MyTodoSummary(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, out)
}

// CreateMyTodo 给自己建一条待办（用户自助）。
//
//	@Summary 新建待办
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/todos [post]
func (h *Handler) CreateMyTodo(c *gin.Context) {
	var req service.TodoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	p := principal.Must(c)
	req.UserID = p.UserID // 自助创建只能给自己
	id, err := h.Svc.CreateTodo(c.Request.Context(), &req, p, service.TodoSourceManual)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateMyTodoStatus 完成 / 取消 / 恢复我的待办。
//
//	@Summary 更新待办状态
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/todos/{id}/status [put]
func (h *Handler) UpdateMyTodoStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var body struct {
		Status int `json:"status" binding:"required,oneof=1 2 3"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	if err := h.Svc.UpdateMyTodoStatus(c.Request.Context(), id, body.Status, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteMyTodo 删除我的待办。
//
//	@Summary 删除待办
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/todos/{id} [delete]
func (h *Handler) DeleteMyTodo(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteMyTodo(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListMyNotices 我收到的通知（已发布公告 + 已读状态）。
//
//	@Summary 我的通知
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/notices [get]
func (h *Handler) ListMyNotices(c *gin.Context) {
	var q service.TodoQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMyNotices(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// MarkNoticeRead 标记通知已读。
//
//	@Summary 通知已读
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /my/notices/{id}/read [post]
func (h *Handler) MarkNoticeRead(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.MarkNoticeRead(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 推送（后台手工发起 / 业务系统对接） ============================

// PushMessage 给指定用户或全员推送站内信。
//
//	@Summary 推送站内信
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /system/push/messages [post]
func (h *Handler) PushMessage(c *gin.Context) {
	var req service.PushMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	n, err := h.Svc.PushMessages(c.Request.Context(), &req, principal.Must(c), service.TodoSourceSystem)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"created": n})
}

// PushTodo 给指定用户派发待办。
//
//	@Summary 派发待办
//	@Tags 工作中心
//	@Success 200 {object} response.Body
//	@Router /system/push/todos [post]
func (h *Handler) PushTodo(c *gin.Context) {
	var req service.TodoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	p := principal.Must(c)
	if req.UserID == 0 {
		req.UserID = p.UserID
	}
	id, err := h.Svc.CreateTodo(c.Request.Context(), &req, p, service.TodoSourceSystem)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// ============================ 开放接口（业务系统对接） ============================

// OpenPushMessage 业务系统推送站内信（需 X-API-KEY）。
//
//	@Summary 开放接口-推送站内信
//	@Tags 开放接口
//	@Success 200 {object} response.Body
//	@Router /open/messages [post]
func (h *Handler) OpenPushMessage(c *gin.Context) {
	var req service.PushMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	p := principal.Must(c)
	n, err := h.Svc.PushMessages(c.Request.Context(), &req, p, service.TodoSourceOpen)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"created": n})
}

// OpenPushTodo 业务系统派发待办（需 X-API-KEY）。
//
//	@Summary 开放接口-派发待办
//	@Tags 开放接口
//	@Success 200 {object} response.Body
//	@Router /open/todos [post]
func (h *Handler) OpenPushTodo(c *gin.Context) {
	var req service.TodoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	p := principal.Must(c)
	if req.UserID == 0 {
		response.Fail(c, errs.Invalid("开放接口必须指定接收人 userId"))
		return
	}
	id, err := h.Svc.CreateTodo(c.Request.Context(), &req, p, service.TodoSourceOpen)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"id": id})
}
