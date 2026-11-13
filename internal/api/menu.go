package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// MenuTree 全部菜单树（菜单管理）。
func (h *Handler) MenuTree(c *gin.Context) {
	tree, err := h.Svc.MenuTree(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// CreateMenu 新增菜单。
func (h *Handler) CreateMenu(c *gin.Context) {
	var req service.MenuReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateMenu(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateMenu 修改菜单。
func (h *Handler) UpdateMenu(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.MenuReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateMenu(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteMenu 删除菜单。
func (h *Handler) DeleteMenu(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteMenu(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeptTree 部门树。
func (h *Handler) DeptTree(c *gin.Context) {
	tree, err := h.Svc.DeptTree(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// CreateDept 新增部门。
// DeptDetail 部门详情。
func (h *Handler) DeptDetail(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	d, err := h.Svc.DeptDetail(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, d)
}

func (h *Handler) CreateDept(c *gin.Context) {
	var req service.DeptReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateDept(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateDept 修改部门。
func (h *Handler) UpdateDept(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.DeptReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateDept(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteDept 删除部门。
func (h *Handler) DeleteDept(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteDept(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}
