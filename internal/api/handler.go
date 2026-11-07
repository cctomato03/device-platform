// Package api HTTP 处理层：参数绑定、调用 service、输出统一响应。
package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// Handler 聚合全部 HTTP 处理器。
type Handler struct {
	Svc *service.Service

	// ReloadJobs 任务变更后热重载调度器，由 bootstrap 注入（可为 nil）。
	ReloadJobs func(ctx context.Context) error
}

func New(svc *service.Service) *Handler { return &Handler{Svc: svc} }

// bindJSON 解析 JSON 请求体，失败直接返回参数错误。
func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		response.Fail(c, errs.InvalidErr(err))
		return false
	}
	return true
}

// pathID 解析路径参数 :id。
func pathID(c *gin.Context) (uint64, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		response.Fail(c, errs.InvalidErr(err))
		return 0, false
	}
	return v, true
}

// queryID 解析查询参数。
func queryID(c *gin.Context, key string) uint64 {
	v, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return v
}
