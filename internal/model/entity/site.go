package entity

// SiteConfig 站点个性化配置（单行表，ID 固定为 1）。
// 前端启动时读取公开接口后写入全局配置：网站名称、logo、登录页背景与文案、注册开关。
type SiteConfig struct {
	Base
	SiteName        string `gorm:"column:site_name" json:"siteName"`
	SiteLogo        string `gorm:"column:site_logo" json:"siteLogo"`
	LoginBg         string `gorm:"column:login_bg" json:"loginBg"`
	LoginDesc       string `gorm:"column:login_desc" json:"loginDesc"`
	LoginSubTitle   string `gorm:"column:login_sub_title" json:"loginSubTitle"`
	RegisterEnabled bool   `gorm:"column:register_enabled" json:"registerEnabled"`
}

// TableName 表名。
func (SiteConfig) TableName() string { return "sys_site_config" }
