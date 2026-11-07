package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/response"
)

// ListMyMessages 我的站内信分页。
func (h *Handler) ListMyMessages(c *gin.Context) {
	var q service.MessageQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMyMessages(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// UnreadMessageCount 未读数量。
func (h *Handler) UnreadMessageCount(c *gin.Context) {
	n, err := h.Svc.UnreadMessageCount(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"count": n})
}

// MarkMessageRead 标记已读。
func (h *Handler) MarkMessageRead(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.MarkMessageRead(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// MarkAllMessagesRead 全部已读。
func (h *Handler) MarkAllMessagesRead(c *gin.Context) {
	n, err := h.Svc.MarkAllMessagesRead(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"updated": n})
}

// DeleteMyMessage 删除我的站内信。
func (h *Handler) DeleteMyMessage(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteMyMessage(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}
