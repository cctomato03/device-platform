package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// APIKey 开放接口鉴权：业务系统用 `X-API-KEY` 头对接，不需要用户登录。
//
// 使用场景：定时任务/外部系统把「消息」「待办」推进本系统。
// 安全约定：
//   - 未配置 integration.api_key 时接口直接 404（默认关闭，避免误暴露）；
//   - 常量时间比较防止时序侧信道；
//   - 上下文里的主体固定为「系统账号」：UserID=0、TenantID=配置的默认租户，
//     因此开放接口只能写「目标用户」的数据，不能冒用任何人的身份查数据。
func APIKey(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := svc.Cfg.Integration
		if !cfg.Enabled || strings.TrimSpace(cfg.APIKey) == "" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": errs.ErrNotFound.Code, "message": "接口未开放"})
			return
		}
		got := strings.TrimSpace(c.GetHeader("X-API-KEY"))
		if got == "" {
			got = strings.TrimSpace(c.Query("apiKey"))
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(cfg.APIKey)) != 1 {
			response.Fail(c, errs.ErrUnauthorized)
			c.Abort()
			return
		}
		principal.Set(c, &principal.Info{
			UserID:   0,
			TenantID: cfg.TenantID,
			Username: "system-integration",
			Nickname: "开放接口",
			IsSuper:  false,
		})
		c.Next()
	}
}
