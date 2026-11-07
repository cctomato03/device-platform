package api

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"web-basic/pkg/response"
)

// GenTables 代码生成器：库表清单。
func (h *Handler) GenTables(c *gin.Context) {
	list, err := h.Svc.GenTables(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// GenTableDetail 代码生成器：表字段详情。
func (h *Handler) GenTableDetail(c *gin.Context) {
	table := c.Param("table")
	meta, err := h.Svc.GenTableDetail(c.Request.Context(), table)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, meta)
}

// GenPreview 代码生成器：预览生成代码。
func (h *Handler) GenPreview(c *gin.Context) {
	table := c.Param("table")
	files, err := h.Svc.GenPreview(c.Request.Context(), table)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, files)
}

// GenDownload 代码生成器：打包下载。
func (h *Handler) GenDownload(c *gin.Context) {
	var req struct {
		Tables []string `json:"tables" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	data, name, err := h.Svc.GenDownload(c.Request.Context(), req.Tables)
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	c.Data(http.StatusOK, "application/zip", data)
}
