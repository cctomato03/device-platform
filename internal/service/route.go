package service

import (
	"context"
	"sort"
	"strings"

	"device-platform/internal/model/entity"
	"device-platform/internal/principal"
	"device-platform/pkg/errs"
)

// LayoutComponent 前端布局容器组件路径（与前端 router/routesAlias.ts 中 RoutesAlias.Layout 保持一致）。
const LayoutComponent = "/index/index"

// RouteAuth 路由上的按钮权限标识，前端 v-auth 指令据此控制按钮显隐。
type RouteAuth struct {
	Title    string `json:"title"`
	AuthMark string `json:"authMark"`
}

// RouteMeta 前端路由元信息（字段命名对齐 Web Admin Pro 约定）。
type RouteMeta struct {
	Title string `json:"title"`
	// TitleEn 英文标题（菜单表 name_en）；前端按当前语言二选一，未配置时回落中文
	TitleEn   string      `json:"titleEn,omitempty"`
	Icon      string      `json:"icon,omitempty"`
	KeepAlive bool        `json:"keepAlive,omitempty"`
	IsHide    bool        `json:"isHide,omitempty"`
	IsHideTab bool        `json:"isHideTab,omitempty"`
	FixedTab  bool        `json:"fixedTab,omitempty"`
	Link      string      `json:"link,omitempty"`
	IsIframe  bool        `json:"isIframe,omitempty"`
	AuthList  []RouteAuth `json:"authList,omitempty"`

	// 以下为 device-platform 扩展字段：让前端「菜单管理」页可直接复用同一份数据
	Type       int    `json:"type"`
	Permission string `json:"permission,omitempty"`
	Sort       int    `json:"sort,omitempty"`
}

// RouteNode 前端动态路由节点。
type RouteNode struct {
	ID        uint64       `json:"id,string"`
	ParentID  uint64       `json:"parentId,string"`
	Path      string       `json:"path"`
	Name      string       `json:"name"`
	Component string       `json:"component"`
	Redirect  string       `json:"redirect,omitempty"`
	Meta      RouteMeta    `json:"meta"`
	Children  []*RouteNode `json:"children"`
}

// UserRoutes 生成当前登录用户可访问的前端路由树（目录/菜单 + 按钮权限）。
//
// 与 UserMenuTree 的区别：
//   - 输出前端路由契约（component 为视图路径、meta 携带标题/图标/缓存/按钮权限）
//   - 目录菜单的 component 固定为布局容器，子菜单 path 保持相对路径，由前端规范化
func (s *Service) UserRoutes(ctx context.Context, p *principal.Info) ([]*RouteNode, error) {
	var (
		list []entity.Menu
		err  error
	)
	if p.IsSuper {
		list, err = s.Repo.AllMenus(ctx, 0)
	} else {
		list, err = s.Repo.MenusOfUser(ctx, p.UserID)
	}
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}

	grouped := make(map[uint64][]entity.Menu, len(list))
	for _, m := range list {
		if m.Status != 1 {
			continue
		}
		grouped[m.ParentID] = append(grouped[m.ParentID], m)
	}
	for pid := range grouped {
		items := grouped[pid]
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Sort != items[j].Sort {
				return items[i].Sort < items[j].Sort
			}
			return items[i].ID < items[j].ID
		})
		grouped[pid] = items
	}

	used := make(map[string]bool, len(list))
	var build func(pid uint64) []*RouteNode
	build = func(pid uint64) []*RouteNode {
		nodes := make([]*RouteNode, 0, 4)
		for _, m := range grouped[pid] {
			if m.Type == entity.MenuTypeButton || m.Type == entity.MenuTypeAPI {
				continue
			}
			// 隐藏菜单（visible=0）仍然下发为路由，只把 meta.isHide 置 true：
			// 否则「个人中心」这类隐藏页面在 backend 权限模式下根本没有路由可跳转。
			node := &RouteNode{
				ID:       m.ID,
				ParentID: m.ParentID,
				Path:     m.Path,
				Redirect: m.Redirect,
				Meta: RouteMeta{
					Title:      m.Name,
					TitleEn:    m.NameEn,
					Icon:       normalizeMenuIcon(m.Icon),
					KeepAlive:  m.KeepAlive,
					IsHide:     !m.Visible,
					FixedTab:   m.Affix,
					Type:       m.Type,
					Permission: m.Permission,
					Sort:       m.Sort,
					AuthList:   authListOf(grouped[m.ID]),
				},
			}
			if m.Type == entity.MenuTypeDir {
				node.Component = LayoutComponent
			} else {
				node.Component = normalizeComponentPath(m.Component)
			}
			children := build(m.ID)
			if len(children) > 0 {
				node.Children = children
			}
			node.Name = uniqueRouteName(routeName(m, node.Component), m.ID, used)
			nodes = append(nodes, node)
		}
		return nodes
	}

	return build(0), nil
}

