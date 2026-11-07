// Package config 集中定义配置结构与加载逻辑（yaml + 环境变量覆盖）。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App         App         `mapstructure:"app"`
	MySQL       MySQL       `mapstructure:"mysql"`
	Redis       Redis       `mapstructure:"redis"`
	JWT         JWT         `mapstructure:"jwt"`
	Auth        Auth        `mapstructure:"auth"`
	Upload      Upload      `mapstructure:"upload"`
	Storage     Storage     `mapstructure:"storage"`
	Log         Log         `mapstructure:"log"`
	Tenant      Tenant      `mapstructure:"tenant"`
	Metrics     Metrics     `mapstructure:"metrics"`
	Swagger     Swagger     `mapstructure:"swagger"`
	Scheduler   Scheduler   `mapstructure:"scheduler"`
	Integration Integration `mapstructure:"integration"`
}

// Integration 业务系统对接（开放接口）。
//
// 业务系统（OA/ERP/工单等）通过 `POST /api/v1/open/messages|todos` + `X-API-KEY`
// 把消息与待办推进本系统；biz_key 做幂等，重试安全。
type Integration struct {
	// Enabled 是否开放（默认关闭，未配置 key 时接口返回 404）
	Enabled bool `mapstructure:"enabled"`
	// APIKey 对接密钥（生产请用环境变量 APP__INTEGRATION__API_KEY 覆盖）
	APIKey string `mapstructure:"api_key"`
	// TenantID 开放接口写入的租户（默认 1）
	TenantID uint64 `mapstructure:"tenant_id"`
}

type App struct {
	Name          string   `mapstructure:"name"`
	Env           string   `mapstructure:"env"` // dev | test | prod
	Port          int      `mapstructure:"port"`
	Mode          string   `mapstructure:"mode"` // debug | release
	Timezone      string   `mapstructure:"timezone"`
	Lang          string   `mapstructure:"lang"`
	CorsOrigins   []string `mapstructure:"cors_origins"`
	AdminPassword string   `mapstructure:"admin_password"` // 仅用于首次初始化种子超管
}

type MySQL struct {
	DSN             string `mapstructure:"dsn"`
	MaxIdle         int    `mapstructure:"max_idle"`
	MaxOpen         int    `mapstructure:"max_open"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_min"`
	SlowThresholdMS int    `mapstructure:"slow_threshold_ms"`
	AutoMigrate     bool   `mapstructure:"auto_migrate"`
	LogSQL          bool   `mapstructure:"log_sql"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
	PoolSize int    `mapstructure:"pool_size"`
}

type JWT struct {
	Secret     string        `mapstructure:"secret"`
	SecretPrev string        `mapstructure:"secret_previous"` // 轮换期兼容旧密钥
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	Issuer     string        `mapstructure:"issuer"`
}

type Auth struct {
	SingleSession  bool `mapstructure:"single_session"`
	MaxFailCount   int  `mapstructure:"max_fail_count"`
	LockMinutes    int  `mapstructure:"lock_minutes"`
	CaptchaEnabled bool `mapstructure:"captcha_enabled"`
	PwdMinLen      int  `mapstructure:"pwd_min_len"`
	PwdExpireDays  int  `mapstructure:"pwd_expire_days"`
}

type Upload struct {
	MaxSizeMB int      `mapstructure:"max_size_mb"`
	AllowExt  []string `mapstructure:"allow_ext"`
}

type Storage struct {
	Type  string     `mapstructure:"type"` // local | oss
	Local LocalStore `mapstructure:"local"`
	OSS   OSSStore   `mapstructure:"oss"`
}

type LocalStore struct {
	BaseDir   string `mapstructure:"base_dir"`
	URLPrefix string `mapstructure:"url_prefix"`
}

type OSSStore struct {
	Endpoint   string `mapstructure:"endpoint"`
	Bucket     string `mapstructure:"bucket"`
	AccessKey  string `mapstructure:"access_key"`
	SecretKey  string `mapstructure:"secret_key"`
	Domain     string `mapstructure:"domain"`
	Private    bool   `mapstructure:"private"`
	SignExpire int    `mapstructure:"sign_expire"`
	// DirectUpload 允许前端直传：后端只签名，文件字节从浏览器直接进 OSS，
	// 省掉「浏览器 → 应用服务器 → OSS」的一次中转带宽。默认开启（仅 oss 类型生效）。
	DirectUpload bool `mapstructure:"direct_upload"`
	// VerifyExists 直传回执时是否向 OSS 校验对象真实存在（多一次 HEAD 请求，默认开启）。
	VerifyExists bool `mapstructure:"verify_exists"`
}

type Log struct {
	Level      string `mapstructure:"level"`
	Dir        string `mapstructure:"dir"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Console    bool   `mapstructure:"console"`
}

// Tenant 租户配置。说明：租户隔离随登录态常开（由 JWT 中的 tenantId 决定），
// 没有"关闭开关"，因此只保留默认租户 ID。
type Tenant struct {
	DefaultID uint64 `mapstructure:"default_id"`
}

// Metrics Prometheus 指标暴露配置。
type Metrics struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token"` // 非空时抓取需带 X-Metrics-Token 头或 ?token=
}

