package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/principal"
	"device-platform/pkg/crypto"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
	"device-platform/pkg/jwt"
)

// LoginReq 登录入参。
type LoginReq struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
	TenantID    uint64 `json:"tenantId,string"`
}

// CaptchaResp 验证码出参。
type CaptchaResp struct {
	Enabled   bool   `json:"enabled"`
	CaptchaID string `json:"captchaId"`
	Image     string `json:"image"`
}

// TokenResp 令牌出参。
type TokenResp struct {
	AccessToken  string          `json:"accessToken"`
	RefreshToken string          `json:"refreshToken"`
	ExpiresIn    int64           `json:"expiresIn"`
	User         *principal.Info `json:"user,omitempty"`
}

// CaptchaInfo 生成验证码（开关关闭时返回 enabled=false，前端据此隐藏输入框）。
func (s *Service) CaptchaInfo(ctx context.Context) (*CaptchaResp, error) {
	if !s.loginCaptchaEnabled(ctx) {
		return &CaptchaResp{Enabled: false}, nil
	}
	cid, img, err := s.Captcha.Generate()
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return &CaptchaResp{Enabled: true, CaptchaID: cid, Image: img}, nil
}

// Login 账号密码登录。
func (s *Service) Login(ctx context.Context, req *LoginReq, ip, ua string) (*TokenResp, error) {
	if s.loginCaptchaEnabled(ctx) && !s.Captcha.Verify(req.CaptchaID, req.CaptchaCode) {
		s.writeLoginLog(ctx, req.Username, 0, 0, "验证码错误", ip, ua, 0)
		return nil, errs.ErrCaptchaInvalid
	}

	tenantID := req.TenantID
	if tenantID == 0 {
		tenantID = s.Cfg.Tenant.DefaultID
	}

	user, err := s.Repo.FindUserByUsername(ctx, tenantID, strings.TrimSpace(req.Username))
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if user == nil {
		s.writeLoginLog(ctx, req.Username, tenantID, 0, "账号不存在", ip, ua, 0)
		return nil, errs.ErrPasswordWrong
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		s.writeLoginLog(ctx, req.Username, tenantID, 0, "账号锁定中", ip, ua, 0)
		return nil, errs.ErrAccountLocked
	}
	if user.Status != 1 {
		s.writeLoginLog(ctx, req.Username, tenantID, 0, "账号已停用", ip, ua, 0)
		return nil, errs.ErrAccountDisabled
	}
	if !crypto.VerifyPassword(user.Password, req.Password) {
		_ = s.Repo.UpdateLoginFail(ctx, user.ID, s.Cfg.Auth.MaxFailCount, s.Cfg.Auth.LockMinutes)
		s.writeLoginLog(ctx, req.Username, tenantID, 0, "密码错误", ip, ua, 0)
		return nil, errs.ErrPasswordWrong
	}

	// 密码有效期（pwd_expire_days>0 时生效）：超期强制改密
	if s.Cfg.Auth.PwdExpireDays > 0 && user.PwdUpdateTime != nil {
		if user.PwdUpdateTime.AddDate(0, 0, s.Cfg.Auth.PwdExpireDays).Before(time.Now()) {
			s.writeLoginLog(ctx, req.Username, user.TenantID, 0, "密码已过期", ip, ua, user.ID)
			return nil, errs.ErrPasswordExpired
		}
	}

	// 租户校验（密码校验之后做，避免通过报错差异探测租户是否存在）：
	// 租户停用/已过期时不允许登录。
	if ae := s.checkTenantUsable(ctx, user.TenantID); ae != nil {
		s.writeLoginLog(ctx, req.Username, user.TenantID, 0, ae.Msg, ip, ua, 0)
		return nil, ae
	}

	if err := s.Repo.UpdateLoginSuccess(ctx, user.ID, ip); err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	s.writeLoginLog(ctx, user.Username, user.TenantID, 1, "登录成功", ip, ua, user.ID)

	return s.issueTokens(ctx, user)
}

