package api

import (
	"github.com/gin-gonic/gin"

	"device-platform/internal/service"
	"device-platform/pkg/response"
)

// ListJobs 定时任务列表。
func (h *Handler) ListJobs(c *gin.Context) {
	list, err := h.Svc.ListJobs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreateJob 新增定时任务。
func (h *Handler) CreateJob(c *gin.Context) {
	var p service.JobPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	job, err := h.Svc.CreateJob(c.Request.Context(), &p)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.reloadJobs(c)
	response.OkData(c, job)
}

// UpdateJob 编辑定时任务。
func (h *Handler) UpdateJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var p service.JobPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	if err := h.Svc.UpdateJob(c.Request.Context(), id, &p); err != nil {
		response.Fail(c, err)
		return
	}
	h.reloadJobs(c)
	response.Ok(c)
}

// SetJobStatus 启停定时任务。
func (h *Handler) SetJobStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var body struct {
		Status int `json:"status" binding:"oneof=1 2"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	if err := h.Svc.SetJobStatus(c.Request.Context(), id, body.Status); err != nil {
		response.Fail(c, err)
		return
	}
	h.reloadJobs(c)
	response.Ok(c)
}

// RunJob 立即执行一次定时任务。
func (h *Handler) RunJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.RunJob(c.Request.Context(), id, "manual"); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteJob 删除定时任务。
func (h *Handler) DeleteJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteJob(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	h.reloadJobs(c)
	response.Ok(c)
}

// ListJobLogs 任务执行日志。
func (h *Handler) ListJobLogs(c *gin.Context) {
	var q service.JobQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListJobLogs(c.Request.Context(), &q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	response.Page(c, list, total, page, pageSize)
}

// reloadJobs 任务变更后热重载调度器（未启用调度器时静默跳过）。
func (h *Handler) reloadJobs(c *gin.Context) {
	if h.ReloadJobs == nil {
		return
	}
	h.ReloadJobs(c.Request.Context())
}

// JobMeta 任务类型与内置处理器清单（前端据此动态渲染表单，避免把类型/处理器写死）。
//
//	@Summary 定时任务元信息
//	@Tags 定时任务
//	@Success 200 {object} response.Body{data=service.JobMetaResp}
//	@Router /system/jobs/meta [get]
func (h *Handler) JobMeta(c *gin.Context) {
	response.OkData(c, h.Svc.JobMeta())
}
