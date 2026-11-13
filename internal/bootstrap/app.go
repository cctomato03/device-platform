// Package bootstrap 应用装配：配置 → 目录 → 日志 → 数据库 → 缓存 → 迁移 → 种子 → 路由 → HTTP 服务。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"device-platform/internal/api"
	"device-platform/internal/config"
	"device-platform/internal/database"
	"device-platform/internal/logger"
	"device-platform/internal/migrate"
	"device-platform/internal/paths"
	"device-platform/internal/plugin"
	"device-platform/internal/repository"
	"device-platform/internal/router"
	"device-platform/internal/scheduler"
	"device-platform/internal/service"
	"device-platform/internal/sysinfo"
	"device-platform/pkg/cache"
	"device-platform/pkg/captcha"
	"device-platform/pkg/id"
	"device-platform/pkg/jwt"
	"device-platform/pkg/storage"
)

// App 运行中的应用实例。
type App struct {
	Cfg    *config.Config
	Paths  *paths.Layout
	Svc    *service.Service
	Engine *gin.Engine
	server *http.Server
}

// Run 启动应用，ctx 取消时优雅关停。
//
// configPath 为空时使用 <home>/conf/config.yaml。
func Run(ctx context.Context, configPath, home string) error {
	cfg, err := config.Load(resolveConfigPath(configPath, home))
	if err != nil {
		return err
	}
	if home != "" {
		if err := cfg.ApplyHome(home); err != nil {
			return err
		}
	}

	// 统一数据目录：数据库/日志/插件/上传文件全部落在 home 下
	lay, err := paths.Resolve(cfg.App.Home)
	if err != nil {
		return err
	}
	cfg.FillPaths(lay)

	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logger.Sync()

	// 首次启动落一份配置文件，方便运维直接改
	if configPath == "" && !fileExists(lay.ConfigFile()) {
		if err := config.WriteDefault(lay.ConfigFile(), cfg); err != nil {
			logger.S().Warnw("写入默认配置失败", "err", err)
		}
	}

	logger.S().Infow("starting",
		"app", cfg.App.Name, "env", cfg.App.Env, "port", cfg.App.Port,
		"home", lay.Home, "db", cfg.Database.Path, "arch", archName())

	// 雪花 ID
	if err := id.Init(1); err != nil {
		return fmt.Errorf("初始化 ID 生成器失败: %w", err)
	}

	// SQLite
	db, err := database.OpenSQLite(cfg.Database, cfg.App.Timezone)
	if err != nil {
		return err
	}
	if err := migrate.Run(db); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	logger.S().Info("数据库迁移完成")

	// 进程内缓存（替代 Redis）
	c := cache.New("devplat:")
	defer c.Stop()

	// 存储
	st, err := storage.New(cfg.Storage)
	if err != nil {
		return fmt.Errorf("初始化文件存储失败: %w", err)
	}

	jwtMgr := jwt.New(cfg.JWT.Secret, cfg.JWT.SecretPrev, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.Issuer)
	captchaSvc := captcha.NewService(captcha.NewMemoryStore(2*time.Minute), cfg.Auth.CaptchaEnabled)

	repo := repository.New(db, c, cfg)
	pluginMgr := plugin.NewManager(cfg, lay, db)
	svc := service.New(&service.Deps{
		Repo:    repo,
		Cache:   c,
		Cfg:     cfg,
		Paths:   lay,
		JWT:     jwtMgr,
		Captcha: captchaSvc,
		Storage: st,
		Sys:     sysinfo.NewCollector(),
		Plugins: pluginMgr,
	})

	// 初始化超管账号
	if err := ensureAdmin(ctx, cfg, repo, svc); err != nil {
		return fmt.Errorf("初始化管理员失败: %w", err)
	}

	h := api.New(svc)
	engine := router.New(cfg, svc, h)

	// 定时任务调度器
	svc.RegisterJobHandlers()
	sch := scheduler.New(svc)
	if cfg.Scheduler.Enabled {
		sch.Start(ctx)
		defer sch.Stop()
		h.ReloadJobs = sch.Reload
	}

	// 设备平台后台任务：指标采集、告警判定、插件自动拉起
	svc.StartPlatformTasks(ctx)

	app := &App{Cfg: cfg, Paths: lay, Svc: svc, Engine: engine}
	app.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.S().Infof("HTTP 服务已启动: http://localhost:%d", cfg.App.Port)
		if err := app.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.S().Info("收到退出信号，开始优雅关停")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	svc.StopPlatformTasks()
	if err := app.server.Shutdown(shutdownCtx); err != nil {
		logger.S().Errorw("HTTP 服务关停异常", "err", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	logger.S().Info("已退出")
	return nil
}

func resolveConfigPath(configPath, home string) string {
	if configPath != "" {
		return configPath
	}
	if home != "" {
		return home + string(os.PathSeparator) + "conf" + string(os.PathSeparator) + "config.yaml"
	}
	if h := os.Getenv("DEVPLAT_HOME"); h != "" {
		return h + string(os.PathSeparator) + "conf" + string(os.PathSeparator) + "config.yaml"
	}
	return "conf/config.yaml"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