// checkTenantUsable 校验租户是否可用（不存在 / 已停用 / 已过期 → 拒绝登录）。
func (s *Service) checkTenantUsable(ctx context.Context, tenantID uint64) *errs.AppError {
	if tenantID == 0 {
		return nil
	}
	var t entity.Tenant
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", tenantID).First(&t).Error; err != nil {
		return errs.ErrTenantDisabled
	}
	if t.Status != 1 {
		return errs.ErrTenantDisabled
	}
	if exp := strings.TrimSpace(t.ExpireAt); exp != "" {
		if d, perr := time.ParseInLocation("2006-01-02", exp, time.Local); perr == nil && d.AddDate(0, 0, 1).Before(time.Now()) {
			return errs.ErrTenantDisabled
		}
	}
	return nil
}

// issueTokens 签发双令牌、写主体缓存、维护在线会话。
func (s *Service) issueTokens(ctx context.Context, user *entity.User) (*TokenResp, error) {
	access, err := s.JWT.Generate(user.ID, user.TenantID, jwt.TypeAccess)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	refresh, err := s.JWT.Generate(user.ID, user.TenantID, jwt.TypeRefresh)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}

	// 刷新令牌登记（可撤销）：jti 单条 key + 用户维度集合（用于"踢下线/改密"整体撤销）
	_ = s.Cache.Set(ctx, fmt.Sprintf(keyRefresh, refresh.JTI), "1", s.JWT.RefreshTTL())
	_ = s.Cache.SetAdd(ctx, fmt.Sprintf(keyRefreshUser, user.ID), refresh.JTI)
	// 单点登录：记录当前会话 jti，旧会话自动失效
	if s.Cfg.Auth.SingleSession {
		_ = s.Cache.Set(ctx, fmt.Sprintf(keySSO, user.ID), access.JTI, s.JWT.RefreshTTL())
	}
	// 在线用户集合 + 会话索引（退出时按 uid+jti 精确删除，避免全量扫描集合）
	member := fmt.Sprintf("%d:%s:%d", user.ID, access.JTI, time.Now().Unix())
	_ = s.Cache.SetAdd(ctx, keyOnline, member)
	_ = s.Cache.Set(ctx, fmt.Sprintf(keyOnlineIdx, user.ID, access.JTI), member, s.JWT.RefreshTTL())

	p, err := s.loadPrincipal(ctx, user)
	if err != nil {
		return nil, err
	}
	p.JTI = access.JTI

	return &TokenResp{
		AccessToken:  access.Value,
		RefreshToken: refresh.Value,
		ExpiresIn:    int64(access.TTL.Seconds()),
		User:         p,
	}, nil
}

// Refresh 刷新令牌（轮换式：旧 refresh 立即作废）。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenResp, error) {
	claims, err := s.JWT.Parse(refreshToken, jwt.TypeRefresh)
	if err != nil {
		return nil, errs.ErrRefreshInvalid
	}
	key := fmt.Sprintf(keyRefresh, claims.ID)
	if !s.Cache.Exists(ctx, key) {
		return nil, errs.ErrRefreshInvalid
	}
	_ = s.Cache.Del(ctx, key) // 轮换：一次性使用

	user, err := s.Repo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if user == nil || user.Status != 1 {
		return nil, errs.ErrAccountDisabled
	}
	return s.issueTokens(ctx, user)
}

// Logout 退出登录：access 加入黑名单、该账号的 refresh token 全部作废、清理在线状态。
func (s *Service) Logout(ctx context.Context, accessToken string, p *principal.Info) error {
	claims, err := s.JWT.Parse(accessToken, jwt.TypeAccess)
	if err != nil {
		return nil
	}
	if ttl := time.Until(claims.ExpiresAt.Time); ttl > 0 {
		_ = s.Cache.Set(ctx, fmt.Sprintf(keyBlacklist, claims.ID), "1", ttl)
	}
	uid := claims.UserID
	if p != nil {
		uid = p.UserID
	}
	s.removeOnline(ctx, uid, claims.ID)
	s.revokeUserSessions(ctx, uid)
	return nil
}

