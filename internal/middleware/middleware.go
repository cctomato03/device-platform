// Package middleware Gin 中间件：链路追踪、异常恢复、跨域、认证、鉴权、限流、操作日志。
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"device-platform/internal/audit"
	"device-platform/internal/logger"
	"device-platform/internal/metrics"
	"device-platform/internal/principal"
	"device-platform/internal/service"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
	"device-platform/pkg/jwt"
	"device-platform/pkg/response"
	"device-platform/pkg/version"
)

// Trace 生成/透传 traceId，并记录访问日志。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetHeader("X-Trace-Id")
		if tid == "" {
			tid = uuid.NewString()[:16]
		}
		c.Set("traceId", tid)
		c.Header("X-Trace-Id", tid)

		start := time.Now()
		c.Next()

		logger.L().Info("access",
			zap.String("traceId", tid),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("cost", time.Since(start)),
		)
	}
}

// Recovery 异常恢复：记录堆栈并返回标准 500。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				tid, _ := c.Get("traceId")
				logger.S().Errorw("panic recovered", "traceId", tid, "panic", r, "path", c.Request.URL.Path)
				response.FailCode(c, errs.ErrInternal.Code, errs.ErrInternal.Msg)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS 跨域处理（前后端分离部署时使用）。
func CORS(origins []string) gin.HandlerFunc {
	allowAll := len(origins) == 0
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || allowed[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Trace-Id,X-Tenant-Id,Idempotency-Key,Accept-Language")
			c.Header("Access-Control-Expose-Headers", "X-Trace-Id")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth 认证中间件：解析 access token → 加载主体 → 注入上下文。
func Auth(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			unauthorized(c, errs.ErrUnauthorized)
			return
		}
		claims, err := svc.JWT.Parse(token, jwt.TypeAccess)
		if err != nil {
			if svc.IsInvalidToken(err) {
				unauthorized(c, errs.ErrTokenExpired)
				return
			}
			unauthorized(c, errs.ErrTokenInvalid)
			return
		}
		if svc.IsBlacklisted(c.Request.Context(), claims.ID) {
			unauthorized(c, errs.ErrTokenExpired)
			return
		}
		if !svc.CheckSingleSession(c.Request.Context(), claims.UserID, claims.ID) {
			unauthorized(c, errs.ErrSessionKicked)
			return
		}
		p, err := svc.CurrentPrincipal(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		p.JTI = claims.ID
		principal.Set(c, p)
		c.Next()
	}
}

// Perm 接口级权限校验：权限码与前端按钮码同源。
func Perm(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal.Must(c)
		if !p.HasPerm(code) {
			response.Fail(c, errs.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimit 简单滑动窗口限流（Redis INCR），按 IP + 路径维度。
func RateLimit(svc *service.Service, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		bucket := time.Now().Unix() / int64(window.Seconds())
		key := "rate:" + c.ClientIP() + ":" + c.FullPath() + ":" + strconv.FormatInt(bucket, 10)
		n, err := svc.Cache.Incr(c.Request.Context(), key, window)
		if err == nil && n > int64(limit) {
			response.Fail(c, errs.ErrTooManyReqs)
			c.Abort()
			return
		}
		c.Next()
	}
}

// OperLog 操作日志：非 GET 请求异步写库（含请求体与耗时）。
func OperLog(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 读取请求体片段用于操作日志。
		// 注意：只能"预读前 4KB 后把剩余流接回去"，不能把 body 替换成这 4KB，
		// 否则大 JSON 请求体会被截断、multipart 上传会直接解析失败（表现为 10001 参数校验失败）。
		params := ""
		if c.Request.Body != nil && c.Request.Method != http.MethodGet {
			contentType := strings.ToLower(c.GetHeader("Content-Type"))
			if strings.HasPrefix(contentType, "multipart/form-data") {
				// 二进制上传不记录请求体，只记录类型与大小
				params = fmt.Sprintf("[multipart] %s size=%d", c.GetHeader("Content-Type"), c.Request.ContentLength)
			} else {
				head, err := io.ReadAll(io.LimitReader(c.Request.Body, 4096))
				if err == nil {
					if len(head) > 0 {
						params = maskSensitive(strings.TrimSpace(string(head)))
					}
					// 用 head + 原流剩余部分还原完整请求体
					c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(head), c.Request.Body))
				}
			}
		}

		// 挂载变更收集器：service 层调用 audit.Note 记录字段级差异，这里统一落库
		auditCtx, _ := audit.NewContext(c.Request.Context())
		c.Request = c.Request.WithContext(auditCtx)

		c.Next()

		if c.Request.Method == http.MethodGet {
			return
		}
		status := 1
		errMsg := ""
		// 失败判定：handler 里通过 response.Fail 写入了 gin 错误链，或 HTTP 状态码 >= 400
		if len(c.Errors) > 0 || c.Writer.Status() >= http.StatusBadRequest {
			status = 0
			errMsg = c.Errors.String()
		}
		// 字段级变更差异（仅成功的写操作才有意义）
		diffJSON := ""
		if status == 1 {
			if changes := audit.ChangesFromContext(c.Request.Context()); len(changes) > 0 {
				if b, err := json.Marshal(changes); err == nil {
					diffJSON = string(b)
				}
			}
		}
		p := principal.Must(c)
		tid, _ := c.Get("traceId")

		title := strings.TrimPrefix(c.FullPath(), "/api/v1")
		entry := map[string]any{
			"id":            id.Next(),
			"trace_id":      tid,
			"title":         title,
			"method":        c.HandlerName(),
			"request_uri":   c.Request.URL.RequestURI(),
			"http_method":   c.Request.Method,
			"oper_user_id":  p.UserID,
			"tenant_id":     p.TenantID,
			"oper_name":     p.Username,
			"dept_name":     p.DeptName,
			"oper_ip":       c.ClientIP(),
			"oper_location": "",
			"params":        params,
			"diff":          diffJSON,
			"result":        "",
			"status":        status,
			"error_msg":     truncate(errMsg, 2000),
			"cost_ms":       int(time.Since(start).Milliseconds()),
			"oper_time":     time.Now().Format("2006-01-02 15:04:05"),
		}
		ctx := context.WithoutCancel(c.Request.Context())
		go func() {
			if err := svc.Repo.DB.WithContext(ctx).Table("sys_oper_log").Create(entry).Error; err != nil {
				logger.S().Warnw("write oper log failed", "err", err)
			}
		}()
	}
}

