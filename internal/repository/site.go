package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"device-platform/internal/model/entity"
)

// SiteConfigID 站点配置单行表主键（业务层更新时复用，避免散落魔法值 1）。
const SiteConfigID uint64 = 1

// GetSiteConfig 读取站点配置（不存在时返回默认值与 nil 错误，保证公开接口永远可用）。
func (r *Repo) GetSiteConfig(ctx context.Context) (*entity.SiteConfig, error) {
	var cfg entity.SiteConfig
	err := r.DB.WithContext(ctx).Where("id = ?", SiteConfigID).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 兜底默认值与迁移脚本里的初始行保持一致
			return &entity.SiteConfig{
				Base:            entity.Base{ID: SiteConfigID},
				SiteName:        "Web Basic Admin",
				LoginDesc:       "一款开箱即用的 Go + Vue3 后台管理脚手架",
				LoginSubTitle:   "美观实用的界面，经过视觉优化，确保卓越的用户体验",
				RegisterEnabled: false,
			}, nil
		}
		return nil, err
	}
	return &cfg, nil
}
