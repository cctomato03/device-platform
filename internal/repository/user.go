package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/model/scope"
	"web-basic/internal/principal"
)

// UserQuery 用户列表查询条件。
type UserQuery struct {
	entity.PageQuery
	Username string `form:"username"`
	Mobile   string `form:"mobile"`
	Status   *int   `form:"status"`
	DeptID   uint64 `form:"deptId"`
}

// FindUserByID 按 ID 查询用户。
func (r *Repo) FindUserByID(ctx context.Context, id uint64) (*entity.User, error) {
	var u entity.User
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

// FindUserByUsername 按账号查询（租户内或平台级）。
func (r *Repo) FindUserByUsername(ctx context.Context, tenantID uint64, username string) (*entity.User, error) {
	var u entity.User
	q := r.DB.WithContext(ctx).Where("username = ?", username)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// ListUsers 分页查询用户（自动套用租户与数据权限）。
func (r *Repo) ListUsers(ctx context.Context, q *UserQuery, p *principal.Info, deptIDs []uint64) ([]entity.User, int64, error) {
	q.Normalize()
	tx := r.DB.WithContext(ctx).Model(&entity.User{}).
		Scopes(scope.Tenant(p.TenantID), scope.DataScope(p, "dept_id", "create_by"))

	if q.Username != "" {
		tx = tx.Where("username LIKE ? OR nickname LIKE ?", q.Username+"%", q.Username+"%")
	}
	if q.Mobile != "" {
		tx = tx.Where("mobile LIKE ?", q.Mobile+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.DeptID > 0 {
		tx = tx.Where("dept_id IN ?", deptIDs)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	list := make([]entity.User, 0, q.PageSize)
	order := orderClause(q.OrderBy, q.Order, map[string]string{
		"createTime": "create_time",
		"username":   "username",
		"id":         "id",
	}, "id DESC")
	err := tx.Order(order).Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error
	return list, total, err
}

// CreateUser 新增用户。
func (r *Repo) CreateUser(ctx context.Context, u *entity.User) error {
	return r.DB.WithContext(ctx).Create(u).Error
}

// CreateUserWithRelations 在同一个事务里创建用户并写入角色/岗位关联。
// 分开写会出现"用户建了但角色没写进"的半成品数据（前端表现为新用户没有任何权限）。
func (r *Repo) CreateUserWithRelations(ctx context.Context, u *entity.User, roleIDs, postIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		for _, rid := range roleIDs {
			if err := tx.Create(&entity.UserRole{UserID: u.ID, RoleID: rid}).Error; err != nil {
				return err
			}
		}
		for _, pid := range postIDs {
			if err := tx.Create(&entity.UserPost{UserID: u.ID, PostID: pid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateUserWithRelations 在同一个事务里更新用户字段与角色/岗位关联（roleIDs/postIDs 为 nil 表示不改）。
func (r *Repo) UpdateUserWithRelations(ctx context.Context, id uint64, fields map[string]any, roleIDs, postIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := UpdateByID(ctx, tx, &entity.User{}, id, fields); err != nil {
			return err
		}
		if roleIDs != nil {
			if err := tx.Where("user_id = ?", id).Delete(&entity.UserRole{}).Error; err != nil {
				return err
			}
			for _, rid := range roleIDs {
				if err := tx.Create(&entity.UserRole{UserID: id, RoleID: rid}).Error; err != nil {
					return err
				}
			}
		}
		if postIDs != nil {
			if err := tx.Where("user_id = ?", id).Delete(&entity.UserPost{}).Error; err != nil {
				return err
			}
			for _, pid := range postIDs {
				if err := tx.Create(&entity.UserPost{UserID: id, PostID: pid}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// UpdateUserFields 更新指定字段。
func (r *Repo) UpdateUserFields(ctx context.Context, id uint64, fields map[string]any) error {
	return UpdateByID(ctx, r.DB, &entity.User{}, id, fields)
}

// SoftDeleteUser 软删除用户，同时清理其角色/岗位关联，避免留下孤儿关联记录。
func (r *Repo) SoftDeleteUser(ctx context.Context, id uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := Result(tx.Delete(&entity.User{}, id)); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&entity.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", id).Delete(&entity.UserPost{}).Error
	})
}

// RoleIDsOfUser 用户角色 ID 列表。
func (r *Repo) RoleIDsOfUser(ctx context.Context, userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.DB.WithContext(ctx).Model(&entity.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

// PostIDsOfUser 用户岗位 ID 列表。
func (r *Repo) PostIDsOfUser(ctx context.Context, userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.DB.WithContext(ctx).Model(&entity.UserPost{}).
		Where("user_id = ?", userID).Pluck("post_id", &ids).Error
	return ids, err
}

// SetUserRoles 覆盖式设置用户角色。
func (r *Repo) SetUserRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&entity.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]entity.UserRole, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rows = append(rows, entity.UserRole{UserID: userID, RoleID: rid})
		}
		return tx.Create(&rows).Error
	})
}

// SetUserPosts 覆盖式设置用户岗位。
func (r *Repo) SetUserPosts(ctx context.Context, userID uint64, postIDs []uint64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&entity.UserPost{}).Error; err != nil {
			return err
		}
		if len(postIDs) == 0 {
			return nil
		}
		rows := make([]entity.UserPost, 0, len(postIDs))
		for _, pid := range postIDs {
			rows = append(rows, entity.UserPost{UserID: userID, PostID: pid})
		}
		return tx.Create(&rows).Error
	})
}

// UpdateLoginSuccess 登录成功后更新登录信息并清零失败计数。
func (r *Repo) UpdateLoginSuccess(ctx context.Context, id uint64, ip string) error {
	now := time.Now()
	return r.DB.WithContext(ctx).Model(&entity.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login_ip":   ip,
		"last_login_time": now,
		"fail_count":      0,
		"locked_until":    nil,
	}).Error
}

// UpdateLoginFail 登录失败：失败次数 +1，超过阈值则锁定。
func (r *Repo) UpdateLoginFail(ctx context.Context, id uint64, maxFail int, lockMinutes int) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u entity.User
		if err := tx.Where("id = ?", id).First(&u).Error; err != nil {
			return err
		}
		fail := u.FailCount + 1
		fields := map[string]any{"fail_count": fail}
		if maxFail > 0 && fail >= maxFail {
			fields["locked_until"] = time.Now().Add(time.Duration(lockMinutes) * time.Minute)
			fields["fail_count"] = 0
		}
		return tx.Model(&entity.User{}).Where("id = ?", id).Updates(fields).Error
	})
}

// orderClause 生成安全的排序语句（字段白名单）。
func orderClause(orderBy, order string, allow map[string]string, fallback string) string {
	col, ok := allow[orderBy]
	if !ok {
		return fallback
	}
	dir := "ASC"
	if strings.EqualFold(order, "desc") {
		dir = "DESC"
	}
	return col + " " + dir
}
