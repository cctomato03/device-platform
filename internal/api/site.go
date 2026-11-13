package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// GetSiteConfig 读取站点配置（公开接口与后台编辑页共用：登录页需要网站名称/logo/背景/注册开关）。
func (h *Handler) GetSiteConfig(c *gin.Context) {
	vo, err := h.Svc.SiteConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// UpdateSiteConfig 保存站点配置。
func (h *Handler) UpdateSiteConfig(c *gin.Context) {
	var req service.SiteConfigReq
	if !bindJSON(c, &req) {
		return
	}
	vo, err := h.Svc.UpdateSiteConfig(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}
