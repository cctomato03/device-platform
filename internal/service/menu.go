package service

import (
	"context"
	"fmt"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// MenuNode 菜单树节点。
type MenuNode struct {
	entity.Menu
	Children []*MenuNode `json:"children,omitempty"`
}

// buildMenuTree 扁平菜单 → 树。
func buildMenuTree(list []entity.Menu) []*MenuNode {
	nodes := make(map[uint64]*MenuNode, len(list))
	order := make([]uint64, 0, len(list))
	for i := range list {
		n := &MenuNode{Menu: list[i]}
		// 图标统一输出 Iconify 名称（ep:xxx / ri:xxx），前端 ArtSvgIcon 直接渲染
		n.Menu.Icon = NormalizeMenuIcon(n.Menu.Icon)
		nodes[list[i].ID] = n
		order = append(order, list[i].ID)
	}
	roots := make([]*MenuNode, 0, 8)
	for _, id := range order {
		n := nodes[id]
		pid := n.ParentID
		if pid == 0 {
			roots = append(roots, n)
			continue
		}
		if parent, ok := nodes[pid]; ok {
			parent.Children = append(parent.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots
}

// MenuTree 全部菜单树（菜单管理页面，含按钮与隐藏项）。
func (s *Service) MenuTree(ctx context.Context, p *principal.Info) ([]*MenuNode, error) {
	list, err := s.Repo.AllMenus(ctx, 0) // 菜单为平台级配置，跨租户共享
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return buildMenuTree(list), nil
}

// UserMenuTree 当前用户可见导航菜单（目录 + 菜单，过滤隐藏与停用）。
func (s *Service) UserMenuTree(ctx context.Context, p *principal.Info) ([]*MenuNode, error) {
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

	nav := make([]entity.Menu, 0, len(list))
	for _, m := range list {
		if m.Type == entity.MenuTypeDir || m.Type == entity.MenuTypeMenu {
			if !m.Visible {
				continue
			}
			nav = append(nav, m)
		}
	}
	return buildMenuTree(nav), nil
}

// UserInfoResp 登录用户信息（含权限码与菜单树）。
type UserInfoResp struct {
	User        *principal.Info `json:"user"`
	Permissions []string        `json:"permissions"`
	Menus       []*MenuNode     `json:"menus"`
}

// UserInfo 组装 /auth/userinfo 响应。
func (s *Service) UserInfo(ctx context.Context, p *principal.Info) (*UserInfoResp, error) {
	menus, err := s.UserMenuTree(ctx, p)
	if err != nil {
		return nil, err
	}
	perms := p.Perms
	if p.IsSuper {
		perms = []string{"*:*:*"} // 超管前端据此放开所有按钮
	}
	if perms == nil {
		perms = []string{}
	}
	return &UserInfoResp{User: p, Permissions: perms, Menus: menus}, nil
}

// MenuReq 菜单新增/修改入参。
type MenuReq struct {
	ParentID   jsonx.Uint64 `json:"parentId"`
	Name       string       `json:"name" binding:"required,max=64"`
	Type       int          `json:"type" binding:"required,oneof=1 2 3 4"`
	Permission string       `json:"permission" binding:"max=128"`
	Path       string       `json:"path" binding:"max=200"`
	Component  string       `json:"component" binding:"max=200"`
	Redirect   string       `json:"redirect" binding:"max=200"`
	Icon       string       `json:"icon" binding:"max=64"`
	Sort       int          `json:"sort"`
	Visible    bool         `json:"visible"`
	KeepAlive  bool         `json:"keepAlive"`
	Affix      bool         `json:"affix"`
	IsFrame    bool         `json:"isFrame"`
	IsPublic   bool         `json:"isPublic"`
	APIMethod  string       `json:"apiMethod"`
	APIPath    string       `json:"apiPath"`
	Status     int          `json:"status"`
	Remark     string       `json:"remark"`
}

// CreateMenu 新增菜单。
func (s *Service) CreateMenu(ctx context.Context, req *MenuReq, p *principal.Info) (uint64, error) {
	now := time.Now()
	m := &entity.Menu{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   p.UserID,
			CreateTime: now,
			UpdateBy:   p.UserID,
			UpdateTime: &now,
			Remark:     req.Remark,
		},
		ParentID:   req.ParentID.Uint64(),
		Name:       req.Name,
		Type:       req.Type,
		Permission: req.Permission,
		Path:       req.Path,
		Component:  req.Component,
		Redirect:   req.Redirect,
		Icon:       req.Icon,
		Sort:       req.Sort,
		Visible:    req.Visible,
		KeepAlive:  req.KeepAlive,
		Affix:      req.Affix,
		IsFrame:    req.IsFrame,
		IsPublic:   req.IsPublic,
		APIMethod:  req.APIMethod,
		APIPath:    req.APIPath,
		Status:     req.Status,
		TenantID:   0,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(m).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return m.ID, nil
}

// UpdateMenu 修改菜单。
func (s *Service) UpdateMenu(ctx context.Context, menuID uint64, req *MenuReq, p *principal.Info) error {
	if menuID == req.ParentID.Uint64() {
		return errs.ErrBadRequest
	}
	// 防环：沿父链上溯，若回到自己说明把父级设成了自己的子孙
	if parentID := req.ParentID.Uint64(); parentID > 0 {
		cur := parentID
		for i := 0; i < 32 && cur > 0; i++ {
			if cur == menuID {
				return errs.InvalidErr(fmt.Errorf("不能把菜单挂到自己的子菜单下"))
			}
			var up uint64
			if err := s.Repo.DB.WithContext(ctx).Model(&entity.Menu{}).Where("id = ?", cur).Pluck("parent_id", &up).Error; err != nil {
				return errs.ErrDB.WithErr(err)
			}
			cur = up
		}
	}
	err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Menu{}, menuID, map[string]any{
		"parent_id":   req.ParentID.Uint64(),
		"name":        req.Name,
		"type":        req.Type,
		"permission":  req.Permission,
		"path":        req.Path,
		"component":   req.Component,
		"redirect":    req.Redirect,
		"icon":        req.Icon,
		"sort":        req.Sort,
		"visible":     req.Visible,
		"keep_alive":  req.KeepAlive,
		"affix":       req.Affix,
		"is_frame":    req.IsFrame,
		"is_public":   req.IsPublic,
		"api_method":  req.APIMethod,
		"api_path":    req.APIPath,
		"status":      req.Status,
		"remark":      req.Remark,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	})
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}

// DeleteMenu 删除菜单（存在子节点时拒绝）。
func (s *Service) DeleteMenu(ctx context.Context, menuID uint64) error {
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Menu{}).Where("parent_id = ?", menuID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.ErrHasChildren
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Menu{}, menuID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 关联表清理：0 行属正常（菜单可能未授权给任何角色），不能用 repository.Result 判定不存在
	if err := s.Repo.DB.WithContext(ctx).Where("menu_id = ?", menuID).Delete(&entity.RoleMenu{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}
