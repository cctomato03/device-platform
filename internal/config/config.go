// Package config 集中定义配置结构与加载逻辑（yaml + 环境变量覆盖）。
//
// 本平台面向「单机设备」部署：数据库为嵌入式 SQLite，缓存为进程内实现，
// 不依赖任何外部中间件；所有运行时文件统一放在 app.home 指向的数据根目录下。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"device-platform/internal/paths"
)

// Config 配置根对象。
type Config struct {
	App         App         `mapstructure:"app"`
	Database    Database    `mapstructure:"database"`
	JWT         JWT         `mapstructure:"jwt"`
	Auth        Auth        `mapstructure:"auth"`
	Upload      Upload      `mapstructure:"upload"`
	Storage     Storage     `mapstructure:"storage"`
	Log         Log         `mapstructure:"log"`
	Tenant      Tenant      `mapstructure:"tenant"`
	Metrics     Metrics     `mapstructure:"metrics"`
	Swagger     Swagger     `mapstructure:"swagger"`
	Scheduler   Scheduler   `mapstructure:"scheduler"`
	Plugin      Plugin      `mapstructure:"plugin"`
	System      System      `mapstructure:"system"`
	Integration Integration `mapstructure:"integration"`
}

// App 应用基础配置。
type App struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"` // dev | test | prod
	// Home 数据根目录：数据库/日志/插件/上传文件全部在这里，便于整机备份与迁移。
	// 为空时依次取环境变量 DEVPLAT_HOME，再回落 ./runtime（开发模式）。
	Home          string   `mapstructure:"home"`
	Port          int      `mapstructure:"port"`
	Mode          string   `mapstructure:"mode"` // debug | release
	Timezone      string   `mapstructure:"timezone"`
	Lang          string   `mapstructure:"lang"`
	CorsOrigins   []string `mapstructure:"cors_origins"`
	AdminPassword string   `mapstructure:"admin_password"` // 仅用于首次初始化种子超管
}

// Database 嵌入式数据库配置（SQLite）。
type Database struct {
	// Driver 预留多驱动能力，当前仅支持 sqlite。
	Driver string `mapstructure:"driver"`
	// Path 数据库文件路径；为空时使用 <home>/data/device-platform.db。
	Path            string `mapstructure:"path"`
	MaxOpen         int    `mapstructure:"max_open"`
	BusyTimeoutMS   int    `mapstructure:"busy_timeout_ms"`
	WAL             bool   `mapstructure:"wal"`
	LogSQL          bool   `mapstructure:"log_sql"`
	SlowThresholdMS int    `mapstructure:"slow_threshold_ms"`
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
	// BaseDir 为空时使用 <home>/storage。
	BaseDir   string `mapstructure:"base_dir"`
	URLPrefix string `mapstructure:"url_prefix"`
}

type OSSStore struct {
	Endpoint     string `mapstructure:"endpoint"`
	Bucket       string `mapstructure:"bucket"`
	AccessKey    string `mapstructure:"access_key"`
	SecretKey    string `mapstructure:"secret_key"`
	Domain       string `mapstructure:"domain"`
	Private      bool   `mapstructure:"private"`
	SignExpire   int    `mapstructure:"sign_expire"`
	DirectUpload bool   `mapstructure:"direct_upload"`
	VerifyExists bool   `mapstructure:"verify_exists"`
}

type Log struct {
	Level string `mapstructure:"level"`
	// Dir 为空时使用 <home>/logs。
	Dir        string `mapstructure:"dir"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Console    bool   `mapstructure:"console"`
}

// Tenant 租户配置。设备平台通常单租户，保留兼容能力。
type Tenant struct {
	DefaultID uint64 `mapstructure:"default_id"`
}

type Metrics struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token"`
}

type Swagger struct {
	Enabled bool   `mapstructure:"enabled"`
	Title   string `mapstructure:"title"`
	Desc    string `mapstructure:"desc"`
}

type Scheduler struct {
	Enabled  bool `mapstructure:"enabled"`
	PoolSize int  `mapstructure:"pool_size"`
	// ShellEnabled 是否允许创建/执行「Shell 脚本」型定时任务（默认关闭）。
	ShellEnabled bool `mapstructure:"shell_enabled"`
}

