package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// ListRoles 角色列表。
func (h *Handler) ListRoles(c *gin.Context) {
	var q service.RoleQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListRoles(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// AllRoles 全部启用角色（下拉）。
func (h *Handler) AllRoles(c *gin.Context) {
	list, err := h.Svc.AllRoles(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreateRole 新增角色。
func (h *Handler) CreateRole(c *gin.Context) {
	var req service.RoleReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateRole(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateRole 修改角色。
func (h *Handler) UpdateRole(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.RoleReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateRole(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteRole 删除角色。
func (h *Handler) DeleteRole(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteRole(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// GrantRole 角色授权。
func (h *Handler) GrantRole(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.GrantReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.GrantRole(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// RoleGranted 角色已授权信息。
func (h *Handler) RoleGranted(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	data, err := h.Svc.RoleGrantedInfo(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}
