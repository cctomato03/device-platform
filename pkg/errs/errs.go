// Package errs 统一业务错误定义。
// 约定：错误码分段见 docs/04-权限模型与API规范.md §5.3
package errs

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// AppError 业务错误，handler 层统一转换为标准响应体。
type AppError struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Err  error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

func (e *AppError) Unwrap() error { return e.Err }

// WithErr 附加底层错误（返回副本，避免并发修改共享变量）。
func (e *AppError) WithErr(err error) *AppError {
	c := *e
	c.Err = err
	return &c
}

// New 创建业务错误。
func New(code int, msg string) *AppError { return &AppError{Code: code, Msg: msg} }

// Invalid 构造「参数校验失败 + 具体原因」的业务错误：message 直接给前端展示。
func Invalid(msg string) *AppError { return &AppError{Code: ErrBadRequest.Code, Msg: msg} }

// InvalidErr 把校验类错误转换成可读的业务错误（message = 底层错误文本）。
//
// 只用于「用户输入/业务规则」这类可以直说原因的场景；数据库/系统错误请继续用 ErrDB/ErrInternal，
// 避免把内部细节（SQL、表名）暴露到前端。
func InvalidErr(err error) *AppError {
	if err == nil {
		return Invalid(ErrBadRequest.Msg)
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = ErrBadRequest.Msg
	}
	return Invalid(msg)
}

// Normalize 把底层错误（数据库/驱动）归一化为业务错误，
// 避免唯一键冲突、记录不存在这类可预期情况被当成 500 内部错误返回给前端。
func Normalize(err error) error {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		// 包装过的错误（如 ErrDB.WithErr(dbErr)）要看底层原因
		if ae.Err != nil {
			if mapped := normalizeCause(ae.Err); mapped != nil {
				return mapped.WithErr(ae.Err)
			}
		}
		return err
	}
	if mapped := normalizeCause(err); mapped != nil {
		return mapped.WithErr(err)
	}
	return err
}

// normalizeCause 识别常见数据库错误；无法识别返回 nil。
func normalizeCause(err error) *AppError {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), strings.Contains(msg, "record not found"):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey),
		strings.Contains(msg, "duplicate entry"),
		strings.Contains(msg, "duplicated key not allowed"),
		strings.Contains(msg, "unique constraint"):
		return ErrDuplicate
	case strings.Contains(msg, "foreign key constraint"):
		return ErrRecordUsed
	case strings.Contains(msg, "data too long"),
		strings.Contains(msg, "out of range value"),
		strings.Contains(msg, "incorrect string value"),
		strings.Contains(msg, "truncated incorrect"):
		return ErrFieldTooLong
	}
	return nil
}

// ---------- 通用 1xxxx ----------

var (
	ErrBadRequest   = New(10001, "参数校验失败")
	ErrBodyInvalid  = New(10002, "请求体格式错误")
	ErrMissingField = New(10003, "缺少必填项")
	ErrNotFound     = New(10004, "数据不存在")
	ErrDuplicate    = New(10005, "数据已存在")
	ErrTooManyReqs  = New(10006, "请求过于频繁，请稍后再试")
	ErrRecordUsed   = New(10007, "数据已被引用，无法操作")
	ErrFieldTooLong = New(10008, "字段内容超出长度限制")
)

// ---------- 认证 2xxxx ----------

var (
	ErrUnauthorized    = New(20001, "未登录或登录已失效")
	ErrTokenExpired    = New(20002, "登录已过期，请重新登录")
	ErrTokenInvalid    = New(20003, "令牌无效")
	ErrRefreshInvalid  = New(20004, "刷新令牌已失效，请重新登录")
	ErrCaptchaInvalid  = New(20005, "验证码错误或已过期")
	ErrPasswordWrong   = New(20006, "账号或密码错误")
	ErrAccountLocked   = New(20007, "账号已被锁定，请稍后再试")
	ErrAccountDisabled = New(20008, "账号已被停用")
	ErrPasswordExpired = New(20009, "密码已过期，请修改密码")
	ErrSessionKicked   = New(20010, "账号已在其他设备登录")
	ErrTenantDisabled  = New(20011, "租户已停用或不存在")
)

// ---------- 权限 3xxxx ----------

var (
	ErrForbidden        = New(30001, "没有操作权限")
	ErrDataForbidden    = New(30002, "没有数据权限")
	ErrCannotOperateSup = New(30003, "不能操作超级管理员")
	ErrCannotOperateOwn = New(30004, "不能操作当前登录账号")
)

// ---------- 业务 4xxxx ----------

var (
	ErrInvalidStatus = New(40001, "当前状态不允许该操作")
	ErrHasChildren   = New(40002, "存在子节点，无法删除")
	ErrJobFailed     = New(40004, "任务执行失败")
)

// ---------- 系统 5xxxx ----------

var (
	ErrInternal    = New(50000, "系统内部错误")
	ErrDB          = New(50001, "数据库操作异常")
	ErrCache       = New(50002, "缓存服务异常")
	ErrStorage     = New(50003, "文件存储异常")
	ErrThirdParty  = New(50004, "第三方服务调用失败")
	ErrUnavailable = New(50005, "依赖服务不可用")
)

// ---------- 文件 6xxxx ----------

var (
	ErrFileTooLarge = New(60001, "文件大小超出限制")
	ErrFileType     = New(60002, "文件类型不允许")
	ErrUploadFailed = New(60003, "文件上传失败")
	ErrFileNotFound = New(60004, "文件不存在")
)