// Plugin 插件市场与插件运行时配置。
type Plugin struct {
	// Dir 插件包安装目录，为空时使用 <home>/plugins。
	Dir string `mapstructure:"dir"`
	// DataDir 插件实例数据目录（配置、日志、持久化数据），为空时使用 <home>/plugin-data。
	DataDir string `mapstructure:"data_dir"`
	// AllowDocker 是否允许安装 docker 类型插件（设备未装 docker 时置 false）。
	AllowDocker bool `mapstructure:"allow_docker"`
	// AutoStart 平台启动时是否自动拉起 state=enabled 且 auto_start=true 的插件。
	AutoStart bool `mapstructure:"auto_start"`
	// InstallTimeoutSec 单个插件包安装解压/校验超时。
	InstallTimeoutSec int `mapstructure:"install_timeout_sec"`
	// LogTailLines 插件日志接口默认返回行数。
	LogTailLines int `mapstructure:"log_tail_lines"`
}

// System 设备管理域配置。
type System struct {
	// MonitorIntervalSec 硬件指标采集与告警判定周期。
	MonitorIntervalSec int `mapstructure:"monitor_interval_sec"`
	// MetricRetainHours 本地指标历史保留时长。
	MetricRetainHours int `mapstructure:"metric_retain_hours"`
	// ToolsEnabled 排查工具（ping/traceroute/tcpdump/netstat）总开关。
	ToolsEnabled bool `mapstructure:"tools_enabled"`
	// Tools 允许执行的命令白名单。
	Tools ToolAllow `mapstructure:"tools"`
	// LogRetainDays 系统日志/审计日志保留天数。
	LogRetainDays int `mapstructure:"log_retain_days"`
	// SnifferIface 流量嗅探默认网卡。
	SnifferIface string `mapstructure:"sniffer_iface"`
}

// ToolAllow 排查工具白名单（每项可独立关闭）。
type ToolAllow struct {
	Ping       bool `mapstructure:"ping"`
	Traceroute bool `mapstructure:"traceroute"`
	Tcpdump    bool `mapstructure:"tcpdump"`
	Netstat    bool `mapstructure:"netstat"`
}

// Integration 业务系统对接（开放接口）。
type Integration struct {
	Enabled  bool   `mapstructure:"enabled"`
	APIKey   string `mapstructure:"api_key"`
	TenantID uint64 `mapstructure:"tenant_id"`
}

// Load 读取配置文件并应用环境变量覆盖（APP__DATABASE__PATH 形式）。
//
// 配置文件不存在时不报错：使用内置默认值并把默认配置写入该路径，
// 这样一键安装脚本只需投放二进制，首次启动自动生成 conf/config.yaml。
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			if _, ok := err.(*os.PathError); !ok {
				return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
			}
		}
	}

	// 环境变量覆盖：APP__<配置路径，层级用 __ 分隔>，例如 APP__APP__PORT、APP__DATABASE__PATH
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

// ApplyHome 重新指定数据根目录并重新校验（命令行 -d 覆盖配置/环境变量）。
func (c *Config) ApplyHome(home string) error {
	c.App.Home = home
	abs, err := filepath.Abs(home)
	if err != nil {
		return fmt.Errorf("数据根目录非法: %w", err)
	}
	c.App.Home = abs
	return nil
}

// FillPaths 用统一目录布局补全所有未显式配置的路径。
//
// 这样运维只需要关心 app.home 一个配置项，其余目录自动派生，
// 「插件、日志、数据库、上传文件都在同一个目录里管理」。
func (c *Config) FillPaths(l *paths.Layout) {
	if c.Database.Path == "" {
		c.Database.Path = l.DBFile
	}
	if c.Log.Dir == "" {
		c.Log.Dir = l.Logs
	}
	if c.Storage.Type == "local" && c.Storage.Local.BaseDir == "" {
		c.Storage.Local.BaseDir = l.Storage
	}
	if c.Plugin.Dir == "" {
		c.Plugin.Dir = l.Plugins
	}
	if c.Plugin.DataDir == "" {
		c.Plugin.DataDir = l.PluginData
	}
}

