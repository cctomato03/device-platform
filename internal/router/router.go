// Package router 路由注册：全局中间件 + 模块分组 + 权限码声明。
package router

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"web-basic/internal/api"
	"web-basic/internal/config"
	"web-basic/internal/metrics"
	"web-basic/internal/middleware"
	"web-basic/internal/openapi"
	"web-basic/internal/service"
	"web-basic/internal/webui"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// New 构建 Gin 引擎。
func New(cfg *config.Config, svc *service.Service, h *api.Handler) *gin.Engine {
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		middleware.Trace(),
		middleware.Recovery(),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.App.CorsOrigins),
		middleware.Metrics(),
	)

	// 健康检查（免认证，供探针使用）
	health := &middleware.Health{Svc: svc, Boot: time.Now()}
	health.Register(r)

	// 在线接口文档：/swagger/index.html（页面） + /swagger/openapi.json（规范）
	if cfg.Swagger.Enabled {
		index, spec := openapi.Handler(r.Routes, openapi.Info{
			Title:       cfg.Swagger.Title,
			Description: cfg.Swagger.Desc,
			Version:     middleware.BuildVersion(),
			BasePath:    "/api/v1",
		})
		r.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusFound, "/swagger/index.html") })
		r.GET("/swagger/index.html", index)
		r.GET("/swagger/openapi.json", spec)
	}

	// Prometheus 指标（默认开启；配置 token 后需带 X-Metrics-Token 头或 ?token=）
	if cfg.Metrics.Enabled {
		r.GET("/metrics", metricsHandler(svc, cfg))
	}

	// 本地存储文件直出（OSS 模式由 bucket 域名/CDN 提供，不需此路由）
	if cfg.Storage.Type == "local" {
		dir := cfg.Storage.Local.BaseDir
		prefix := cfg.Storage.Local.URLPrefix
		if prefix == "" {
			prefix = "/static"
		}
		if err := os.MkdirAll(dir, 0o755); err == nil {
			r.Static(prefix, dir)
		}
	}

	v1 := r.Group("/api/v1")

	// ---------- 公开接口（限流保护） ----------
	pub := v1.Group("", middleware.RateLimit(svc, 30, time.Minute))
	{
		pub.GET("/auth/captcha", h.Captcha)
		pub.POST("/auth/login", h.Login)
		pub.POST("/auth/refresh", h.Refresh)
		pub.GET("/system/configs/public", h.PublicConfig)
		pub.GET("/public/site-config", h.GetSiteConfig)
		pub.GET("/public/password-policy", h.PasswordPolicy) // 密码规则：注册/改密前展示，避免用户盲试 // 站点配置：登录页渲染前就要拿到（名称/logo/背景/注册开关）
	}

	// ---------- 需登录接口 ----------
	authed := v1.Group("", middleware.Auth(svc), middleware.OperLog(svc))
	{
		authed.POST("/auth/logout", h.Logout)
		authed.GET("/auth/userinfo", h.UserInfo)
		authed.GET("/auth/routes", h.UserRoutes) // 前端动态路由表（Web Admin Pro 后端权限模式）
		authed.GET("/dicts/:code", h.DictData)   // 字典为高频只读接口，登录即可读

		// 站点个性化配置
		authed.GET("/system/site-config", middleware.Perm("system:site:list"), h.GetSiteConfig)
		authed.PUT("/system/site-config", middleware.Perm("system:site:update"), h.UpdateSiteConfig)

		authed.GET("/profile", h.MyProfile)
		authed.PUT("/profile", h.UpdateMyProfile)
		authed.PUT("/profile/password", h.ChangePassword)

		// 工作中心（每个用户都有：待办 / 消息 / 通知）
		my := authed.Group("/my")
		my.GET("/summary", h.MyWorkbenchSummary)
		my.GET("/todos", h.ListMyTodos)
		my.GET("/todos/summary", h.MyTodoSummary)
		my.POST("/todos", h.CreateMyTodo)
		my.PUT("/todos/:id/status", h.UpdateMyTodoStatus)
		my.DELETE("/todos/:id", h.DeleteMyTodo)
		my.GET("/notices", h.ListMyNotices)
		my.POST("/notices/:id/read", h.MarkNoticeRead)

		// 用户
		u := authed.Group("/system/users")
		u.GET("", middleware.Perm("system:user:list"), h.ListUsers)
		u.GET("/:id", middleware.Perm("system:user:list"), h.GetUser)
		u.POST("", middleware.Perm("system:user:create"), h.CreateUser)
		u.PUT("/:id", middleware.Perm("system:user:update"), h.UpdateUser)
		u.DELETE("", middleware.Perm("system:user:delete"), h.DeleteUsers)
		u.PUT("/:id/status", middleware.Perm("system:user:update"), h.ChangeUserStatus)
		u.PUT("/:id/reset-password", middleware.Perm("system:user:reset-pwd"), h.ResetUserPassword)
		u.PUT("/:id/roles", middleware.Perm("system:user:assign"), h.AssignUserRoles)

		// 角色
		ro := authed.Group("/system/roles")
		ro.GET("", middleware.Perm("system:role:list"), h.ListRoles)
		ro.GET("/all", middleware.Perm("system:role:list"), h.AllRoles)
		ro.POST("", middleware.Perm("system:role:create"), h.CreateRole)
		ro.PUT("/:id", middleware.Perm("system:role:update"), h.UpdateRole)
		ro.DELETE("/:id", middleware.Perm("system:role:delete"), h.DeleteRole)
		ro.GET("/:id/granted", middleware.Perm("system:role:list"), h.RoleGranted)
		ro.PUT("/:id/grant", middleware.Perm("system:role:grant"), h.GrantRole)

		// 菜单
		m := authed.Group("/system/menus")
		m.GET("/tree", middleware.Perm("system:menu:list"), h.MenuTree)
		m.POST("", middleware.Perm("system:menu:create"), h.CreateMenu)
		m.PUT("/:id", middleware.Perm("system:menu:update"), h.UpdateMenu)
		m.DELETE("/:id", middleware.Perm("system:menu:delete"), h.DeleteMenu)

		// 部门
		d := authed.Group("/system/depts")
		d.GET("/tree", middleware.Perm("system:dept:list"), h.DeptTree)
		d.GET("/detail/:id", middleware.Perm("system:dept:list"), h.DeptDetail)
		d.POST("", middleware.Perm("system:dept:create"), h.CreateDept)
		d.PUT("/:id", middleware.Perm("system:dept:update"), h.UpdateDept)
		d.DELETE("/:id", middleware.Perm("system:dept:delete"), h.DeleteDept)

		// 岗位
		p := authed.Group("/system/posts")
		p.GET("", middleware.Perm("system:post:list"), h.ListPosts)
		p.POST("", middleware.Perm("system:post:create"), h.CreatePost)
		p.PUT("/:id", middleware.Perm("system:post:update"), h.UpdatePost)
		p.DELETE("/:id", middleware.Perm("system:post:delete"), h.DeletePost)

		// 字典
		dd := authed.Group("/system/dict")
		dd.GET("/types", middleware.Perm("system:dict:list"), h.ListDictTypes)
		dd.POST("/types", middleware.Perm("system:dict:create"), h.CreateDictType)
		dd.PUT("/types/:id", middleware.Perm("system:dict:update"), h.UpdateDictType)
		dd.DELETE("/types/:id", middleware.Perm("system:dict:delete"), h.DeleteDictType)
		dd.POST("/data", middleware.Perm("system:dict:create"), h.CreateDictData)
		dd.PUT("/data/:id", middleware.Perm("system:dict:update"), h.UpdateDictData)
		dd.DELETE("/data/:id", middleware.Perm("system:dict:delete"), h.DeleteDictData)

		// 参数
		cf := authed.Group("/system/configs")
		cf.GET("", middleware.Perm("system:config:list"), h.ListConfigs)
		cf.POST("", middleware.Perm("system:config:create"), h.CreateConfig)
		cf.PUT("/:id", middleware.Perm("system:config:update"), h.UpdateConfig)
		cf.DELETE("/:id", middleware.Perm("system:config:delete"), h.DeleteConfig)

		// 租户
		tn := authed.Group("/system/tenants")
		tn.GET("", middleware.Perm("system:tenant:list"), h.ListTenants)
		tn.POST("", middleware.Perm("system:tenant:create"), h.CreateTenant)
		tn.PUT("/:id", middleware.Perm("system:tenant:update"), h.UpdateTenant)
		tn.DELETE("/:id", middleware.Perm("system:tenant:delete"), h.DeleteTenant)

		// 代码生成器
		gen := authed.Group("/tool/gen")
		gen.GET("/tables", middleware.Perm("tool:gen:list"), h.GenTables)
		gen.GET("/tables/:table", middleware.Perm("tool:gen:list"), h.GenTableDetail)
		gen.GET("/tables/:table/preview", middleware.Perm("tool:gen:preview"), h.GenPreview)
		gen.POST("/download", middleware.Perm("tool:gen:download"), h.GenDownload)

		// Excel 导出 / 导入
		u.GET("/export", middleware.Perm("system:user:export"), h.ExportUsers)
		u.GET("/import-template", middleware.Perm("system:user:import"), h.UserImportTemplate)
		u.POST("/import", middleware.Perm("system:user:import"), h.ImportUsers)

		// 通知公告
		nt := authed.Group("/system/notices")
		nt.GET("", middleware.Perm("system:notice:list"), h.ListNotices)
		nt.GET("/:id", middleware.Perm("system:notice:list"), h.GetNotice)
		nt.POST("", middleware.Perm("system:notice:create"), h.CreateNotice)
		nt.PUT("/:id", middleware.Perm("system:notice:update"), h.UpdateNotice)
		nt.DELETE("/:id", middleware.Perm("system:notice:delete"), h.DeleteNotice)
		nt.POST("/:id/publish", middleware.Perm("system:notice:publish"), h.PublishNotice)

		// 站内信（当前登录用户自己的，登录即可用，无需额外权限码）
		// 消息与待办推送（后台手工发起；开放接口见下方 open 分组）
		push := authed.Group("/system/push")
		push.POST("/messages", middleware.Perm("system:push:create"), h.PushMessage)
		push.POST("/todos", middleware.Perm("system:push:create"), h.PushTodo)

		msg := authed.Group("/messages")
		msg.GET("", h.ListMyMessages)
		msg.GET("/unread-count", h.UnreadMessageCount)
		msg.PUT("/:id/read", h.MarkMessageRead)
		msg.PUT("/read-all", h.MarkAllMessagesRead)
		msg.DELETE("/:id", h.DeleteMyMessage)

		// 文件
		f := authed.Group("/files")
		f.POST("/upload", middleware.Perm("system:file:upload"), h.UploadFile)
		// OSS 直传：签名 + 回执（存储为本地磁盘时签名接口会返回「不支持」，前端自动回退到 /files/upload）
		f.POST("/oss-ticket", middleware.Perm("system:file:upload"), h.OSSUploadTicket)
		f.POST("/oss-record", middleware.Perm("system:file:upload"), h.OSSUploadRecord)
		f.GET("", middleware.Perm("system:file:list"), h.ListFiles)
		f.DELETE("/:id", middleware.Perm("system:file:delete"), h.DeleteFile)
		f.GET("/:id/url", h.FileURL)

		// 监控
		mon := authed.Group("/monitor")
		mon.GET("/login-logs", middleware.Perm("monitor:login-log:list"), h.ListLoginLogs)
		mon.GET("/oper-logs", middleware.Perm("monitor:oper-log:list"), h.ListOperLogs)
		mon.GET("/oper-logs/export", middleware.Perm("monitor:oper-log:export"), h.ExportOperLogs)
		mon.GET("/online", middleware.Perm("monitor:online:list"), h.OnlineUsers)
		mon.DELETE("/online", middleware.Perm("monitor:online:list"), h.KickUser)
		mon.GET("/server", middleware.Perm("monitor:server:view"), h.ServerInfo)

		// 首页概览
		authed.GET("/dashboard/summary", h.DashboardSummary)

		// 定时任务
		j := authed.Group("/system/jobs")
		j.GET("", middleware.Perm("system:job:list"), h.ListJobs)
		j.GET("/meta", middleware.Perm("system:job:list"), h.JobMeta)
		j.POST("", middleware.Perm("system:job:create"), h.CreateJob)
		j.PUT("/:id", middleware.Perm("system:job:update"), h.UpdateJob)
		j.PUT("/:id/status", middleware.Perm("system:job:update"), h.SetJobStatus)
		j.POST("/:id/run", middleware.Perm("system:job:run"), h.RunJob)
		j.DELETE("/:id", middleware.Perm("system:job:delete"), h.DeleteJob)
		j.GET("/logs", middleware.Perm("system:job:list"), h.ListJobLogs)
	}

	// ---------- 开放接口（业务系统对接：X-API-KEY，未配置时整体 404） ----------
	open := v1.Group("/open", middleware.APIKey(svc))
	{
		open.POST("/messages", h.OpenPushMessage)
		open.POST("/todos", h.OpenPushTodo)
	}

	// 未知 API 统一 404
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			response.Fail(c, errs.ErrNotFound)
			return
		}
		serveSPA(c)
	})

	return r
}

