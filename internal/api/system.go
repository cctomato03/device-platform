package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/errs"
	"device-platform/pkg/response"
)

// ---------------- 岗位 ----------------

func (h *Handler) ListPosts(c *gin.Context) {
	list, err := h.Svc.ListPosts(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

func (h *Handler) CreatePost(c *gin.Context) {
	var req service.PostReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreatePost(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

func (h *Handler) UpdatePost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.PostReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdatePost(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) DeletePost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePost(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ---------------- 字典 ----------------

func (h *Handler) ListDictTypes(c *gin.Context) {
	list, err := h.Svc.ListDictTypes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

func (h *Handler) CreateDictType(c *gin.Context) {
	var req service.DictTypeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateDictType(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

func (h *Handler) UpdateDictType(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.DictTypeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateDictType(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) DeleteDictType(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteDictType(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DictData 按编码取字典项（前端字典组件使用）。
func (h *Handler) DictData(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, errs.ErrBadRequest)
		return
	}
	list, err := h.Svc.DictData(c.Request.Context(), code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

func (h *Handler) CreateDictData(c *gin.Context) {
	var req service.DictDataReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateDictData(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

func (h *Handler) UpdateDictData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.DictDataReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateDictData(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) DeleteDictData(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteDictData(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ---------------- 参数 ----------------

func (h *Handler) ListConfigs(c *gin.Context) {
	list, err := h.Svc.ListConfigs(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

func (h *Handler) CreateConfig(c *gin.Context) {
	var req service.ConfigReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateConfig(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.ConfigReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateConfig(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) DeleteConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteConfig(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ---------------- 租户 ----------------

func (h *Handler) ListTenants(c *gin.Context) {
	list, err := h.Svc.ListTenants(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

func (h *Handler) CreateTenant(c *gin.Context) {
	var req service.TenantReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateTenant(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

func (h *Handler) UpdateTenant(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.TenantReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateTenant(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) DeleteTenant(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteTenant(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ---------------- 文件 ----------------

// UploadFile 单文件上传。
func (h *Handler) UploadFile(c *gin.Context) {
	// 请求体硬上限：multipart 会先落临时文件，必须在校验前就限制大小，
	// 否则超大上传可以先写满磁盘再被 upload.max_size_mb 拒绝。
	if maxBytes := int64(h.Svc.Cfg.Upload.MaxSizeMB)*1024*1024 + 1024*1024; maxBytes > 1024*1024 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	}
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.InvalidErr(err))
		return
	}
	bizType := c.PostForm("bizType")
	if bizType == "" {
		bizType = c.Query("bizType")
	}
	rec, err := h.Svc.UploadFile(c.Request.Context(), fh, bizType, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, rec)
}

func (h *Handler) ListFiles(c *gin.Context) {
	var q service.FileQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListFiles(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

func (h *Handler) DeleteFile(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteFile(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// FileURL 获取文件访问地址（私有文件返回签名 URL）。
func (h *Handler) FileURL(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	url, err := h.Svc.FileURL(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"url": url})
}

// ---------------- 日志与监控 ----------------

func (h *Handler) ListLoginLogs(c *gin.Context) {
	var q service.LogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListLoginLogs(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

func (h *Handler) ListOperLogs(c *gin.Context) {
	var q service.LogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListOperLogs(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

func (h *Handler) OnlineUsers(c *gin.Context) {
	list, err := h.Svc.OnlineUsers(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// KickUser 强制下线。
func (h *Handler) KickUser(c *gin.Context) {
	uid := queryID(c, "userId")
	jti := c.Query("jti")
	if uid == 0 || jti == "" {
		response.Fail(c, errs.ErrBadRequest)
		return
	}
	if err := h.Svc.KickUser(c.Request.Context(), uid, jti); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

func (h *Handler) ServerInfo(c *gin.Context) {
	response.OkData(c, h.Svc.ServerInfo(c.Request.Context()))
}

// OSSUploadTicket 申请前端直传签名（浏览器直接把文件推到 OSS，不占用应用带宽）。
//
//	@Summary 申请直传签名
//	@Tags 文件
//	@Success 200 {object} response.Body{data=service.UploadTicketVO}
//	@Router /files/oss-ticket [post]
func (h *Handler) OSSUploadTicket(c *gin.Context) {
	var req service.UploadTicketReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	vo, err := h.Svc.CreateUploadTicket(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// OSSUploadRecord 直传完成回执（落 sys_file）。
//
//	@Summary 直传完成回执
//	@Tags 文件
//	@Success 200 {object} response.Body{data=entity.File}
//	@Router /files/oss-record [post]
func (h *Handler) OSSUploadRecord(c *gin.Context) {
	var req service.OSSRecordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	rec, err := h.Svc.RecordDirectUpload(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, rec)
}
