package service

import (
	"context"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
)

// SiteConfigReq 站点配置入参。
type SiteConfigReq struct {
	SiteName        string `json:"siteName" binding:"required,max=64"`
	SiteLogo        string `json:"siteLogo" binding:"max=512"`
	LoginBg         string `json:"loginBg" binding:"max=512"`
	LoginDesc       string `json:"loginDesc" binding:"max=255"`
	LoginSubTitle   string `json:"loginSubTitle" binding:"max=255"`
	RegisterEnabled bool   `json:"registerEnabled"`
}

// SiteConfigVO 站点配置出参（公开接口与后台共用）。
type SiteConfigVO struct {
	SiteName        string `json:"siteName"`
	SiteLogo        string `json:"siteLogo"`
	LoginBg         string `json:"loginBg"`
	LoginDesc       string `json:"loginDesc"`
	LoginSubTitle   string `json:"loginSubTitle"`
	RegisterEnabled bool   `json:"registerEnabled"`
	UpdateTime      string `json:"updateTime"`
}

// SiteConfig 读取站点配置（公开接口，免登录；表为空时返回内置默认值）。
func (s *Service) SiteConfig(ctx context.Context) (*SiteConfigVO, error) {
	cfg, err := s.Repo.GetSiteConfig(ctx)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return toSiteConfigVO(cfg), nil
}

// UpdateSiteConfig 保存站点配置（单行表：存在则更新并记录字段级 diff，不存在则创建）。
func (s *Service) UpdateSiteConfig(ctx context.Context, req *SiteConfigReq, p *principal.Info) (*SiteConfigVO, error) {
	req.SiteName = strings.TrimSpace(req.SiteName)
	req.SiteLogo = strings.TrimSpace(req.SiteLogo)
	req.LoginBg = strings.TrimSpace(req.LoginBg)
	req.LoginDesc = strings.TrimSpace(req.LoginDesc)
	req.LoginSubTitle = strings.TrimSpace(req.LoginSubTitle)
	if req.SiteName == "" {
		return nil, errs.Invalid("网站名称不能为空")
	}
	if !validResourceRef(req.SiteLogo) {
		return nil, errs.Invalid("logo 地址需为站内路径（/static/...）或 http(s) 链接")
	}
	if !validResourceRef(req.LoginBg) {
		return nil, errs.Invalid("背景图地址需为站内路径（/static/...）或 http(s) 链接")
	}

	now := time.Now()
	fields := map[string]any{
		"site_name":        req.SiteName,
		"site_logo":        req.SiteLogo,
		"login_bg":         req.LoginBg,
		"login_desc":       req.LoginDesc,
		"login_sub_title":  req.LoginSubTitle,
		"register_enabled": req.RegisterEnabled,
		"update_by":        p.UserID,
		"update_time":      now,
	}

	err := repository.UpdateByID(ctx, s.Repo.DB, &entity.SiteConfig{}, repository.SiteConfigID, fields)
	if err != nil {
		if !repository.IsNotFound(err) {
			return nil, errs.ErrDB.WithErr(err)
		}
		// 首行缺失（数据库被人工清空）：直接插入默认行
		row := &entity.SiteConfig{
			Base:            entity.Base{ID: repository.SiteConfigID, CreateBy: p.UserID, CreateTime: now, UpdateTime: &now},
			SiteName:        req.SiteName,
			SiteLogo:        req.SiteLogo,
			LoginBg:         req.LoginBg,
			LoginDesc:       req.LoginDesc,
			LoginSubTitle:   req.LoginSubTitle,
			RegisterEnabled: req.RegisterEnabled,
		}
		if err := s.Repo.DB.WithContext(ctx).Create(row).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
	}

	cfg, err := s.Repo.GetSiteConfig(ctx)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return toSiteConfigVO(cfg), nil
}

// validResourceRef 校验图片地址：允许空（用内置默认）、站内绝对路径、http(s) 地址。
func validResourceRef(v string) bool {
	if v == "" {
		return true
	}
	if strings.HasPrefix(v, "/") {
		return true
	}
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}

func toSiteConfigVO(cfg *entity.SiteConfig) *SiteConfigVO {
	vo := &SiteConfigVO{
		SiteName:        cfg.SiteName,
		SiteLogo:        cfg.SiteLogo,
		LoginBg:         cfg.LoginBg,
		LoginDesc:       cfg.LoginDesc,
		LoginSubTitle:   cfg.LoginSubTitle,
		RegisterEnabled: cfg.RegisterEnabled,
	}
	if cfg.UpdateTime != nil {
		vo.UpdateTime = cfg.UpdateTime.Format("2006-01-02 15:04:05")
	}
	return vo
}