// metricsHandler 输出 Prometheus 文本格式指标（含 MySQL / Redis 探针）。
func metricsHandler(svc *service.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Metrics.Token != "" {
			token := c.GetHeader("X-Metrics-Token")
			if token == "" {
				token = c.Query("token")
			}
			if token != cfg.Metrics.Token {
				c.String(http.StatusUnauthorized, "metrics token required")
				return
			}
		}
		ctx := c.Request.Context()
		extras := metrics.Extras{
			Build: map[string]string{
				"version":   middleware.BuildVersion(),
				"commit":    middleware.BuildCommit(),
				"buildTime": middleware.BuildTime(),
			},
			MySQL: map[string]float64{},
		}
		if sqlDB, err := svc.Repo.DB.DB(); err == nil {
			st := sqlDB.Stats()
			extras.MySQL["open"] = float64(st.OpenConnections)
			extras.MySQL["in_use"] = float64(st.InUse)
			extras.MySQL["idle"] = float64(st.Idle)
		}
		if svc.Cache.Ping(ctx) == nil {
			extras.Redis = 1
		}
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics.Default.Render(c.Writer, extras)
	}
}

// serveSPA 托管前端产物并支持 history 路由回退（前后端分离时此段不会命中）。
func serveSPA(c *gin.Context) {
	static, err := webui.Dist()
	if err != nil {
		c.String(http.StatusNotFound, "前端产物未就绪：请构建 web/dist 或使用前后端分离部署")
		return
	}
	p := strings.TrimPrefix(c.Request.URL.Path, "/")
	if p != "" {
		if f, err := static.Open(p); err == nil {
			defer f.Close()
			http.FileServer(http.FS(static)).ServeHTTP(c.Writer, c.Request)
			return
		}
	}
	index, err := fs.ReadFile(static, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "index.html 不存在")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", index)
}