// revokeUserSessions 撤销某账号的全部会话：refresh token 作废 + 单点登录标记清除。
// 退出登录、被强制下线、重置密码、修改密码后都必须调用，否则旧 refresh token 仍能换取新的 access token。
func (s *Service) revokeUserSessions(ctx context.Context, userID uint64) {
	idxKey := fmt.Sprintf(keyRefreshUser, userID)
	for _, jti := range s.Cache.SetMembers(ctx, idxKey) {
		_ = s.Cache.Del(ctx, fmt.Sprintf(keyRefresh, jti))
	}
	_ = s.Cache.Del(ctx, idxKey)
	_ = s.Cache.Del(ctx, fmt.Sprintf(keySSO, userID))
	s.invalidatePrincipal(ctx, userID)
}

// IsBlacklisted 判断 access 令牌是否已失效（退出登录/被踢）。
func (s *Service) IsBlacklisted(ctx context.Context, jti string) bool {
	return s.Cache.Exists(ctx, fmt.Sprintf(keyBlacklist, jti))
}

// CheckSingleSession 单点登录校验：开启时同一账号只允许一个有效会话。
func (s *Service) CheckSingleSession(ctx context.Context, uid uint64, jti string) bool {
	if !s.Cfg.Auth.SingleSession {
		return true
	}
	v, ok := s.Cache.Get(ctx, fmt.Sprintf(keySSO, uid))
	return ok && v == jti
}

// removeOnline 从在线集合移除。
//
// 优先用会话索引（uid+jti → 成员）精确删除，O(1)；
// 索引缺失时（例如升级前的历史会话）退回一次集合扫描，并且只发一次批量 SREM。
func (s *Service) removeOnline(ctx context.Context, userID uint64, jti string) {
	idxKey := fmt.Sprintf(keyOnlineIdx, userID, jti)
	if member, ok := s.Cache.Get(ctx, idxKey); ok && member != "" {
		_ = s.Cache.SetRemove(ctx, keyOnline, member)
		_ = s.Cache.Del(ctx, idxKey)
		return
	}
	prefix := fmt.Sprintf("%d:%s", userID, jti)
	stale := make([]string, 0, 4)
	for _, m := range s.Cache.SetMembers(ctx, keyOnline) {
		if strings.HasPrefix(m, prefix) {
			stale = append(stale, m)
		}
	}
	if len(stale) > 0 {
		_ = s.Cache.SetRemove(ctx, keyOnline, stale...)
	}
}

