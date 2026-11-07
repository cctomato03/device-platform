package service

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 运行期开关：把「参数配置(sys_config)」里少量安全相关开关做成可热更新，
// 避免为了开关验证码就要改配置文件 + 重启服务。
//
// 取值优先级：环境变量/配置文件（作为总闸） && 数据库开关（后台可改）。
// 数据库每 5 秒最多读一次，避免登录页每次刷新都打库。

// switchTTL 数据库开关缓存时间。
const switchTTL = 5 * time.Second

type cachedSwitch struct {
	value   bool
	expires time.Time
}

var (
	switchCacheMu sync.RWMutex
	switchCache   = map[string]cachedSwitch{}
)

// configBool 读取布尔型参数配置；值缺失或解析失败时返回 fallback。
func (s *Service) configBool(ctx context.Context, key string, fallback bool) bool {
	switchCacheMu.RLock()
	if c, ok := switchCache[key]; ok && time.Now().Before(c.expires) {
		switchCacheMu.RUnlock()
		return c.value
	}
	switchCacheMu.RUnlock()

	value := fallback
	var row struct{ Value string }
	err := s.Repo.DB.WithContext(ctx).
		Model(nil).
		Table("sys_config").
		Select("value").
		Where("config_key = ?", key).
		Limit(1).
		Scan(&row).Error
	if err == nil && strings.TrimSpace(row.Value) != "" {
		if b, perr := strconv.ParseBool(strings.TrimSpace(row.Value)); perr == nil {
			value = b
		}
	}

	switchCacheMu.Lock()
	switchCache[key] = cachedSwitch{value: value, expires: time.Now().Add(switchTTL)}
	switchCacheMu.Unlock()
	return value
}

// invalidateSwitchCache 清理开关缓存（参数变更后调用，让开关立刻生效）。
func InvalidateSwitchCache(keys ...string) {
	switchCacheMu.Lock()
	defer switchCacheMu.Unlock()
	if len(keys) == 0 {
		switchCache = map[string]cachedSwitch{}
		return
	}
	for _, k := range keys {
		delete(switchCache, k)
	}
}

// 参数配置键名（与 migrations 里的种子数据保持一致）
const (
	// ConfigKeyLoginCaptcha 登录图形验证码开关（公开参数，登录页据此显示/隐藏验证码）
	ConfigKeyLoginCaptcha = "sys.login.captcha"
	// ConfigKeyLoginSlider 登录滑块校验开关（纯前端交互，公开参数）
	ConfigKeyLoginSlider = "sys.login.slider"
	// ConfigKeyUploadDirect 是否支持前端直传（运行时值，随存储配置变化，不落库）
	ConfigKeyUploadDirect = "sys.upload.direct"
)

// loginCaptchaEnabled 登录是否校验图形验证码：
// 配置文件 auth.captcha_enabled 是总闸（false 则彻底关闭），开启时再受后台参数开关控制。
func (s *Service) loginCaptchaEnabled(ctx context.Context) bool {
	if s.Captcha == nil || !s.Captcha.Enabled() {
		return false
	}
	return s.configBool(ctx, ConfigKeyLoginCaptcha, true)
}

// LoginSliderEnabled 登录页是否启用滑块校验（供公开参数接口使用）。
func (s *Service) LoginSliderEnabled(ctx context.Context) bool {
	return s.configBool(ctx, ConfigKeyLoginSlider, true)
}
