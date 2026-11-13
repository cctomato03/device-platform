package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// ListNotices 公告分页列表。
func (h *Handler) ListNotices(c *gin.Context) {
	var q service.NoticeQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListNotices(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetNotice 公告详情。
func (h *Handler) GetNotice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	n, err := h.Svc.NoticeDetail(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, n)
}

// CreateNotice 新增公告。
func (h *Handler) CreateNotice(c *gin.Context) {
	var req service.NoticeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateNotice(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateNotice 修改公告。
func (h *Handler) UpdateNotice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.NoticeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateNotice(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteNotice 删除公告。
func (h *Handler) DeleteNotice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteNotice(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// PublishNotice 发布公告（扇出站内信）。
func (h *Handler) PublishNotice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	sent, err := h.Svc.PublishNotice(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"pushed": sent})
}
