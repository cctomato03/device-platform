// Package service 业务逻辑层：事务边界、权限数据组装、外部能力编排。
package service

import (
	"context"
	"fmt"
	"time"

	"web-basic/internal/config"
	"web-basic/internal/repository"
	"web-basic/pkg/cache"
	"web-basic/pkg/captcha"
	"web-basic/pkg/jwt"
	"web-basic/pkg/storage"
)

// 缓存 key
const (
	keyPrincipal        = "auth:principal:%s:%d" // 用户主体（权限/部门范围），前缀为全局权限版本
	keyPrincipalVersion = "auth:principal:ver"   // 全局权限版本（菜单/角色/部门变更时自增）
	keyRefresh          = "auth:refresh:%s"      // 刷新令牌 jti
	keyRefreshUser      = "auth:refresh:user:%d" // 用户持有的 refresh jti 集合（用于整体撤销）
	keyBlacklist        = "auth:blacklist:%s"    // 已退出/失效的 access jti
	keySSO              = "auth:sso:%d"          // 单点登录当前 jti
	keyOnline           = "auth:online"          // 在线用户集合
	principalTTL        = 30 * time.Minute
)

// Service 业务服务聚合入口。
type Service struct {
	Repo    *repository.Repo
	Cache   *cache.Cache
	Cfg     *config.Config
	JWT     *jwt.Manager
	Captcha *captcha.Service
	Storage storage.Storage
}

func New(deps *Deps) *Service {
	return &Service{
		Repo:    deps.Repo,
		Cache:   deps.Cache,
		Cfg:     deps.Cfg,
		JWT:     deps.JWT,
		Captcha: deps.Captcha,
		Storage: deps.Storage,
	}
}

// Deps 依赖集合（由 bootstrap 装配）。
type Deps struct {
	Repo    *repository.Repo
	Cache   *cache.Cache
	Cfg     *config.Config
	JWT     *jwt.Manager
	Captcha *captcha.Service
	Storage storage.Storage
}

// principalVersion 当前全局权限版本；未初始化时生成一个（永久 key）。
func (s *Service) principalVersion(ctx context.Context) string {
	if v, ok := s.Cache.Get(ctx, keyPrincipalVersion); ok && v != "" {
		return v
	}
	v := fmt.Sprintf("%d", time.Now().UnixNano())
	_ = s.Cache.Set(ctx, keyPrincipalVersion, v, 0)
	return v
}

// principalKey 用户主体缓存 key（带版本号，版本变化即全量失效）。
func (s *Service) principalKey(ctx context.Context, userID uint64) string {
	return fmt.Sprintf(keyPrincipal, s.principalVersion(ctx), userID)
}

// invalidatePrincipal 清除用户主体缓存（角色/菜单/数据范围变更后必须调用）。
func (s *Service) invalidatePrincipal(ctx context.Context, userIDs ...uint64) {
	for _, uid := range userIDs {
		_ = s.Cache.Del(ctx, fmt.Sprintf(keyPrincipal, s.principalVersion(ctx), uid))
	}
}

// InvalidateRoleUsers 角色变更后失效其下所有用户的权限缓存。
func (s *Service) InvalidateRoleUsers(ctx context.Context, roleID uint64) {
	ids, err := s.Repo.UserIDsOfRole(ctx, roleID)
	if err != nil {
		return
	}
	s.invalidatePrincipal(ctx, ids...)
}

// InvalidateAll 全量失效：更换权限版本号，所有用户的主体缓存立即不可见。
// 注意：必须替换版本号（而不是删除某个从不被读取的 key），否则权限变更最长要等 principalTTL 才生效。
func (s *Service) InvalidateAll(ctx context.Context) {
	_ = s.Cache.Set(ctx, keyPrincipalVersion, fmt.Sprintf("%d", time.Now().UnixNano()), 0)
}
