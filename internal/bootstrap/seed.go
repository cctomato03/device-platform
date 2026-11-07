package bootstrap

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/config"
	"web-basic/internal/logger"
	"web-basic/internal/model/entity"
	"web-basic/internal/repository"
	"web-basic/pkg/crypto"
	"web-basic/pkg/id"
)

// ensureAdmin 首次启动时创建超级管理员账号。
// 密码优先级：环境变量 APP__APP__ADMIN_PASSWORD > configs 中的 app.admin_password > 随机生成（打印到日志）。
func ensureAdmin(ctx context.Context, cfg *config.Config, repo *repository.Repo, _ any) error {
	var count int64
	if err := repo.DB.WithContext(ctx).Model(&entity.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password := cfg.App.AdminPassword
	generated := false
	if password == "" {
		password = crypto.RandomPassword(12)
		generated = true
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}

	now := time.Now()
	tenantID := cfg.Tenant.DefaultID
	if tenantID == 0 {
		tenantID = 1
	}

	// 取租户根部门
	var dept entity.Dept
	deptID := uint64(1)
	if err := repo.DB.WithContext(ctx).Where("tenant_id = ? AND parent_id = 0", tenantID).
		Order("id ASC").First(&dept).Error; err == nil {
		deptID = dept.ID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	admin := &entity.User{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   0,
			CreateTime: now,
			UpdateTime: &now,
			Remark:     "系统初始化创建",
		},
		Username:      "admin",
		Nickname:      "超级管理员",
		Password:      hash,
		DeptID:        deptID,
		Sex:           0,
		Status:        1,
		PwdUpdateTime: &now,
		IsSuper:       true,
		TenantID:      tenantID,
	}
	if err := repo.DB.WithContext(ctx).Create(admin).Error; err != nil {
		return err
	}

	// 绑定内置超管角色
	var role entity.Role
	if err := repo.DB.WithContext(ctx).Where("code = ?", "super_admin").First(&role).Error; err == nil {
		_ = repo.DB.WithContext(ctx).Create(&entity.UserRole{UserID: admin.ID, RoleID: role.ID}).Error
	}

	if generated {
		logger.S().Warnw("已创建超级管理员账号，请立即登录并修改密码",
			"username", "admin", "password", password)
	} else {
		logger.S().Infow("已创建超级管理员账号", "username", "admin", "password", "(取自配置/环境变量)")
	}
	return nil
}