// AccessLog 访问日志：记录所有 API 请求到 sys_access_log（设备端的「访问日志」页）。
//
// 与操作日志的区别：这里只关心「谁、什么时候、访问了哪个接口、耗时、状态码」，
// 不做字段级 diff；为了不拖慢接口，写入走异步 goroutine。
// 静态资源、健康检查、指标抓取不入库，避免日志洪水。
func AccessLog(svc *service.Service) gin.HandlerFunc {
	skipPrefixes := []string{"/static", "/healthz", "/readyz", "/metrics", "/assets", "/swagger"}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			return
		}
		for _, p := range skipPrefixes {
			if strings.HasPrefix(path, p) {
				return
			}
		}

		entry := map[string]any{
			"id":          id.Next(),
			"trace_id":    c.GetString("traceId"),
			"ip":          c.ClientIP(),
			"method":      c.Request.Method,
			"path":        path,
			"status":      c.Writer.Status(),
			"cost_ms":     int(time.Since(start).Milliseconds()),
			"bytes_out":   c.Writer.Size(),
			"user_agent":  truncate(c.GetHeader("User-Agent"), 300),
			"user_id":     principalUserID(c),
			"access_time": time.Now().Format("2006-01-02 15:04:05"),
		}
		ctx := context.WithoutCancel(c.Request.Context())
		go func() {
			if err := svc.Repo.DB.WithContext(ctx).Table("sys_access_log").Create(entry).Error; err != nil {
				logger.S().Debugw("write access log failed", "err", err)
			}
		}()
	}
}

