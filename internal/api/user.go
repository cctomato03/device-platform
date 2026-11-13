package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/repository"
	"device-platform/internal/service"
	"device-platform/pkg/jsonx"
	"device-platform/pkg/response"
)

// ListUsers 用户列表。
func (h *Handler) ListUsers(c *gin.Context) {
	var q repository.UserQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListUsers(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetUser 用户详情。
func (h *Handler) GetUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vo, err := h.Svc.GetUser(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// CreateUser 新增用户。
func (h *Handler) CreateUser(c *gin.Context) {
	var req service.CreateUserReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateUser(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateUser 修改用户。
func (h *Handler) UpdateUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.UpdateUserReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateUser(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteUsers 批量删除用户。
func (h *Handler) DeleteUsers(c *gin.Context) {
	var req struct {
		IDs jsonx.Uint64Slice `json:"ids" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.DeleteUsers(c.Request.Context(), req.IDs.ToUint64(), principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ChangeUserStatus 启用/停用用户。
func (h *Handler) ChangeUserStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int `json:"status" binding:"oneof=0 1 2"` // 2 兼容前端"停用"语义
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.ChangeUserStatus(c.Request.Context(), id, req.Status, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ResetUserPassword 重置密码，返回新密码。
func (h *Handler) ResetUserPassword(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&req)
	pwd, err := h.Svc.ResetPassword(c.Request.Context(), id, req.Password, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"password": pwd})
}

// AssignUserRoles 分配角色。
func (h *Handler) AssignUserRoles(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		RoleIDs jsonx.Uint64Slice `json:"roleIds"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.AssignUserRoles(c.Request.Context(), id, req.RoleIDs.ToUint64(), principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}