// Swagger 在线接口文档配置（/swagger/index.html）。
type Swagger struct {
	Enabled bool   `mapstructure:"enabled"`
	Title   string `mapstructure:"title"`
	Desc    string `mapstructure:"desc"`
}

type Scheduler struct {
	Enabled  bool `mapstructure:"enabled"`
	PoolSize int  `mapstructure:"pool_size"`
	// ShellEnabled 是否允许创建/执行「Shell 脚本」型定时任务。
	// 默认关闭：开启后等于允许后台用户在本机执行任意命令，必须在明确需要时才打开。
	ShellEnabled bool `mapstructure:"shell_enabled"`
}

// Load 读取配置文件并应用环境变量覆盖（APP__MYSQL__DSN 形式）。
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	// 环境变量覆盖：APP__<配置路径，层级用 __ 分隔>，例如
	//   APP__MYSQL__DSN、APP__APP__PORT、APP__STORAGE__OSS__BUCKET
	// 注意：不用 viper 的 SetEnvPrefix/AutomaticEnv —— 它拼出的名字是 APP_APP__PORT，
	// 且 Unmarshal 读不到"仅来自环境变量"的键，容器部署时覆盖会全部失效。
	applyEnvOverrides(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// envPrefix 环境变量前缀（层级分隔符为 __）。
const envPrefix = "APP__"

// applyEnvOverrides 把 APP__X__Y=value 写入 viper（优先级最高）。
func applyEnvOverrides(v *viper.Viper) {
	for _, kv := range os.Environ() {
		i := strings.Index(kv, "=")
		if i <= 0 {
			continue
		}
		name, val := kv[:i], kv[i+1:]
		if !strings.HasPrefix(name, envPrefix) {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(name, envPrefix), "__", "."))
		if key == "" {
			continue
		}
		// 切片类型（如 app.cors_origins）用逗号分隔
		if _, ok := v.Get(key).([]string); ok {
			parts := strings.Split(val, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			v.Set(key, out)
			continue
		}
		v.Set(key, val)
	}
}

func (c *Config) validate() error {
	if c.MySQL.DSN == "" {
		return fmt.Errorf("mysql.dsn 不能为空")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空（生产环境请通过环境变量 APP__JWT__SECRET 注入）")
	}
	if c.App.Port <= 0 {
		c.App.Port = 8080
	}
	loc, err := time.LoadLocation(c.App.Timezone)
	if err != nil {
		return fmt.Errorf("app.timezone 非法: %w", err)
	}
	time.Local = loc
	if c.App.Env == "prod" && c.Auth.CaptchaEnabled == false {
		// 生产环境建议开启验证码，仅提示不阻断
		fmt.Println("[WARN] 生产环境未开启登录验证码，请确认是否合理")
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "web-basic")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.mode", "debug")
	v.SetDefault("app.timezone", "Asia/Shanghai")
	v.SetDefault("app.lang", "zh-CN")
	v.SetDefault("app.cors_origins", []string{"http://localhost:5173", "http://127.0.0.1:5173"})

	v.SetDefault("mysql.max_idle", 10)
	v.SetDefault("mysql.max_open", 100)
	v.SetDefault("mysql.conn_max_lifetime_min", 60)
	v.SetDefault("mysql.slow_threshold_ms", 200)
	v.SetDefault("mysql.auto_migrate", true)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.prefix", "web:")
	v.SetDefault("redis.pool_size", 50)

	v.SetDefault("jwt.access_ttl", 2*time.Hour)
	v.SetDefault("jwt.refresh_ttl", 168*time.Hour)
	v.SetDefault("jwt.issuer", "web-basic")

	v.SetDefault("auth.max_fail_count", 5)
	v.SetDefault("auth.lock_minutes", 10)
	v.SetDefault("auth.captcha_enabled", true)
	v.SetDefault("auth.captcha_after_fail", 3)
	v.SetDefault("auth.pwd_min_len", 8)
	v.SetDefault("auth.pwd_expire_days", 0)

	v.SetDefault("upload.max_size_mb", 20)
	v.SetDefault("upload.allow_ext", []string{"jpg", "jpeg", "png", "gif", "webp", "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "zip", "txt", "mp4"})

	v.SetDefault("storage.type", "local")
	v.SetDefault("storage.local.base_dir", "./storage")
	v.SetDefault("storage.local.url_prefix", "/static")
	// OSS 直传：默认开启（仅 storage.type=oss 时生效），回执默认校验对象存在
	v.SetDefault("storage.oss.direct_upload", true)
	v.SetDefault("storage.oss.verify_exists", true)
	v.SetDefault("storage.oss.sign_expire", 900)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.dir", "./logs")
	v.SetDefault("log.max_size_mb", 100)
	v.SetDefault("log.max_backups", 30)
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.console", true)

	v.SetDefault("tenant.enabled", true)
	v.SetDefault("tenant.default_id", 1)
	v.SetDefault("tenant.header_key", "X-Tenant-Id")
	v.SetDefault("tenant.field_enabled", true)

	v.SetDefault("integration.enabled", false)
	v.SetDefault("integration.tenant_id", 1)

	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.pool_size", 10)
	// Shell 脚本型定时任务默认关闭（等于允许后台执行任意命令）
	v.SetDefault("scheduler.shell_enabled", false)
}
