// Package bootstrap 应用装配：配置 → 日志 → 数据库 → 缓存 → 迁移 → 种子 → 路由 → HTTP 服务。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"web-basic/internal/api"
	"web-basic/internal/config"
	"web-basic/internal/database"
	"web-basic/internal/logger"
	"web-basic/internal/migrate"
	"web-basic/internal/repository"
	"web-basic/internal/router"
	"web-basic/internal/scheduler"
	"web-basic/internal/service"
	"web-basic/pkg/cache"
	"web-basic/pkg/captcha"
	"web-basic/pkg/id"
	"web-basic/pkg/jwt"
	"web-basic/pkg/storage"
)

// App 运行中的应用实例。
type App struct {
	Cfg     *config.Config
	Svc     *service.Service
	Engine  *gin.Engine
	server  *http.Server
	closeFn []func()
}

// Run 启动应用，ctx 取消时优雅关停。
func Run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logger.Sync()

	logger.S().Infow("starting", "app", cfg.App.Name, "env", cfg.App.Env, "port", cfg.App.Port)

	// 雪花 ID
	if err := id.Init(1); err != nil {
		return fmt.Errorf("初始化 ID 生成器失败: %w", err)
	}

	// MySQL
	db, err := database.OpenMySQL(cfg.MySQL)
	if err != nil {
		return err
	}
	if cfg.MySQL.AutoMigrate {
		if err := migrate.Run(db); err != nil {
			return fmt.Errorf("数据库迁移失败: %w", err)
		}
		logger.S().Info("数据库迁移完成")
	}

	// Redis
	rdb, err := database.OpenRedis(cfg.Redis)
	if err != nil {
		return err
	}
	c := cache.New(rdb, cfg.Redis.Prefix)

	// 存储
	st, err := storage.New(cfg.Storage)
	if err != nil {
		return fmt.Errorf("初始化文件存储失败: %w", err)
	}

	// 基础设施服务
	jwtMgr := jwt.New(cfg.JWT.Secret, cfg.JWT.SecretPrev, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.Issuer)
	captchaSvc := captcha.NewService(
		captcha.NewRedisStore(rdb, cfg.Redis.Prefix, 2*time.Minute),
		cfg.Auth.CaptchaEnabled,
	)

	repo := repository.New(db, c, cfg)
	svc := service.New(&service.Deps{
		Repo:    repo,
		Cache:   c,
		Cfg:     cfg,
		JWT:     jwtMgr,
		Captcha: captchaSvc,
		Storage: st,
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

	app := &App{Cfg: cfg, Svc: svc, Engine: engine}
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
	if err := app.server.Shutdown(shutdownCtx); err != nil {
		logger.S().Errorw("HTTP 服务关停异常", "err", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	_ = rdb.Close()

	logger.S().Info("已退出")
	return nil
}