// authListOf 摘取菜单下的按钮权限点。
func authListOf(children []entity.Menu) []RouteAuth {
	auths := make([]RouteAuth, 0, len(children))
	for _, c := range children {
		if c.Type != entity.MenuTypeButton || c.Permission == "" {
			continue
		}
		auths = append(auths, RouteAuth{Title: c.Name, AuthMark: c.Permission})
	}
	if len(auths) == 0 {
		return nil
	}
	return auths
}

// normalizeComponentPath 将库中存储的组件路径规范为前端可解析的形式：
// `system/user/index` → `/system/user`；`Layout` / 空 → 布局容器。
func normalizeComponentPath(component string) string {
	c := strings.TrimSpace(component)
	if c == "" || strings.EqualFold(c, "layout") {
		return LayoutComponent
	}
	c = "/" + strings.Trim(c, "/")
	c = strings.TrimSuffix(c, "/index")
	return c
}

// routeName 生成唯一路由名称：取组件路径的 PascalCase（如 /system/user → SystemUser）。
func routeName(m entity.Menu, component string) string {
	base := component
	if base == "" || base == LayoutComponent {
		base = m.Path
	}
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "index" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// uniqueRouteName 兜底保证路由名唯一（重名时追加菜单 ID）。
func uniqueRouteName(name string, id uint64, used map[string]bool) string {
	if name == "" {
		name = "Menu"
	}
	candidate := name
	for used[candidate] {
		candidate = name + "-" + uintToString(id)
		if used[candidate] {
			id++
			candidate = name + "-" + uintToString(id)
		}
	}
	used[candidate] = true
	return candidate
}

func uintToString(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// 图标别名：兼容存量数据中的 Element Plus 裸名称（settings / user ...），
// 统一转换为 Iconify 名称（ep:xxx / ri:xxx），前端 ArtSvgIcon 可直接渲染。
var menuIconAlias = map[string]string{
	"setting":   "ep:setting",
	"settings":  "ep:setting",
	"user":      "ep:user",
	"users":     "ep:user-filled",
	"team":      "ep:user-filled",
	"people":    "ep:user",
	"monitor":   "ep:monitor",
	"dashboard": "ep:odometer",
	"document":  "ep:document",
	"file":      "ep:document-copy",
	"folder":    "ep:folder",
	"list":      "ep:list",
	"edit":      "ep:edit",
	"lock":      "ep:lock",
	"tool":      "ep:tools",
	"tools":     "ep:tools",
	"tree":      "ep:share",
	"dict":      "ep:notebook",
	"bell":      "ep:bell",
	"home":      "ep:home-filled",
	"link":      "ep:link",
	"chart":     "ep:trend-charts",
	"log":       "ep:tickets",
	"database":  "ep:coin",
	"menu":      "ep:menu",
	"post":      "ep:postcard",
	"tenant":    "ep:office-building",
	"online":    "ep:connection",
	"server":    "ep:cpu",
	"shield":    "ep:lock",
	// 角色 / 部门 / 岗位 / 字典 / 参数 / 任务 / 监控 等常见语义名
	"safe":          "ep:avatar",
	"role":          "ep:avatar",
	"cluster":       "ep:office-building",
	"dept":          "ep:office-building",
	"id-card":       "ep:postcard",
	"book":          "ep:notebook",
	"sliders":       "ep:operation",
	"clock":         "ep:timer",
	"timer":         "ep:timer",
	"flag":          "ep:flag",
	"login":         "ep:key",
	"record":        "ep:tickets",
	"management":    "ep:management",
	"organization":  "ep:management",
	"permission":    "ep:key",
	"key":           "ep:key",
	"box":           "ep:box",
	"files":         "ep:files",
	"data":          "ep:data-line",
	"grid":          "ep:grid",
	"calendar":      "ep:calendar",
	"message":       "ep:message",
	"pie-chart":     "ep:pie-chart",
	"histogram":     "ep:histogram",
	"finished":      "ep:finished",
	"switch-button": "ep:switch-button",
	"set-up":        "ep:set-up",
	"guide":         "ep:guide",
}

// NormalizeMenuIcon 图标名称规范化（导出：菜单树接口同样需要把裸名转成 Iconify 名）。
func NormalizeMenuIcon(icon string) string {
	return normalizeMenuIcon(icon)
}

// normalizeMenuIcon 图标名称规范化。
func normalizeMenuIcon(icon string) string {
	ic := strings.TrimSpace(icon)
	if ic == "" {
		return ""
	}
	if strings.Contains(ic, ":") {
		return ic
	}
	lower := strings.ToLower(ic)
	if v, ok := menuIconAlias[lower]; ok {
		return v
	}
	return "ep:" + lower
}