// principalUserID 尽量取当前登录用户（未登录接口返回 0）。
func principalUserID(c *gin.Context) uint64 {
	if p, ok := principal.Get(c); ok && p != nil {
		return p.UserID
	}
	return 0
}

// Metrics 采集 HTTP 请求指标（路由用 gin FullPath，避免高基数）。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		metrics.Default.ObserveHTTP(c.Request.Method, route, strconv.Itoa(c.Writer.Status()), time.Since(start).Seconds())
	}
}

// SecurityHeaders 基础安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "no-referrer-when-downgrade")
		c.Next()
	}
}

// BuildVersion 构建版本号。
func BuildVersion() string { return version.Version }

// BuildCommit 构建提交号。
func BuildCommit() string { return version.Commit }

// BuildTime 构建时间。
func BuildTime() string { return version.BuildTime }

// SetBuildInfo 注入 -ldflags 写入的版本信息（main 启动时调用一次）。
func SetBuildInfo(v, commit, buildTime string) { version.Set(v, commit, buildTime) }

// Health 提供健康检查与就绪检查所需依赖。
type Health struct {
	Svc  *service.Service
	Boot time.Time
}

// Register 挂载健康检查路由。
func (h *Health) Register(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "uptime": int64(time.Since(h.Boot).Seconds())})
	})
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":   version.Version,
			"commit":    version.Commit,
			"buildTime": version.BuildTime,
			"uptime":    int64(time.Since(h.Boot).Seconds()),
		})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx := c.Request.Context()
		mysqlOK := false
		if sqlDB, err := h.Svc.Repo.DB.DB(); err == nil {
			mysqlOK = sqlDB.PingContext(ctx) == nil
		}
		redisOK := h.Svc.Cache.Ping(ctx) == nil
		code := http.StatusOK
		if !mysqlOK || !redisOK {
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, gin.H{"database": mysqlOK, "cache": redisOK})
	})
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if t := c.Query("token"); t != "" {
		return t
	}
	return ""
}

func unauthorized(c *gin.Context, e *errs.AppError) {
	response.Fail(c, e)
	c.Abort()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// sensitiveKeys 操作日志里必须脱敏的字段（明文密码/令牌绝不能落库）。
var sensitiveKeys = map[string]struct{}{
	"password": {}, "oldpassword": {}, "newpassword": {}, "passwd": {},
	"accesstoken": {}, "refreshtoken": {}, "token": {}, "secret": {},
	"secretkey": {}, "appsecret": {}, "privatekey": {},
}

// maskSensitive 对请求体做脱敏：JSON 递归替换敏感字段值为 ***；表单/文本做正则替换。
func maskSensitive(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			maskValue(v)
			if b, err := json.Marshal(v); err == nil {
				return truncate(string(b), 2000)
			}
		}
		return truncate(raw, 2000)
	}
	// 表单编码 / 其它文本：password=xxx、token=xxx 一并抹掉
	masked := formSecretRe.ReplaceAllString(raw, "$1=***")
	return truncate(masked, 2000)
}

// formSecretRe 匹配 password=xxx / token=xxx 这类键值对。
var formSecretRe = regexp.MustCompile(`(?i)\b(password|oldPassword|newPassword|passwd|accessToken|refreshToken|token|secret)=[^&\s]*`)

// maskValue 递归脱敏 map/数组中的敏感字段。
func maskValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if _, hit := sensitiveKeys[strings.ToLower(k)]; hit {
				t[k] = "***"
				continue
			}
			maskValue(val)
		}
	case []any:
		for _, item := range t {
			maskValue(item)
		}
	}
}
