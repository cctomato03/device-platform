package api

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// excelContentType xlsx MIME。
const excelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// writeExcel 以附件形式返回表格（中文文件名用 RFC 5987 编码，避免各浏览器乱码）。
func writeExcel(c *gin.Context, data []byte, filename string) {
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	c.Header("Content-Type", excelContentType)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, excelContentType, data)
}

// ExportUsers 导出用户。
func (h *Handler) ExportUsers(c *gin.Context) {
	var q repository.UserQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	data, name, err := h.Svc.ExportUsers(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, name)
}

// ImportUsers 导入用户（multipart：file 字段）。
func (h *Handler) ImportUsers(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.InvalidErr(err))
		return
	}
	if file.Size > 5<<20 {
		response.Fail(c, errs.InvalidErr(errors.New("文件不能超过 5MB")))
		return
	}
	f, err := file.Open()
	if err != nil {
		response.Fail(c, errs.InvalidErr(err))
		return
	}
	defer func() { _ = f.Close() }()
	res, err := h.Svc.ImportUsers(c.Request.Context(), f, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// UserImportTemplate 下载用户导入模板。
func (h *Handler) UserImportTemplate(c *gin.Context) {
	data, err := h.Svc.UserImportTemplate()
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, "用户导入模板.xlsx")
}

// ExportOperLogs 导出操作日志。
func (h *Handler) ExportOperLogs(c *gin.Context) {
	var q service.LogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	data, name, err := h.Svc.ExportOperLogs(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, name)
}
