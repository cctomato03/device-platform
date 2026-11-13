package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"device-platform/internal/model/entity"
)

// RolesOfUser 用户拥有的角色。
func (r *Repo) RolesOfUser(ctx context.Context, userID uint64) ([]entity.Role, error) {
	var roles []entity.Role
	err := r.DB.WithContext(ctx).Model(&entity.Role{}).
		Joins("JOIN sys_user_role ur ON ur.role_id = sys_role.id").
		Where("ur.user_id = ? AND sys_role.status = 1", userID).
		Order("sys_role.sort ASC").Find(&roles).Error
	return roles, err
}

// PermsOfUser 用户权限码集合（菜单 + 按钮 + 接口，去重）。
func (r *Repo) PermsOfUser(ctx context.Context, userID uint64) ([]string, error) {
	var perms []string
	err := r.DB.WithContext(ctx).Table("sys_menu AS m").
		Select("DISTINCT m.permission").
		Joins("JOIN sys_role_menu rm ON rm.menu_id = m.id").
		Joins("JOIN sys_user_role ur ON ur.role_id = rm.role_id").
		Joins("JOIN sys_role r ON r.id = rm.role_id").
		Where("ur.user_id = ? AND m.status = 1 AND m.permission <> '' AND r.status = 1 AND m.deleted = 0 AND r.deleted = 0", userID).
		Pluck("m.permission", &perms).Error
	return perms, err
}

// CustomDeptIDsOfRoles 角色自定义数据权限部门。
func (r *Repo) CustomDeptIDsOfRoles(ctx context.Context, roleIDs []uint64) ([]uint64, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var ids []uint64
	err := r.DB.WithContext(ctx).Model(&entity.RoleDept{}).
		Where("role_id IN ?", roleIDs).Pluck("dept_id", &ids).Error
	return ids, err
}

// SubtreeDeptIDs 某部门自身及其所有下级部门 ID（基于 ancestors 冗余路径，兼容 5.7 无 CTE）。
func (r *Repo) SubtreeDeptIDs(ctx context.Context, deptID uint64) ([]uint64, error) {
	if deptID == 0 {
		return nil, nil
	}
	var ancestors string
	err := r.DB.WithContext(ctx).Model(&entity.Dept{}).Where("id = ?", deptID).Pluck("ancestors", &ancestors).Error
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s,%d", ancestors, deptID)
	if ancestors == "" {
		path = fmt.Sprintf("0,%d", deptID)
	}
	var ids []uint64
	err = r.DB.WithContext(ctx).Model(&entity.Dept{}).
		Where("id = ? OR ancestors = ? OR ancestors LIKE ?", deptID, path, path+",%").
		Pluck("id", &ids).Error
	return ids, err
}

// MenusOfUser 用户可见菜单（目录/菜单/按钮），按角色并集去重。
func (r *Repo) MenusOfUser(ctx context.Context, userID uint64) ([]entity.Menu, error) {
	var menus []entity.Menu
	err := r.DB.WithContext(ctx).Model(&entity.Menu{}).
		Distinct("sys_menu.*").
		Joins("JOIN sys_role_menu rm ON rm.menu_id = sys_menu.id").
		Joins("JOIN sys_user_role ur ON ur.role_id = rm.role_id").
		Joins("JOIN sys_role r ON r.id = rm.role_id AND r.status = 1 AND r.deleted = 0").
		Where("ur.user_id = ? AND sys_menu.status = 1", userID).
		Order("sys_menu.sort ASC, sys_menu.id ASC").Find(&menus).Error
	return menus, err
}

// AllMenus 全部菜单（超管与菜单管理页面使用）。
func (r *Repo) AllMenus(ctx context.Context, tenantID uint64) ([]entity.Menu, error) {
	var menus []entity.Menu
	q := r.DB.WithContext(ctx).Model(&entity.Menu{}).Order("sort ASC, id ASC")
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Find(&menus).Error
	return menus, err
}

// MenuIDsOfRole 角色已授权的菜单 ID。
func (r *Repo) MenuIDsOfRole(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.DB.WithContext(ctx).Model(&entity.RoleMenu{}).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

// SetRoleMenus 覆盖式设置角色权限。
func (r *Repo) SetRoleMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&entity.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]entity.RoleMenu, 0, len(menuIDs))
		for _, mid := range menuIDs {
			rows = append(rows, entity.RoleMenu{RoleID: roleID, MenuID: mid})
		}
		return tx.Create(&rows).Error
	})
}

// SetRoleDepts 覆盖式设置角色自定义数据权限部门。
func (r *Repo) SetRoleDepts(ctx context.Context, roleID uint64, deptIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&entity.RoleDept{}).Error; err != nil {
			return err
		}
		if len(deptIDs) == 0 {
			return nil
		}
		rows := make([]entity.RoleDept, 0, len(deptIDs))
		for _, did := range deptIDs {
			rows = append(rows, entity.RoleDept{RoleID: roleID, DeptID: did})
		}
		return tx.Create(&rows).Error
	})
}

// UserIDsOfRole 拥有某角色的用户 ID（权限变更时批量失效缓存）。
func (r *Repo) UserIDsOfRole(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.DB.WithContext(ctx).Model(&entity.UserRole{}).
		Where("role_id = ?", roleID).Pluck("user_id", &ids).Error
	return ids, err
}