// OnlineUser 在线会话条目（用户资料由批量查询补齐）。
type OnlineUser struct {
	UserID    uint64 `json:"userId,string"`
	JTI       string `json:"jti"`
	LoginTime int64  `json:"loginTime"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
}

// OnlineUsers 在线用户列表（来自 Redis 集合）。
//
// 性能约定：用户信息必须**一次 IN 批量查询**，不能在循环里逐个查库。
// 早期实现每个会话查一次用户表，350 个在线会话会让接口放大到 350 次查询（远程库上十几秒）。
func (s *Service) OnlineUsers(ctx context.Context) ([]OnlineUser, error) {
	members := s.Cache.SetMembers(ctx, keyOnline)
	out := make([]OnlineUser, 0, len(members))
	uids := make([]uint64, 0, len(members))
	for _, m := range members {
		parts := strings.Split(m, ":")
		if len(parts) < 3 {
			continue
		}
		var uid uint64
		fmt.Sscanf(parts[0], "%d", &uid)
		var ts int64
		fmt.Sscanf(parts[2], "%d", &ts)
		out = append(out, OnlineUser{UserID: uid, JTI: parts[1], LoginTime: ts})
		if uid > 0 {
			uids = append(uids, uid)
		}
	}
	if len(uids) == 0 {
		return out, nil
	}

	users, err := s.Repo.FindUsersByIDs(ctx, uids)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	for i := range out {
		if u, ok := users[out[i].UserID]; ok && u != nil {
			out[i].Username = u.Username
			out[i].Nickname = u.Nickname
		}
	}
	return out, nil
}

// OnlineCount 在线会话数（只读 Redis 集合大小，不查用户表）。
//
// 工作台只需要数量，没必要把会话与用户全查出来——这是「统计卡片一直转圈」的直接原因之一。
func (s *Service) OnlineCount(ctx context.Context) int {
	return len(s.Cache.SetMembers(ctx, keyOnline))
}

// KickUser 强制下线：access 拉黑 + refresh 全部作废。
func (s *Service) KickUser(ctx context.Context, userID uint64, jti string) error {
	s.removeOnline(ctx, userID, jti)
	if jti != "" {
		_ = s.Cache.Set(ctx, fmt.Sprintf(keyBlacklist, jti), "1", s.JWT.RefreshTTL())
	}
	s.revokeUserSessions(ctx, userID)
	return nil
}

// CurrentPrincipal 读取（或回源重建）登录主体。
func (s *Service) CurrentPrincipal(ctx context.Context, userID uint64) (*principal.Info, error) {
	var p principal.Info
	if s.Cache.GetJSON(ctx, s.principalKey(ctx, userID), &p) && p.UserID != 0 {
		p.BuildSet()
		return &p, nil
	}
	user, err := s.Repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if user == nil || user.Status != 1 {
		return nil, errs.ErrUnauthorized
	}
	return s.loadPrincipal(ctx, user)
}

// loadPrincipal 汇总用户权限、角色、数据范围并写缓存。
func (s *Service) loadPrincipal(ctx context.Context, user *entity.User) (*principal.Info, error) {
	roles, err := s.Repo.RolesOfUser(ctx, user.ID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	perms, err := s.Repo.PermsOfUser(ctx, user.ID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}

	dataScope := entity.DataScopeSelf
	roleCodes := make([]string, 0, len(roles))
	roleIDs := make([]uint64, 0, len(roles))
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
		roleIDs = append(roleIDs, r.ID)
		if r.DataScope < dataScope { // 数字越小范围越大，取最宽
			dataScope = r.DataScope
		}
	}
	if len(roles) == 0 {
		dataScope = entity.DataScopeSelf
	}
	if user.IsSuper {
		dataScope = entity.DataScopeAll
	}

	p := &principal.Info{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		DeptID:    user.DeptID,
		Avatar:    user.Avatar,
		Email:     user.Email,
		Mobile:    user.Mobile,
		Sex:       user.Sex,
		IsSuper:   user.IsSuper,
		DataScope: dataScope,
		Roles:     roleCodes,
		Perms:     perms,
	}

	if dept, err := s.Repo.FindDeptByID(ctx, user.DeptID); err == nil && dept != nil {
		p.DeptName = dept.Name
	}
	if dataScope == entity.DataScopeDeptAndSub {
		if ids, err := s.Repo.SubtreeDeptIDs(ctx, user.DeptID); err == nil {
			p.DeptIDs = ids
		}
	}
	if dataScope == entity.DataScopeCustom {
		if ids, err := s.Repo.CustomDeptIDsOfRoles(ctx, roleIDs); err == nil {
			p.CustomDeptIDs = ids
		}
	}
	p.BuildSet()
	_ = s.Cache.SetJSON(ctx, s.principalKey(ctx, user.ID), p, principalTTL)
	return p, nil
}

// writeLoginLog 写登录日志（异步失败不影响主流程）。
func (s *Service) writeLoginLog(ctx context.Context, username string, tenantID uint64, status int, msg, ip, ua string, userID uint64) {
	log := &entity.LoginLog{
		ID:        id.Next(),
		Username:  username,
		LoginType: 1,
		Status:    status,
		Msg:       msg,
		IP:        ip,
		UserAgent: ua,
		TenantID:  tenantID,
		LoginTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	_ = s.Repo.DB.WithContext(context.WithoutCancel(ctx)).Create(log).Error
}

// IsInvalidToken 判定令牌错误类型。
func (s *Service) IsInvalidToken(err error) bool {
	return errors.Is(err, jwt.ErrInvalid) || errors.Is(err, jwt.ErrExpired)
}
