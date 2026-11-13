package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/crypto"
	"device-platform/pkg/errs"
	"device-platform/pkg/response"
)

// Captcha 获取图形验证码（免登录）。
//
//	@Summary 获取图形验证码
//	@Tags 认证
//	@Success 200 {object} response.Body{data=service.CaptchaResp}
//	@Router /auth/captcha [get]
func (h *Handler) Captcha(c *gin.Context) {
	resp, err := h.Svc.CaptchaInfo(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, resp)
}

// Login 登录。
//
//	@Summary 账号密码登录
//	@Tags 认证
//	@Param body body service.LoginReq true "登录参数"
//	@Success 200 {object} response.Body{data=service.TokenResp}
//	@Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req service.LoginReq
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.Login(c.Request.Context(), &req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, resp)
}

// Refresh 刷新令牌。
//
//	@Summary 刷新访问令牌
//	@Tags 认证
//	@Router /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, resp)
}

// Logout 退出登录。
//
//	@Summary 退出登录
//	@Tags 认证
//	@Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if err := h.Svc.Logout(c.Request.Context(), token, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// UserInfo 当前登录用户信息（权限码 + 菜单树）。
//
//	@Summary 获取当前登录用户信息
//	@Tags 认证
//	@Success 200 {object} response.Body{data=service.UserInfoResp}
//	@Router /auth/userinfo [get]
func (h *Handler) UserInfo(c *gin.Context) {
	resp, err := h.Svc.UserInfo(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, resp)
}

// UserRoutes 当前用户的前端动态路由表（目录/菜单 + 按钮权限码）。
//
//	@Summary 获取当前用户动态路由
//	@Tags 认证
//	@Success 200 {object} response.Body{data=[]service.RouteNode}
//	@Router /auth/routes [get]
func (h *Handler) UserRoutes(c *gin.Context) {
	routes, err := h.Svc.UserRoutes(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, routes)
}

// PublicConfig 公开参数（系统名、Logo、验证码开关等，免登录）。
func (h *Handler) PublicConfig(c *gin.Context) {
	m, err := h.Svc.PublicConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, m)
}

// MyProfile 个人资料。
func (h *Handler) MyProfile(c *gin.Context) {
	p, err := h.Svc.MyProfile(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, p)
}

// UpdateMyProfile 修改个人资料。
func (h *Handler) UpdateMyProfile(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname" binding:"required,max=64"`
		Avatar   string `json:"avatar"`
		Mobile   string `json:"mobile"`
		Email    string `json:"email" binding:"omitempty,email"`
		Sex      int    `json:"sex"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateMyProfile(c.Request.Context(), req.Nickname, req.Avatar, req.Mobile, req.Email, req.Sex, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ChangePassword 修改自己的密码。
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6,max=32"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.ChangeMyPassword(c.Request.Context(), req.OldPassword, req.NewPassword, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// 确保 errs 包被使用（部分处理器仅透传错误）。
var _ = errs.ErrUnauthorized

// PasswordPolicy 密码强度规则（免登录，登录页/注册页/改密页展示用）。
//
//	@Summary 密码规则
//	@Tags 认证
//	@Success 200 {object} response.Body{data=crypto.PasswordPolicy}
//	@Router /public/password-policy [get]
func (h *Handler) PasswordPolicy(c *gin.Context) {
	response.OkData(c, crypto.PasswordPolicyOf(h.Svc.Cfg.Auth.PwdMinLen))
}
