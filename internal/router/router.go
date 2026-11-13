// Package router 路由注册：全局中间件 + 模块分组 + 权限码声明。
package router

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"device-platform/internal/api"
	"device-platform/internal/config"
	"device-platform/internal/metrics"
	"device-platform/internal/middleware"
	"device-platform/internal/openapi"
	"device-platform/internal/service"
	"device-platform/internal/webui"
	"device-platform/pkg/errs"
	"device-platform/pkg/response"
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
	authed := v1.Group("", middleware.Auth(svc), middleware.AccessLog(svc), middleware.OperLog(svc))
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

		// ==================== 设备平台：主机管理 ====================
		host := authed.Group("/host")
		host.GET("/overview", middleware.Perm("host:overview:view"), h.HostOverview)
		host.GET("/snapshot", middleware.Perm("host:hardware:view"), h.HostSnapshot)
		host.GET("/metrics", middleware.Perm("host:monitor:view"), h.HostMetricsHistory)
		host.GET("/traffic", middleware.Perm("host:traffic:view"), h.ListTrafficLogs)

		// ==================== 设备平台：设备管理 ====================
		dev := authed.Group("/device")
		dev.GET("/settings", middleware.Perm("device:setting:list"), h.GetDeviceSettings)
		dev.PUT("/settings", middleware.Perm("device:setting:update"), h.UpdateDeviceSettings)
		dev.GET("/time", middleware.Perm("device:setting:list"), h.GetDeviceTime)
		dev.POST("/time/sync", middleware.Perm("device:setting:update"), h.SyncDeviceTime)
		dev.GET("/about", middleware.Perm("device:about:view"), h.PlatformAbout)
		dev.GET("/netstatus", middleware.Perm("device:netstatus:view"), h.NetworkStatus)
		dev.GET("/alert/rules", middleware.Perm("device:alert:list"), h.ListAlertRules)
		dev.POST("/alert/rules", middleware.Perm("device:alert:create"), h.SaveAlertRule)
		dev.PUT("/alert/rules", middleware.Perm("device:alert:update"), h.SaveAlertRule)
		dev.DELETE("/alert/rules/:id", middleware.Perm("device:alert:delete"), h.DeleteAlertRule)
		dev.GET("/alert/records", middleware.Perm("device:alert:list"), h.ListAlertRecords)
		dev.PUT("/alert/records/:id/ack", middleware.Perm("device:alert:update"), h.AckAlertRecord)
		dev.GET("/tools/meta", middleware.Perm("device:tool:list"), h.ToolMeta)
		dev.POST("/tools/run", middleware.Perm("device:tool:run"), h.RunTool)
		dev.GET("/tools/stream", middleware.Perm("device:tool:run"), h.StreamTool)

		// SNMP 服务（net-snmp）
		dev.GET("/snmp", middleware.Perm("device:snmp:list"), h.GetSnmpConfig)
		dev.PUT("/snmp", middleware.Perm("device:snmp:update"), h.SaveSnmpConfig)
		dev.POST("/snmp/apply", middleware.Perm("device:snmp:apply"), h.ApplySnmpConfig)
		dev.POST("/snmp/install", middleware.Perm("device:snmp:install"), h.InstallSNMP)
		dev.GET("/snmp/status", middleware.Perm("device:snmp:list"), h.SnmpStatus)

		// ==================== 设备平台：网络管理 ====================
		net := authed.Group("/network")
		net.GET("/interfaces", middleware.Perm("network:iface:list"), h.ListInterfaces)
		net.GET("/status", middleware.Perm("network:iface:list"), h.NetworkStatus)
		net.GET("/routes", middleware.Perm("network:route:list"), h.ListStaticRoutes)
		net.POST("/routes", middleware.Perm("network:route:create"), h.SaveStaticRoute)
		net.PUT("/routes", middleware.Perm("network:route:update"), h.SaveStaticRoute)
		net.DELETE("/routes/:id", middleware.Perm("network:route:delete"), h.DeleteStaticRoute)
		net.POST("/routes/apply", middleware.Perm("network:route:apply"), h.ApplyStaticRoutes)
		net.GET("/dns", middleware.Perm("network:dns:list"), h.GetDNSConfig)
		net.PUT("/dns", middleware.Perm("network:dns:update"), h.UpdateDNSConfig)

		// 网卡配置（netplan：静态地址 / DHCP / MTU / 网桥 / 端口聚合）
		net.GET("/iface-configs", middleware.Perm("network:ifacecfg:list"), h.ListIfaceConfigs)
		net.POST("/iface-configs", middleware.Perm("network:ifacecfg:update"), h.SaveIfaceConfig)
		net.PUT("/iface-configs", middleware.Perm("network:ifacecfg:update"), h.SaveIfaceConfig)
		net.DELETE("/iface-configs/:id", middleware.Perm("network:ifacecfg:update"), h.DeleteIfaceConfig)
		net.POST("/iface-configs/apply", middleware.Perm("network:ifacecfg:apply"), h.ApplyIfaceConfigs)
		net.GET("/iface-configs/meta", middleware.Perm("network:ifacecfg:list"), h.NetplanStatus)
		net.GET("/iface-configs/preview", middleware.Perm("network:ifacecfg:list"), h.NetplanPreview)

		// 策略路由（ip rule + 路由表）
		net.GET("/policy-routes", middleware.Perm("network:policyroute:list"), h.ListPolicyRoutes)
		net.POST("/policy-routes", middleware.Perm("network:policyroute:create"), h.SavePolicyRoute)
		net.PUT("/policy-routes", middleware.Perm("network:policyroute:update"), h.SavePolicyRoute)
		net.DELETE("/policy-routes/:id", middleware.Perm("network:policyroute:delete"), h.DeletePolicyRoute)
		net.POST("/policy-routes/apply", middleware.Perm("network:policyroute:apply"), h.ApplyPolicyRoutes)

		// DHCP 服务（dnsmasq）
		net.GET("/dhcp", middleware.Perm("network:dhcp:list"), h.GetDhcpConfig)
		net.PUT("/dhcp", middleware.Perm("network:dhcp:update"), h.SaveDhcpConfig)
		net.POST("/dhcp/apply", middleware.Perm("network:dhcp:apply"), h.ApplyDhcpConfig)
		net.POST("/dhcp/install", middleware.Perm("network:dhcp:install"), h.InstallDnsmasq)

		// DNS 服务（dnsmasq）
		net.GET("/dns-server", middleware.Perm("network:dnsserver:list"), h.GetDnsServerConfig)
		net.PUT("/dns-server", middleware.Perm("network:dnsserver:update"), h.SaveDnsServerConfig)
		net.POST("/dns-server/apply", middleware.Perm("network:dnsserver:apply"), h.ApplyDnsServerConfig)
		net.POST("/dns-server/install", middleware.Perm("network:dnsserver:install"), h.InstallDnsmasq)
		net.GET("/dnsmasq/status", middleware.Perm("network:dhcp:list"), h.DnsmasqStatus)

		// 动态路由 BGP / OSPF（FRR）
		net.GET("/routing", middleware.Perm("network:routing:list"), h.ListRoutingConfigs)
		net.PUT("/routing", middleware.Perm("network:routing:update"), h.SaveRoutingConfig)
		net.POST("/routing/apply", middleware.Perm("network:routing:apply"), h.ApplyRoutingConfigs)
		net.POST("/routing/install", middleware.Perm("network:routing:install"), h.InstallFRR)
		net.GET("/routing/status", middleware.Perm("network:routing:list"), h.RoutingStatus)

		// ==================== 设备平台：安全策略 ====================
		sec := authed.Group("/security")
		sec.GET("/firewall", middleware.Perm("security:firewall:list"), h.FirewallStatus)
		sec.POST("/firewall/install", middleware.Perm("security:firewall:install"), h.InstallFirewalld)
		sec.GET("/policies", middleware.Perm("security:policy:list"), h.ListPolicies)
		sec.POST("/policies", middleware.Perm("security:policy:create"), h.SavePolicy)
		sec.PUT("/policies", middleware.Perm("security:policy:update"), h.SavePolicy)
		sec.DELETE("/policies/:id", middleware.Perm("security:policy:delete"), h.DeletePolicy)
		sec.POST("/policies/apply", middleware.Perm("security:policy:apply"), h.ApplyPolicies)
		sec.GET("/nat", middleware.Perm("security:nat:list"), h.ListNATRules)
		sec.POST("/nat", middleware.Perm("security:nat:create"), h.SaveNATRule)
		sec.PUT("/nat", middleware.Perm("security:nat:update"), h.SaveNATRule)
		sec.DELETE("/nat/:id", middleware.Perm("security:nat:delete"), h.DeleteNATRule)
		sec.POST("/nat/apply", middleware.Perm("security:nat:apply"), h.ApplyNATRules)
		sec.GET("/qos", middleware.Perm("security:qos:list"), h.ListQosRules)
		sec.POST("/qos", middleware.Perm("security:qos:create"), h.SaveQosRule)
		sec.PUT("/qos", middleware.Perm("security:qos:update"), h.SaveQosRule)
		sec.DELETE("/qos/:id", middleware.Perm("security:qos:delete"), h.DeleteQosRule)
		sec.POST("/qos/apply", middleware.Perm("security:qos:apply"), h.ApplyQosRules)

		// ==================== 设备平台：插件市场 ====================
		pl := authed.Group("/plugins")
		pl.GET("", middleware.Perm("plugin:market:list"), h.ListPlugins)
		pl.GET("/meta", middleware.Perm("plugin:market:list"), h.PluginRuntimeMeta)
		pl.GET("/bundled", middleware.Perm("plugin:market:list"), h.ListBundledPlugins)
		pl.POST("/bundled/:key/install", middleware.Perm("plugin:market:install"), h.InstallBundledPlugin)
		pl.POST("/install", middleware.Perm("plugin:market:install"), h.InstallPlugin)
		pl.GET("/:key", middleware.Perm("plugin:market:list"), h.GetPlugin)
		pl.GET("/:key/status", middleware.Perm("plugin:market:list"), h.PluginStatus)
		pl.GET("/:key/logs", middleware.Perm("plugin:log:view"), h.PluginLogs)
		pl.PUT("/:key/config", middleware.Perm("plugin:config:update"), h.UpdatePluginConfig)
		pl.DELETE("/:key", middleware.Perm("plugin:market:uninstall"), h.UninstallPlugin)
		pl.POST("/:key/:action", middleware.Perm("plugin:lifecycle:update"), h.PluginAction)

		// 在线插件市场（远程索引 + 签名校验 + 自动升级）
		pl.GET("/sources", middleware.Perm("plugin:source:list"), h.ListMarketSources)
		pl.POST("/sources", middleware.Perm("plugin:source:create"), h.SaveMarketSource)
		pl.PUT("/sources", middleware.Perm("plugin:source:update"), h.SaveMarketSource)
		pl.DELETE("/sources/:id", middleware.Perm("plugin:source:delete"), h.DeleteMarketSource)
		pl.GET("/market", middleware.Perm("plugin:market:list"), h.ListMarketPlugins)
		pl.POST("/market/install", middleware.Perm("plugin:market:install"), h.InstallMarketPlugin)

		// ==================== 设备平台：日志审计 ====================
		logs := authed.Group("/logs")
		logs.GET("/access", middleware.Perm("monitor:access-log:list"), h.ListAccessLogs)
		logs.GET("/traffic", middleware.Perm("monitor:traffic-log:list"), h.ListTrafficLogs)
		logs.GET("/system", middleware.Perm("monitor:system-log:list"), h.SystemLog)

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
			DB: map[string]float64{},
		}
		if sqlDB, err := svc.Repo.DB.DB(); err == nil {
			st := sqlDB.Stats()
			extras.DB["open"] = float64(st.OpenConnections)
			extras.DB["in_use"] = float64(st.InUse)
			extras.DB["idle"] = float64(st.Idle)
		}
		if svc.Cache.Ping(ctx) == nil {
			extras.Cache = 1
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
