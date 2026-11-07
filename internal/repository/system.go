package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
)

// FindDeptByID 查询部门。
func (r *Repo) FindDeptByID(ctx context.Context, id uint64) (*entity.Dept, error) {
	if id == 0 {
		return nil, nil
	}
	var d entity.Dept
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

// ListDepts 部门列表（租户内）。
func (r *Repo) ListDepts(ctx context.Context, tenantID uint64) ([]entity.Dept, error) {
	var list []entity.Dept
	q := r.DB.WithContext(ctx).Model(&entity.Dept{}).Order("sort ASC, id ASC")
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// DeptHasChildren 是否存在子部门。
func (r *Repo) DeptHasChildren(ctx context.Context, id uint64) (bool, error) {
	var n int64
	err := r.DB.WithContext(ctx).Model(&entity.Dept{}).Where("parent_id = ?", id).Count(&n).Error
	return n > 0, err
}

// DeptHasUsers 部门下是否有用户。
func (r *Repo) DeptHasUsers(ctx context.Context, id uint64) (bool, error) {
	var n int64
	err := r.DB.WithContext(ctx).Model(&entity.User{}).Where("dept_id = ?", id).Count(&n).Error
	return n > 0, err
}

// ListPosts 岗位列表。
func (r *Repo) ListPosts(ctx context.Context, tenantID uint64) ([]entity.Post, error) {
	var list []entity.Post
	q := r.DB.WithContext(ctx).Model(&entity.Post{}).Order("sort ASC, id ASC")
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// DictDataByCode 按字典编码取字典项。
func (r *Repo) DictDataByCode(ctx context.Context, code string) ([]entity.DictData, error) {
	var list []entity.DictData
	err := r.DB.WithContext(ctx).Model(&entity.DictData{}).
		Where("dict_code = ? AND status = 1", code).Order("sort ASC, id ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// PublicConfigs 免登录可读的公开参数。
func (r *Repo) PublicConfigs(ctx context.Context) (map[string]string, error) {
	var rows []entity.Config
	if err := r.DB.WithContext(ctx).Model(&entity.Config{}).
		Where("is_public = 1").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Value
	}
	return out, nil
}

// FindRoleByID 查询角色。
func (r *Repo) FindRoleByID(ctx context.Context, id uint64) (*entity.Role, error) {
	var role entity.Role
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// CountUsersOfRole 角色下的有效用户数（排除已软删除用户）。
func (r *Repo) CountUsersOfRole(ctx context.Context, roleID uint64) (int64, error) {
	var n int64
	err := r.DB.WithContext(ctx).Table("sys_user_role AS ur").
		Joins("JOIN sys_user AS u ON u.id = ur.user_id AND u.deleted = 0").
		Where("ur.role_id = ?", roleID).
		Count(&n).Error
	return n, err
}
