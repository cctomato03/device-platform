// Package principal 当前登录用户上下文（从 Redis 缓存加载，随请求传递）。
package principal

import "github.com/gin-gonic/gin"

const ctxKey = "principal"

// Info 登录主体信息。权限集合与数据范围随缓存刷新，不在 JWT 内冗余。
type Info struct {
	UserID        uint64          `json:"userId,string"`
	TenantID      uint64          `json:"tenantId,string"`
	Username      string          `json:"username"`
	Nickname      string          `json:"nickname"`
	DeptID        uint64          `json:"deptId,string"`
	DeptName      string          `json:"deptName"`
	Avatar        string          `json:"avatar"`
	Email         string          `json:"email"`
	Mobile        string          `json:"mobile"`
	Sex           int             `json:"sex"`
	IsSuper       bool            `json:"isSuper"`
	DataScope     int             `json:"dataScope"`
	Roles         []string        `json:"roles"`
	Perms         []string        `json:"perms"`
	PermSet       map[string]bool `json:"-"`
	DeptIDs       []uint64        `json:"-"` // 本部门及以下
	CustomDeptIDs []uint64        `json:"-"` // 自定义数据权限部门
	JTI           string          `json:"jti"`
}

// BuildSet 构建权限集合（加载后调用）。
func (p *Info) BuildSet() {
	p.PermSet = make(map[string]bool, len(p.Perms))
	for _, code := range p.Perms {
		p.PermSet[code] = true
	}
}

// HasPerm 是否拥有权限码。
func (p *Info) HasPerm(code string) bool {
	if p.IsSuper {
		return true
	}
	if p.PermSet != nil {
		return p.PermSet[code]
	}
	for _, c := range p.Perms {
		if c == code {
			return true
		}
	}
	return false
}

// HasAnyPerm 是否拥有任一权限码。
func (p *Info) HasAnyPerm(codes ...string) bool {
	for _, c := range codes {
		if p.HasPerm(c) {
			return true
		}
	}
	return false
}

// Set 注入上下文。
func Set(c *gin.Context, p *Info) { c.Set(ctxKey, p) }

// Get 读取上下文。
func Get(c *gin.Context) (*Info, bool) {
	v, ok := c.Get(ctxKey)
	if !ok {
		return nil, false
	}
	p, ok := v.(*Info)
	return p, ok
}

// Must 读取上下文（中间件保证存在）。
func Must(c *gin.Context) *Info {
	p, _ := Get(c)
	if p == nil {
		return &Info{}
	}
	return p
}
