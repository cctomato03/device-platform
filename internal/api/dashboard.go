package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/pkg/response"
)

// DashboardSummary 首页概览数据。
func (h *Handler) DashboardSummary(c *gin.Context) {
	p := principal.Must(c)
	tenantID := uint64(0)
	if p != nil {
		tenantID = p.TenantID
	}
	data, err := h.Svc.Dashboard(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}
