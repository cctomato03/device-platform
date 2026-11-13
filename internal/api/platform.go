package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"device-platform/internal/service"
	"device-platform/pkg/errs"
	"device-platform/pkg/response"
)

// ---------------- 主机管理 ----------------

// HostOverview 设备概览。
func (h *Handler) HostOverview(c *gin.Context) {
	data, err := h.Svc.HostOverview(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// HostSnapshot 硬件快照。
func (h *Handler) HostSnapshot(c *gin.Context) {
	data, err := h.Svc.HostSnapshot(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// HostMetricsHistory 指标历史。
func (h *Handler) HostMetricsHistory(c *gin.Context) {
	minutes, _ := strconv.Atoi(c.Query("minutes"))
	list, err := h.Svc.MetricHistory(c.Request.Context(), minutes)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// ---------------- 告警 ----------------

// ListAlertRules 告警规则列表。
func (h *Handler) ListAlertRules(c *gin.Context) {
	list, err := h.Svc.ListAlertRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveAlertRule 新建或修改告警规则。
func (h *Handler) SaveAlertRule(c *gin.Context) {
	var req service.AlertRuleReq
	if !bindJSON(c, &req) {
		return
	}
	if req.ID == 0 {
		row, err := h.Svc.CreateAlertRule(c.Request.Context(), &req)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OkID(c, row.ID)
		return
	}
	if err := h.Svc.UpdateAlertRule(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteAlertRule 删除告警规则。
func (h *Handler) DeleteAlertRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteAlertRule(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListAlertRecords 告警记录。
func (h *Handler) ListAlertRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	status, _ := strconv.Atoi(c.Query("status"))
	list, total, err := h.Svc.ListAlertRecords(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	response.Page(c, list, total, page, pageSize)
}

// AckAlertRecord 确认告警。
func (h *Handler) AckAlertRecord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.AckAlertRecord(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ---------------- 日志 ----------------

// ListAccessLogs 访问日志。
func (h *Handler) ListAccessLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	status, _ := strconv.Atoi(c.Query("status"))
	list, total, err := h.Svc.ListAccessLogs(c.Request.Context(), c.Query("ip"), c.Query("path"), status, page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	response.Page(c, list, total, page, pageSize)
}

// ListTrafficLogs 流量日志。
func (h *Handler) ListTrafficLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	list, total, err := h.Svc.ListTrafficLogs(c.Request.Context(), c.Query("iface"), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	response.Page(c, list, total, page, pageSize)
}

// SystemLog 系统日志（平台日志文件尾部）。
func (h *Handler) SystemLog(c *gin.Context) {
	lines, _ := strconv.Atoi(c.Query("lines"))
	data, err := h.Svc.SystemLog(c.Request.Context(), lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// ---------------- 设备管理 ----------------

// GetDeviceSettings 读取本地设置。
func (h *Handler) GetDeviceSettings(c *gin.Context) {
	data, err := h.Svc.GetDeviceSettings(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// UpdateDeviceSettings 保存本地设置。
func (h *Handler) UpdateDeviceSettings(c *gin.Context) {
	var req service.DeviceSettings
	if !bindJSON(c, &req) {
		return
	}
	results, err := h.Svc.UpdateDeviceSettings(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// GetDeviceTime 时间与 NTP 状态。
func (h *Handler) GetDeviceTime(c *gin.Context) {
	data, err := h.Svc.GetDeviceTime(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// SyncDeviceTime 立即校时。
func (h *Handler) SyncDeviceTime(c *gin.Context) {
	results, err := h.Svc.SyncTimeNow(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// PlatformAbout 关于系统（版本信息）。
func (h *Handler) PlatformAbout(c *gin.Context) {
	data, err := h.Svc.HostOverview(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{
		"platform":  data.Platform,
		"host":      data.Host,
		"cpu":       data.CPU,
		"memory":    data.Memory,
		"device":    data.Device,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ---------------- 排查工具 ----------------

// ToolMeta 工具可用性。
func (h *Handler) ToolMeta(c *gin.Context) {
	response.OkData(c, h.Svc.ToolMeta())
}

// RunTool 执行排查命令。
func (h *Handler) RunTool(c *gin.Context) {
	var req service.ToolReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.RunTool(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// StreamTool 流式执行（SSE）。
//
// 前端用 EventSource 订阅 /tools/stream?action=ping&target=...&count=4。
func (h *Handler) StreamTool(c *gin.Context) {
	req := service.ToolReq{
		Action: c.Query("action"),
		Target: c.Query("target"),
		Iface:  c.Query("iface"),
	}
	req.Count, _ = strconv.Atoi(c.Query("count"))
	req.Port, _ = strconv.Atoi(c.Query("port"))

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := c.Writer.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := h.Svc.StreamTool(c.Request.Context(), &req, c.Writer, flush); err != nil {
		payload := errs.Normalize(err)
		if ae, ok := payload.(*errs.AppError); ok {
			c.SSEvent("error", gin.H{"message": ae.Msg})
		} else {
			c.SSEvent("error", gin.H{"message": "执行失败"})
		}
		flush()
	}
}

// ---------------- 网络管理 ----------------

// NetworkStatus 网络状态。
func (h *Handler) NetworkStatus(c *gin.Context) {
	data, err := h.Svc.GetNetworkStatus(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// ListInterfaces 网卡列表。
func (h *Handler) ListInterfaces(c *gin.Context) {
	list, err := h.Svc.ListInterfaces(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// ListStaticRoutes 静态路由列表。
func (h *Handler) ListStaticRoutes(c *gin.Context) {
	list, err := h.Svc.ListStaticRoutes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveStaticRoute 新建/修改静态路由。
func (h *Handler) SaveStaticRoute(c *gin.Context) {
	var req service.StaticRouteReq
	if !bindJSON(c, &req) {
		return
	}
	row, err := h.Svc.SaveStaticRoute(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, row.ID)
}

// DeleteStaticRoute 删除静态路由。
func (h *Handler) DeleteStaticRoute(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteStaticRoute(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ApplyStaticRoutes 下发静态路由。
func (h *Handler) ApplyStaticRoutes(c *gin.Context) {
	results, err := h.Svc.ApplyStaticRoutes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// GetDNSConfig DNS 配置。
func (h *Handler) GetDNSConfig(c *gin.Context) {
	response.OkData(c, h.Svc.GetDNSConfig(c.Request.Context()))
}

// UpdateDNSConfig 修改 DNS。
func (h *Handler) UpdateDNSConfig(c *gin.Context) {
	var req struct {
		Servers []string `json:"servers"`
	}
	if !bindJSON(c, &req) {
		return
	}
	results, err := h.Svc.UpdateDNSConfig(c.Request.Context(), req.Servers)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ---------------- 安全策略 ----------------

// FirewallStatus 防火墙状态。
func (h *Handler) FirewallStatus(c *gin.Context) {
	data, err := h.Svc.GetFirewallStatus(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, data)
}

// InstallFirewalld 安装/启用 firewalld。
func (h *Handler) InstallFirewalld(c *gin.Context) {
	results, err := h.Svc.InstallFirewalld(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ListPolicies 安全策略列表。
func (h *Handler) ListPolicies(c *gin.Context) {
	list, err := h.Svc.ListPolicies(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SavePolicy 新建/修改安全策略。
func (h *Handler) SavePolicy(c *gin.Context) {
	var req service.PolicyReq
	if !bindJSON(c, &req) {
		return
	}
	row, warnings, err := h.Svc.SavePolicy(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"id": strconv.FormatUint(row.ID, 10), "warnings": warnings})
}

// DeletePolicy 删除安全策略。
func (h *Handler) DeletePolicy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePolicy(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ApplyPolicies 下发安全策略。
func (h *Handler) ApplyPolicies(c *gin.Context) {
	results, err := h.Svc.ApplyPolicies(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ListNATRules NAT 列表。
func (h *Handler) ListNATRules(c *gin.Context) {
	list, err := h.Svc.ListNATRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveNATRule 新建/修改 NAT。
func (h *Handler) SaveNATRule(c *gin.Context) {
	var req service.NatReq
	if !bindJSON(c, &req) {
		return
	}
	row, warnings, err := h.Svc.SaveNATRule(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"id": strconv.FormatUint(row.ID, 10), "warnings": warnings})
}

// DeleteNATRule 删除 NAT。
func (h *Handler) DeleteNATRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteNATRule(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ApplyNATRules 下发 NAT。
func (h *Handler) ApplyNATRules(c *gin.Context) {
	results, err := h.Svc.ApplyNATRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ListQosRules QoS 列表。
func (h *Handler) ListQosRules(c *gin.Context) {
	list, err := h.Svc.ListQosRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveQosRule 新建/修改 QoS。
func (h *Handler) SaveQosRule(c *gin.Context) {
	var req service.QosReq
	if !bindJSON(c, &req) {
		return
	}
	row, warnings, err := h.Svc.SaveQosRule(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"id": strconv.FormatUint(row.ID, 10), "warnings": warnings})
}

// DeleteQosRule 删除 QoS。
func (h *Handler) DeleteQosRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteQosRule(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ApplyQosRules 下发 QoS。
func (h *Handler) ApplyQosRules(c *gin.Context) {
	results, err := h.Svc.ApplyQosRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}
