package service

import (
	"context"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/model/scope"
	"device-platform/internal/principal"
	"device-platform/internal/repository"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
	"device-platform/pkg/jsonx"
)

// RoleQuery 角色列表查询。
type RoleQuery struct {
	entity.PageQuery
	Name   string `form:"name"`
	Status *int   `form:"status"`
}

// RoleReq 角色新增/修改入参。
type RoleReq struct {
	Name      string `json:"name" binding:"required,max=64"`
	Code      string `json:"code" binding:"required,max=64"`
	Sort      int    `json:"sort"`
	DataScope int    `json:"dataScope" binding:"required,oneof=1 2 3 4 5"`
	Status    int    `json:"status"`
	Remark    string `json:"remark"`
}

// ListRoles 分页查询角色。
func (s *Service) ListRoles(ctx context.Context, q *RoleQuery, p *principal.Info) ([]entity.Role, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Scopes(scope.Tenant(p.TenantID))
	if q.Name != "" {
		tx = tx.Where("name LIKE ? OR code LIKE ?", q.Name+"%", q.Name+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Role, 0, q.PageSize)
	if err := tx.Order("sort ASC, id ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// AllRoles 全部启用角色（用户分配角色下拉）。
func (s *Service) AllRoles(ctx context.Context, p *principal.Info) ([]entity.Role, error) {
	var list []entity.Role
	err := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).
		Scopes(scope.Tenant(p.TenantID)).Where("status = 1").Order("sort ASC").Find(&list).Error
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateRole 新增角色。
func (s *Service) CreateRole(ctx context.Context, req *RoleReq, p *principal.Info) (uint64, error) {
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Where("code = ?", req.Code).Count(&n).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return 0, errs.ErrDuplicate
	}
	now := time.Now()
	role := &entity.Role{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   p.UserID,
			CreateTime: now,
			UpdateBy:   p.UserID,
			UpdateTime: &now,
			Remark:     req.Remark,
		},
		Name:      req.Name,
		Code:      req.Code,
		Sort:      req.Sort,
		DataScope: req.DataScope,
		Status:    req.Status,
		TenantID:  p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(role).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return role.ID, nil
}

// UpdateRole 修改角色。
func (s *Service) UpdateRole(ctx context.Context, roleID uint64, req *RoleReq, p *principal.Info) error {
	role, err := s.Repo.FindRoleByID(ctx, roleID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if role == nil {
		return errs.ErrNotFound
	}
	if role.IsBuiltin && role.Code == "super_admin" {
		return errs.ErrCannotOperateSup
	}
	if p != nil && p.TenantID != 0 && role.TenantID != p.TenantID {
		return errs.ErrNotFound
	}
	err = repository.UpdateByID(ctx, s.Repo.DB, &entity.Role{}, roleID, map[string]any{
		"name":        req.Name,
		"code":        req.Code,
		"sort":        req.Sort,
		"data_scope":  req.DataScope,
		"status":      req.Status,
		"remark":      req.Remark,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	})
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateRoleUsers(ctx, roleID)
	return nil
}

// DeleteRole 删除角色。
func (s *Service) DeleteRole(ctx context.Context, roleID uint64, p *principal.Info) error {
	role, err := s.Repo.FindRoleByID(ctx, roleID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if role == nil {
		return errs.ErrNotFound
	}
	if role.IsBuiltin {
		return errs.ErrCannotOperateSup
	}
	if p != nil && p.TenantID != 0 && role.TenantID != p.TenantID {
		return errs.ErrNotFound
	}
	used, err := s.Repo.CountUsersOfRole(ctx, roleID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if used > 0 {
		return errs.ErrRecordUsed
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Role{}, roleID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("role_id = ?", roleID).Delete(&entity.RoleMenu{}).Error
	_ = s.Repo.DB.WithContext(ctx).Where("role_id = ?", roleID).Delete(&entity.RoleDept{}).Error
	s.InvalidateAll(ctx)
	return nil
}

// GrantReq 角色授权入参。
type GrantReq struct {
	MenuIDs jsonx.Uint64Slice `json:"menuIds"`
	DeptIDs jsonx.Uint64Slice `json:"deptIds"`
}

// GrantRole 授权（菜单/按钮 + 自定义数据权限部门）。
func (s *Service) GrantRole(ctx context.Context, roleID uint64, req *GrantReq) error {
	role, err := s.Repo.FindRoleByID(ctx, roleID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if role == nil {
		return errs.ErrNotFound
	}
	if role.Code == "super_admin" {
		return errs.ErrCannotOperateSup
	}
	if err := s.Repo.SetRoleMenus(ctx, roleID, req.MenuIDs.ToUint64()); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// deptIds 为 nil 表示"本次只授权菜单"，不能覆盖已有数据权限部门；
	// 只有显式传入（含空数组）时才重写，避免保存一次角色就把自定义数据范围清空。
	if req.DeptIDs != nil {
		if err := s.Repo.SetRoleDepts(ctx, roleID, req.DeptIDs.ToUint64()); err != nil {
			return errs.ErrDB.WithErr(err)
		}
	}
	s.InvalidateRoleUsers(ctx, roleID)
	return nil
}

// RoleGranted 角色已授权菜单与部门。
type RoleGranted struct {
	MenuIDs jsonx.Uint64Slice `json:"menuIds"`
	DeptIDs jsonx.Uint64Slice `json:"deptIds"`
}

// RoleGrantedInfo 查询角色授权情况。
func (s *Service) RoleGrantedInfo(ctx context.Context, roleID uint64) (*RoleGranted, error) {
	menuIDs, err := s.Repo.MenuIDsOfRole(ctx, roleID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	var deptIDs []uint64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.RoleDept{}).
		Where("role_id = ?", roleID).Pluck("dept_id", &deptIDs).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &RoleGranted{MenuIDs: menuIDs, DeptIDs: deptIDs}, nil
}