// WriteDefault 把当前生效配置写回文件（首次启动生成 conf/config.yaml）。
func WriteDefault(path string, cfg *Config) error {
	v := viper.New()
	setDefaults(v)
	v.SetDefault("app.home", cfg.App.Home)
	v.SetDefault("app.admin_password", "")
	v.Set("database.path", cfg.Database.Path)
	v.Set("log.dir", cfg.Log.Dir)
	v.Set("storage.local.base_dir", cfg.Storage.Local.BaseDir)
	v.Set("plugin.dir", cfg.Plugin.Dir)
	v.Set("plugin.data_dir", cfg.Plugin.DataDir)
	v.Set("jwt.secret", cfg.JWT.Secret)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return v.WriteConfigAs(path)
}

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
	if c.App.Home == "" {
		if h := os.Getenv("DEVPLAT_HOME"); h != "" {
			c.App.Home = h
		} else {
			c.App.Home = "runtime"
		}
	}
	abs, err := filepath.Abs(c.App.Home)
	if err != nil {
		return fmt.Errorf("app.home 非法: %w", err)
	}
	c.App.Home = abs

	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.Driver != "sqlite" {
		return fmt.Errorf("database.driver 仅支持 sqlite（当前为 %s）", c.Database.Driver)
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空（生产环境请通过环境变量 APP__JWT__SECRET 注入）")
	}
	if c.App.Port <= 0 {
		c.App.Port = 8090
	}
	if c.App.Timezone == "" {
		c.App.Timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(c.App.Timezone)
	if err != nil {
		return fmt.Errorf("app.timezone 非法: %w", err)
	}
	time.Local = loc
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "device-platform")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.port", 8090)
	v.SetDefault("app.mode", "debug")
	v.SetDefault("app.timezone", "Asia/Shanghai")
	v.SetDefault("app.lang", "zh-CN")
	v.SetDefault("app.cors_origins", []string{"http://localhost:5173", "http://127.0.0.1:5173"})

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.max_open", 8)
	v.SetDefault("database.busy_timeout_ms", 10000)
	v.SetDefault("database.wal", true)
	v.SetDefault("database.slow_threshold_ms", 200)

	v.SetDefault("jwt.access_ttl", 2*time.Hour)
	v.SetDefault("jwt.refresh_ttl", 168*time.Hour)
	v.SetDefault("jwt.issuer", "device-platform")

	v.SetDefault("auth.max_fail_count", 5)
	v.SetDefault("auth.lock_minutes", 10)
	v.SetDefault("auth.captcha_enabled", true)
	v.SetDefault("auth.pwd_min_len", 8)
	v.SetDefault("auth.pwd_expire_days", 0)

	v.SetDefault("upload.max_size_mb", 20)
	v.SetDefault("upload.allow_ext", []string{"jpg", "jpeg", "png", "gif", "webp", "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "zip", "tar", "gz", "tgz", "txt", "mp4", "bin"})

	v.SetDefault("storage.type", "local")
	v.SetDefault("storage.local.url_prefix", "/static")
	v.SetDefault("storage.oss.direct_upload", true)
	v.SetDefault("storage.oss.verify_exists", true)
	v.SetDefault("storage.oss.sign_expire", 900)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.max_size_mb", 50)
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.max_age_days", 30)
	v.SetDefault("log.console", true)

	v.SetDefault("tenant.default_id", 1)

	v.SetDefault("metrics.enabled", true)

	v.SetDefault("swagger.enabled", false)
	v.SetDefault("swagger.title", "设备管理平台 API")
	v.SetDefault("swagger.desc", "设备管理平台开放接口文档")

	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.pool_size", 10)
	v.SetDefault("scheduler.shell_enabled", false)

	v.SetDefault("plugin.allow_docker", true)
	v.SetDefault("plugin.auto_start", true)
	v.SetDefault("plugin.install_timeout_sec", 300)
	v.SetDefault("plugin.log_tail_lines", 500)

	v.SetDefault("system.monitor_interval_sec", 10)
	v.SetDefault("system.metric_retain_hours", 168)
	v.SetDefault("system.tools_enabled", true)
	v.SetDefault("system.tools.ping", true)
	v.SetDefault("system.tools.traceroute", true)
	v.SetDefault("system.tools.tcpdump", true)
	v.SetDefault("system.tools.netstat", true)
	v.SetDefault("system.log_retain_days", 30)

	v.SetDefault("integration.enabled", false)
	v.SetDefault("integration.tenant_id", 1)
}
