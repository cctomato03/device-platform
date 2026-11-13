package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// 本文件：网络/设备运维能力接口（网卡配置、策略路由、DHCP/DNS 服务、动态路由、SNMP）。

// ==================== 网卡配置（netplan） ====================

// ListIfaceConfigs 网卡配置列表。
func (h *Handler) ListIfaceConfigs(c *gin.Context) {
	list, err := h.Svc.ListIfaceConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveIfaceConfig 新建/修改网卡配置。
func (h *Handler) SaveIfaceConfig(c *gin.Context) {
	var req service.IfaceConfigReq
	if !bindJSON(c, &req) {
		return
	}
	row, err := h.Svc.SaveIfaceConfig(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, row.ID)
}

// DeleteIfaceConfig 删除网卡配置。
func (h *Handler) DeleteIfaceConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteIfaceConfig(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// NetplanStatus 网卡配置页状态（托管文件 + 预览）。
func (h *Handler) NetplanStatus(c *gin.Context) {
	status := h.Svc.GetNetplanStatus(c.Request.Context())
	response.OkData(c, gin.H{"status": status, "bondModes": h.Svc.BondModes()})
}

// NetplanPreview 预览将要生成的 netplan 片段。
func (h *Handler) NetplanPreview(c *gin.Context) {
	preview, err := h.Svc.NetplanPreview(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"preview": preview})
}

// ApplyIfaceConfigs 下发网卡配置。
func (h *Handler) ApplyIfaceConfigs(c *gin.Context) {
	results, err := h.Svc.ApplyIfaceConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ==================== 策略路由 ====================

// ListPolicyRoutes 策略路由列表。
func (h *Handler) ListPolicyRoutes(c *gin.Context) {
	list, err := h.Svc.ListPolicyRoutes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	rules, _ := h.Svc.PolicyRuleStatus(c.Request.Context())
	response.OkData(c, gin.H{"list": list, "kernelRules": rules})
}

// SavePolicyRoute 新建/修改策略路由。
func (h *Handler) SavePolicyRoute(c *gin.Context) {
	var req service.PolicyRouteReq
	if !bindJSON(c, &req) {
		return
	}
	row, err := h.Svc.SavePolicyRoute(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, row.ID)
}

// DeletePolicyRoute 删除策略路由。
func (h *Handler) DeletePolicyRoute(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePolicyRoute(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ApplyPolicyRoutes 下发策略路由。
func (h *Handler) ApplyPolicyRoutes(c *gin.Context) {
	results, err := h.Svc.ApplyPolicyRoutes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ==================== DHCP 服务 ====================

// GetDhcpConfig 读取 DHCP 配置。
func (h *Handler) GetDhcpConfig(c *gin.Context) {
	cfg, err := h.Svc.GetDhcpConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"config": cfg, "status": h.Svc.GetDnsmasqStatus(c.Request.Context())})
}

// SaveDhcpConfig 保存 DHCP 配置。
func (h *Handler) SaveDhcpConfig(c *gin.Context) {
	var req service.DhcpConfigReq
	if !bindJSON(c, &req) {
		return
	}
	cfg, err := h.Svc.SaveDhcpConfig(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, cfg)
}

// ApplyDhcpConfig 下发 DHCP 配置。
func (h *Handler) ApplyDhcpConfig(c *gin.Context) {
	results, err := h.Svc.ApplyDhcpConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// DnsmasqStatus dnsmasq 状态（DHCP/DNS 页面共用）。
func (h *Handler) DnsmasqStatus(c *gin.Context) {
	response.OkData(c, h.Svc.GetDnsmasqStatus(c.Request.Context()))
}

// InstallDnsmasq 安装 dnsmasq。
func (h *Handler) InstallDnsmasq(c *gin.Context) {
	results, err := h.Svc.InstallDnsmasq(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ==================== DNS 服务 ====================

// GetDnsServerConfig 读取 DNS 服务配置。
func (h *Handler) GetDnsServerConfig(c *gin.Context) {
	cfg, err := h.Svc.GetDnsServerConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"config": cfg, "status": h.Svc.GetDnsmasqStatus(c.Request.Context())})
}

// SaveDnsServerConfig 保存 DNS 服务配置。
func (h *Handler) SaveDnsServerConfig(c *gin.Context) {
	var req service.DnsServerReq
	if !bindJSON(c, &req) {
		return
	}
	cfg, err := h.Svc.SaveDnsServerConfig(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, cfg)
}

// ApplyDnsServerConfig 下发 DNS 服务配置。
func (h *Handler) ApplyDnsServerConfig(c *gin.Context) {
	results, err := h.Svc.ApplyDnsServerConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// ==================== 动态路由 BGP / OSPF ====================

// ListRoutingConfigs 动态路由配置。
func (h *Handler) ListRoutingConfigs(c *gin.Context) {
	list, err := h.Svc.ListRoutingConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"list": list, "status": h.Svc.GetRoutingStatus(c.Request.Context())})
}

// SaveRoutingConfig 保存动态路由配置。
func (h *Handler) SaveRoutingConfig(c *gin.Context) {
	var req service.RoutingConfigReq
	if !bindJSON(c, &req) {
		return
	}
	row, err := h.Svc.SaveRoutingConfig(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, row.ID)
}

// ApplyRoutingConfigs 下发动态路由配置。
func (h *Handler) ApplyRoutingConfigs(c *gin.Context) {
	results, err := h.Svc.ApplyRoutingConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// InstallFRR 安装 FRR。
func (h *Handler) InstallFRR(c *gin.Context) {
	results, err := h.Svc.InstallFRR(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// RoutingStatus 动态路由运行状态。
func (h *Handler) RoutingStatus(c *gin.Context) {
	response.OkData(c, h.Svc.GetRoutingStatus(c.Request.Context()))
}

// ==================== SNMP ====================

// GetSnmpConfig 读取 SNMP 配置。
func (h *Handler) GetSnmpConfig(c *gin.Context) {
	cfg, err := h.Svc.GetSnmpConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"config": cfg, "status": h.Svc.GetSnmpStatus(c.Request.Context())})
}

// SaveSnmpConfig 保存 SNMP 配置。
func (h *Handler) SaveSnmpConfig(c *gin.Context) {
	var req service.SnmpConfigReq
	if !bindJSON(c, &req) {
		return
	}
	cfg, err := h.Svc.SaveSnmpConfig(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, cfg)
}

// ApplySnmpConfig 下发 SNMP 配置。
func (h *Handler) ApplySnmpConfig(c *gin.Context) {
	results, err := h.Svc.ApplySnmpConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// InstallSNMP 安装 net-snmp。
func (h *Handler) InstallSNMP(c *gin.Context) {
	results, err := h.Svc.InstallSNMP(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"results": results})
}

// SnmpStatus SNMP 运行状态。
func (h *Handler) SnmpStatus(c *gin.Context) {
	response.OkData(c, h.Svc.GetSnmpStatus(c.Request.Context()))
}

// ==================== 插件市场（在线源） ====================

// ListMarketSources 插件源列表。
func (h *Handler) ListMarketSources(c *gin.Context) {
	list, err := h.Svc.ListPluginSources(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SaveMarketSource 新建/修改插件源。
func (h *Handler) SaveMarketSource(c *gin.Context) {
	var req service.PluginSourceReq
	if !bindJSON(c, &req) {
		return
	}
	row, err := h.Svc.SavePluginSource(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, row.ID)
}

// DeleteMarketSource 删除插件源。
func (h *Handler) DeleteMarketSource(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePluginSource(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListMarketPlugins 在线市场条目（?refresh=true 强制刷新缓存）。
func (h *Handler) ListMarketPlugins(c *gin.Context) {
	force := c.Query("refresh") == "true" || c.Query("refresh") == "1"
	list, err := h.Svc.ListMarket(c.Request.Context(), force)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// InstallMarketPlugin 从市场安装/升级插件。
func (h *Handler) InstallMarketPlugin(c *gin.Context) {
	var req struct {
		SourceID uint64 `json:"sourceId,string"`
		PluginID string `json:"pluginId"`
	}
	if !bindJSON(c, &req) {
		return
	}
	view, err := h.Svc.InstallFromMarket(c.Request.Context(), req.SourceID, req.PluginID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, view)
}
