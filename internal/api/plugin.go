package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"device-platform/pkg/errs"
	"device-platform/pkg/response"
)

// ListPlugins 插件列表。
func (h *Handler) ListPlugins(c *gin.Context) {
	list, err := h.Svc.ListPlugins(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// GetPlugin 插件详情。
func (h *Handler) GetPlugin(c *gin.Context) {
	key := c.Param("key")
	view, err := h.Svc.GetPlugin(c.Request.Context(), key)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, view)
}

// InstallPlugin 上传插件包并安装。
func (h *Handler) InstallPlugin(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.Invalid("请选择插件包文件（.zip / .tar.gz）"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errs.InvalidErr(err))
		return
	}
	defer f.Close()
	view, err := h.Svc.InstallPlugin(c.Request.Context(), fh.Filename, f)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, view)
}

// PluginAction 插件生命周期动作（start/stop/restart/enable/disable/autostart）。
func (h *Handler) PluginAction(c *gin.Context) {
	key := c.Param("key")
	action := c.Param("action")
	value := c.Query("value") == "true" || c.Query("value") == "1"
	if err := h.Svc.PluginAction(c.Request.Context(), key, action, value); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// UninstallPlugin 卸载插件（?purge=true 连数据一起删）。
func (h *Handler) UninstallPlugin(c *gin.Context) {
	key := c.Param("key")
	purge := c.Query("purge") == "true" || c.Query("purge") == "1"
	if err := h.Svc.UninstallPlugin(c.Request.Context(), key, purge); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// UpdatePluginConfig 保存插件配置。
func (h *Handler) UpdatePluginConfig(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Config  map[string]any `json:"config"`
		Restart bool           `json:"restart"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdatePluginConfig(c.Request.Context(), key, req.Config, req.Restart); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// PluginLogs 插件日志。
func (h *Handler) PluginLogs(c *gin.Context) {
	key := c.Param("key")
	lines, _ := strconv.Atoi(c.Query("lines"))
	text, err := h.Svc.PluginLogs(c.Request.Context(), key, lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"text": text})
}

// PluginStatus 插件运行态。
func (h *Handler) PluginStatus(c *gin.Context) {
	key := c.Param("key")
	st, err := h.Svc.PluginStatus(c.Request.Context(), key)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, st)
}

// PluginRuntimeMeta 插件运行时能力（docker 是否可用等）。
func (h *Handler) PluginRuntimeMeta(c *gin.Context) {
	response.OkData(c, h.Svc.GetPluginRuntimeMeta(c.Request.Context()))
}

// ListBundledPlugins 平台自带插件（市场里「可安装」的部分）。
func (h *Handler) ListBundledPlugins(c *gin.Context) {
	list, err := h.Svc.ListBundledPlugins(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// InstallBundledPlugin 安装平台自带插件。
func (h *Handler) InstallBundledPlugin(c *gin.Context) {
	key := c.Param("key")
	view, err := h.Svc.InstallBundledPlugin(c.Request.Context(), key)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, view)
}
