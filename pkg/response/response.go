// Package response 统一 HTTP 响应封装。
package response

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"device-platform/internal/metrics"
	"device-platform/pkg/errs"
)

// Body 标准响应体：code=0 表示成功。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	TraceID string `json:"traceId"`
}

// PageData 分页数据结构。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func traceID(c *gin.Context) string {
	if v, ok := c.Get("traceId"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Ok 返回成功（无数据）。
func Ok(c *gin.Context) { OkData(c, nil) }

// OkData 返回成功（带数据）。
func OkData(c *gin.Context, data any) {
	metrics.Default.IncCode("0")
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data, TraceID: traceID(c)})
}

// OkID 返回新增记录的雪花 ID。
// 注意：ID 必须以字符串输出，否则 JS Number 超过 2^53 会精度丢失（新增后按 ID 编辑会找不到数据）。
func OkID(c *gin.Context, id uint64) {
	OkData(c, gin.H{"id": strconv.FormatUint(id, 10)})
}

// Page 返回分页成功。
func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OkData(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

// FailCode 返回指定错误码。
func FailCode(c *gin.Context, code int, msg string) {
	metrics.Default.IncCode(strconv.Itoa(code))
	c.JSON(httpStatus(code), Body{Code: code, Message: msg, TraceID: traceID(c)})
}

// Fail 返回业务错误；非 AppError 一律按系统内部错误处理，避免泄露内部细节。
func Fail(c *gin.Context, err error) {
	// 统一归一化：唯一键冲突 → 10005/409，记录不存在 → 10004/404
	err = errs.Normalize(err)
	// 写入 gin 错误链：操作日志中间件据此把本次操作记为"失败"并落错误信息
	_ = c.Error(err)
	var ae *errs.AppError
	if errors.As(err, &ae) {
		FailCode(c, ae.Code, ae.Msg)
		return
	}
	FailCode(c, errs.ErrInternal.Code, errs.ErrInternal.Msg)
}

// BindErr 参数绑定错误 → 统一业务错误。
func BindErr(err error) error { return errs.Invalid(bindMessage(err)) }

// bindMessage 把参数绑定/校验错误转成「用户看得懂」的提示。
//
// 默认框架错误（"Key: 'UserReq.Password' Error:Field validation for 'Password' failed on the 'min' tag"）
// 既难看又暴露结构体字段名，这里统一翻译成中文提示。
func bindMessage(err error) string {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		fe := verrs[0]
		field := fieldLabels[fe.Field()]
		if field == "" {
			field = fe.Field()
		}
		switch fe.Tag() {
		case "required":
			return fmt.Sprintf("请填写「%s」", field)
		case "min":
			return fmt.Sprintf("「%s」不能少于 %s 个字符", field, fe.Param())
		case "max":
			return fmt.Sprintf("「%s」不能超过 %s 个字符", field, fe.Param())
		case "email":
			return fmt.Sprintf("「%s」不是合法的邮箱地址", field)
		case "oneof":
			return fmt.Sprintf("「%s」取值不合法，可选值：%s", field, strings.ReplaceAll(fe.Param(), " ", " / "))
		case "numeric":
			return fmt.Sprintf("「%s」必须是数字", field)
		case "gte":
			return fmt.Sprintf("「%s」不能小于 %s", field, fe.Param())
		case "lte":
			return fmt.Sprintf("「%s」不能大于 %s", field, fe.Param())
		default:
			return fmt.Sprintf("「%s」填写不正确（%s）", field, fe.Tag())
		}
	}
	// JSON 结构错误等
	msg := err.Error()
	if strings.Contains(msg, "cannot unmarshal") || strings.Contains(msg, "unexpected end of JSON") {
		return "请求参数格式不正确（不是合法的 JSON 或字段类型不匹配）"
	}
	return "请求参数校验失败：" + msg
}

// fieldLabels 结构体字段 → 中文名（未登记的回落到字段名本身）。
var fieldLabels = map[string]string{
	"Username": "账号", "Password": "密码", "OldPassword": "当前密码", "NewPassword": "新密码",
	"Nickname": "昵称", "Email": "邮箱", "Mobile": "手机号", "Name": "名称", "Title": "标题",
	"Content": "内容", "Cron": "cron 表达式", "DeptID": "部门", "RoleIDs": "角色", "Ids": "ID 列表",
	"SiteName": "网站名称", "LoginDesc": "登录页描述", "LoginSubTitle": "登录页副标题",
	"Page": "页码", "PageSize": "每页条数", "Status": "状态", "Remark": "备注",
}

// httpStatus 业务码 → HTTP 状态码。
func httpStatus(code int) int {
	switch {
	case code == 0:
		return http.StatusOK
	case code == errs.ErrNotFound.Code:
		return http.StatusNotFound
	case code == errs.ErrDuplicate.Code:
		return http.StatusConflict
	case code >= 20000 && code < 30000:
		return http.StatusUnauthorized
	case code >= 30000 && code < 40000:
		return http.StatusForbidden
	case code >= 50000 && code < 60000:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
