// Package openapi 生成 OpenAPI 3.0 描述并托管一个离线可用的文档页。
//
// 为什么不用 swaggo：swag 需要在每个 handler 上堆注释并额外依赖生成步骤，
// 而脚手架更看重"零额外步骤、离线可用"。这里直接遍历 Gin 已注册路由，
// 结合路径语义生成结构化描述（分组 tag、中文摘要、Bearer 鉴权、统一错误响应），
// 足以支撑前后端联调与第三方接入；需要更精细的字段级 schema 时可在此按路径补充。
package openapi

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed index.html
var indexHTML []byte

// IndexHTML 返回离线文档页（无任何 CDN 依赖）。
func IndexHTML() []byte { return indexHTML }

// Handler 返回文档页与 spec 的 HTTP handler。
func Handler(routes func() gin.RoutesInfo, info Info) (index gin.HandlerFunc, spec gin.HandlerFunc) {
	index = func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}
	spec = func(c *gin.Context) {
		data, err := Spec(routes(), info)
		if err != nil {
			c.String(http.StatusInternalServerError, "build openapi spec failed: %v", err)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
	return index, spec
}

// Info 构建 spec 需要的元信息。
type Info struct {
	Title       string
	Description string
	Version     string
	BasePath    string
}

type tagObj struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type opObj struct {
	Tags        []string              `json:"tags"`
	Summary     string                `json:"summary"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId"`
	Parameters  []map[string]any      `json:"parameters,omitempty"`
	Responses   map[string]any        `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

type pathItem map[string]*opObj

// Spec 由路由表构建 OpenAPI 文档。
func Spec(routes gin.RoutesInfo, info Info) ([]byte, error) {
	paths := map[string]pathItem{}
	tagSet := map[string]bool{}

	for _, rt := range routes {
		if !strings.HasPrefix(rt.Path, info.BasePath) {
			continue // 只收录业务接口，排除 /metrics、/healthz 等运维端点
		}
		path := toOpenAPIPath(rt.Path)
		method := strings.ToLower(rt.Method)
		op := buildOp(rt, path)
		tagSet[op.Tags[0]] = true
		if paths[path] == nil {
			paths[path] = pathItem{}
		}
		paths[path][method] = op
	}

	tags := make([]tagObj, 0, len(tagSet))
	for name := range tagSet {
		tags = append(tags, tagObj{Name: name, Description: tagDesc(name)})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })

	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       info.Title,
			"description": info.Description,
			"version":     info.Version,
		},
		"servers": []map[string]any{{"url": "/", "description": "当前服务"}},
		"tags":    tags,
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "登录接口返回的 accessToken",
				},
			},
			"schemas": map[string]any{
				"Resp": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "integer", "description": "0 表示成功"},
						"message": map[string]any{"type": "string"},
						"data":    map[string]any{"description": "业务数据"},
						"traceId": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	return json.MarshalIndent(spec, "", "  ")
}

// toOpenAPIPath /api/v1/system/users/:id → /api/v1/system/users/{id}
func toOpenAPIPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if strings.HasPrefix(s, ":") {
			parts[i] = "{" + strings.TrimPrefix(s, ":") + "}"
		} else if strings.HasPrefix(s, "*") {
			parts[i] = "{" + strings.TrimPrefix(s, "*") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func buildOp(rt gin.RouteInfo, path string) *opObj {
	tag := tagOf(rt.Path)
	op := &opObj{
		Tags:        []string{tag},
		Summary:     summaryOf(rt.Method, rt.Path),
		OperationID: operationID(rt.Method, rt.Path),
		Responses: map[string]any{
			"200": map[string]any{
				"description": "统一响应体",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/Resp"},
					},
				},
			},
			"401": map[string]any{"description": "未登录 / token 失效（业务码 20001）"},
			"403": map[string]any{"description": "无权限（业务码 30001）"},
		},
	}
	// 非白名单接口都需要 Bearer
	if !publicPath(rt.Path) {
		op.Security = []map[string][]string{{"bearerAuth": {}}}
	}
	// 路径参数
	for _, seg := range strings.Split(rt.Path, "/") {
		if strings.HasPrefix(seg, ":") {
			name := strings.TrimPrefix(seg, ":")
			op.Parameters = append(op.Parameters, map[string]any{
				"name":     name,
				"in":       "path",
				"required": true,
				"schema":   map[string]any{"type": "string"},
			})
		}
	}
	// 分页类查询参数
	if rt.Method == "GET" || strings.HasSuffix(rt.Path, ":id") {
		// no-op
	}
	if rt.Method == "GET" && !strings.Contains(rt.Path, ":") {
		op.Parameters = append(op.Parameters,
			map[string]any{"name": "page", "in": "query", "schema": map[string]any{"type": "integer"}, "description": "页码（部分接口无分页）"},
			map[string]any{"name": "pageSize", "in": "query", "schema": map[string]any{"type": "integer"}, "description": "每页条数"},
		)
	}
	if rt.Method == "POST" || rt.Method == "PUT" {
		op.Description = "请求体为 JSON（文件上传类接口为 multipart/form-data）"
	}
	return op
}

// publicPath 免鉴权接口。
func publicPath(p string) bool {
	for _, prefix := range []string{"/api/v1/auth/captcha", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/public"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// tagOf 取路径中的资源段作为分组名（/api/v1/system/users → 系统设置·用户）。
func tagOf(p string) string {
	segs := strings.Split(strings.TrimPrefix(p, "/api/v1/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return "其他"
	}
	// /system/notices/:id/publish → 系统设置·notices；/auth/login → 认证·auth
	resource := segs[0]
	if len(segs) >= 2 && (segs[0] == "system" || segs[0] == "monitor" || segs[0] == "tool") {
		resource = segs[1]
	}
	return moduleName(segs[0]) + "·" + resource
}

func moduleName(seg string) string {
	switch seg {
	case "auth":
		return "认证"
	case "system":
		return "系统设置"
	case "monitor":
		return "系统监控"
	case "tool":
		return "开发工具"
	case "public":
		return "公开接口"
	default:
		return seg
	}
}

func tagDesc(name string) string {
	switch {
	case strings.HasPrefix(name, "认证"):
		return "登录、登出、验证码、令牌刷新"
	case strings.HasPrefix(name, "系统设置"):
		return "用户 / 角色 / 菜单 / 部门 / 岗位 / 字典 / 参数 / 公告 等管理接口"
	case strings.HasPrefix(name, "系统监控"):
		return "在线用户、服务状态、登录日志、操作日志"
	case strings.HasPrefix(name, "开发工具"):
		return "代码生成器等开发辅助接口"
	default:
		return ""
	}
}

// summaryOf 生成中文摘要（按 HTTP 方法 + 路径动词推断）。
func summaryOf(method, p string) string {
	action := ""
	segs := strings.Split(strings.Trim(p, "/"), "/")
	last := segs[len(segs)-1]
	if strings.HasPrefix(last, ":") && len(segs) >= 2 {
		last = segs[len(segs)-2]
	}
	switch last {
	case "list", "tree":
		action = "查询"
	case "export":
		action = "导出"
	case "import":
		action = "导入"
	case "import-template":
		action = "下载导入模板"
	case "publish":
		action = "发布"
	case "status":
		action = "变更状态"
	case "reset-pwd":
		action = "重置密码"
	case "assign":
		action = "分配"
	case "unread-count":
		action = "未读数量"
	case "read", "read-all":
		action = "标记已读"
	case "preview":
		action = "预览"
	case "download":
		action = "下载"
	case "captcha":
		action = "获取图形验证码"
	case "login":
		action = "登录"
	case "logout":
		action = "登出"
	case "refresh":
		action = "刷新令牌"
	case "profile":
		action = "当前用户资料"
	}
	if action == "" {
		switch method {
		case "GET":
			action = "查询"
		case "POST":
			action = "新增"
		case "PUT", "PATCH":
			action = "修改"
		case "DELETE":
			action = "删除"
		default:
			action = "操作"
		}
	}
	// 中段动词补充：/:id/xxx
	if len(segs) >= 2 {
		prev := segs[len(segs)-2]
		if strings.HasPrefix(last, ":") && prev != "" {
			// /xxx/:id 形式
		}
	}
	return action + resourceName(p)
}

// resourceNames 资源段 → 中文名（用于生成可读摘要）。
var resourceNames = map[string]string{
	"auth": "认证", "users": "用户", "roles": "角色", "menus": "菜单", "depts": "部门",
	"posts": "岗位", "dict": "字典", "configs": "参数", "tenants": "租户", "files": "文件",
	"jobs": "定时任务", "notices": "通知公告", "messages": "站内信", "profile": "个人资料",
	"password": "密码", "routes": "路由", "buttons": "按钮权限", "oper-logs": "操作日志",
	"login-logs": "登录日志", "online": "在线用户", "server": "服务状态", "gen": "代码生成",
	"dashboard": "仪表盘", "logs": "任务日志", "status": "状态", "publish": "发布",
	"export": "导出", "import": "导入", "captcha": "验证码", "login": "登录", "logout": "登出",
	"refresh": "令牌刷新", "options": "下拉选项", "stats": "统计",
}

// resourceName 从路径里挑出最具业务含义的段并翻译。
func resourceName(p string) string {
	segs := strings.Split(strings.TrimPrefix(p, "/api/v1/"), "/")
	for _, s := range segs {
		if name, ok := resourceNames[s]; ok {
			return name
		}
	}
	return strings.ReplaceAll(strings.TrimPrefix(p, "/api/v1/"), "/", " ")
}

// operationID 唯一标识：方法 + 路径。
func operationID(method, p string) string {
	s := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(p, "/api/v1/"), "/", "_"))
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToLower(method) + "_" + s
}
