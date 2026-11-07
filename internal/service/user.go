package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/crypto"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
	"web-basic/pkg/mask"
)

// UserVO 用户列表出参。
// 注意：roleIds/postIds 是雪花 ID，必须序列化为字符串，否则前端 JS 精度丢失会把错误的 ID 写回关联表。
type UserVO struct {
	entity.User
	DeptName  string            `json:"deptName"`
	RoleIDs   jsonx.Uint64Slice `json:"roleIds"`
	RoleNames []string          `json:"roleNames"`
	PostIDs   jsonx.Uint64Slice `json:"postIds"`
}

// userOfTenant 按 ID 取用户并校验租户归属（超管 tenant=0 时跳过校验）。
// 列表接口已按租户过滤，详情/改删等按 ID 的操作必须同样校验，否则可跨租户越权操作。
func (s *Service) userOfTenant(ctx context.Context, userID uint64, p *principal.Info) (*entity.User, error) {
	u, err := s.Repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if u == nil {
		return nil, errs.ErrNotFound
	}
	if p != nil && p.TenantID != 0 && u.TenantID != p.TenantID {
		return nil, errs.ErrNotFound // 不暴露其它租户数据的存在性
	}
	return u, nil
}

// CreateUserReq 新增用户入参。
type CreateUserReq struct {
	Username string            `json:"username" binding:"required,min=3,max=32"`
	Nickname string            `json:"nickname" binding:"max=64"`
	Password string            `json:"password" binding:"omitempty,min=6,max=32"`
	DeptID   jsonx.Uint64      `json:"deptId"`
	Mobile   string            `json:"mobile" binding:"omitempty,max=20"`
	Email    string            `json:"email" binding:"omitempty,email"`
	Sex      int               `json:"sex"`
	Status   int               `json:"status"`
	RoleIDs  jsonx.Uint64Slice `json:"roleIds"`
	PostIDs  jsonx.Uint64Slice `json:"postIds"`
	Remark   string            `json:"remark"`
}

// UpdateUserReq 修改用户入参。
type UpdateUserReq struct {
	Nickname string            `json:"nickname" binding:"max=64"`
	DeptID   jsonx.Uint64      `json:"deptId"`
	Mobile   string            `json:"mobile" binding:"omitempty,max=20"`
	Email    string            `json:"email" binding:"omitempty,email"`
	Sex      int               `json:"sex"`
	Status   int               `json:"status"`
	RoleIDs  jsonx.Uint64Slice `json:"roleIds"`
	PostIDs  jsonx.Uint64Slice `json:"postIds"`
	// 指针类型：前端不传 remark 时不覆盖已有备注（用 string 会被默认为 ""，编辑一次就清空备注）
	Remark *string `json:"remark"`
}

// ListUsers 分页查询用户。
func (s *Service) ListUsers(ctx context.Context, q *repository.UserQuery, p *principal.Info) ([]UserVO, int64, error) {
	var deptIDs []uint64
	if q.DeptID > 0 {
		ids, err := s.Repo.SubtreeDeptIDs(ctx, q.DeptID)
		if err != nil {
			return nil, 0, errs.ErrDB.WithErr(err)
		}
		deptIDs = ids
	}

	users, total, err := s.Repo.ListUsers(ctx, q, p, deptIDs)
	if err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}

	deptNames, _ := s.deptNameMap(ctx, p.TenantID)
	out := make([]UserVO, 0, len(users))
	for _, u := range users {
		vo := UserVO{User: u, DeptName: deptNames[u.DeptID]}
		if !p.IsSuper {
			vo.Mobile = mask.Mobile(u.Mobile)
			vo.Email = mask.Email(u.Email)
		}
		roleIDs, _ := s.Repo.RoleIDsOfUser(ctx, u.ID)
		vo.RoleIDs = roleIDs
		postIDs, _ := s.Repo.PostIDsOfUser(ctx, u.ID)
		vo.PostIDs = postIDs
		out = append(out, vo)
	}
	s.fillRoleNames(ctx, out)
	return out, total, nil
}

// deptNameMap 部门 ID → 名称。
func (s *Service) deptNameMap(ctx context.Context, tenantID uint64) (map[uint64]string, error) {
	depts, err := s.Repo.ListDepts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]string, len(depts))
	for _, d := range depts {
		out[d.ID] = d.Name
	}
	return out, nil
}

// fillRoleNames 批量填充角色名。
func (s *Service) fillRoleNames(ctx context.Context, users []UserVO) {
	var roles []entity.Role
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Find(&roles).Error; err != nil {
		return
	}
	byID := make(map[uint64]string, len(roles))
	for _, r := range roles {
		byID[r.ID] = r.Name
	}
	for i := range users {
		names := make([]string, 0, len(users[i].RoleIDs))
		for _, rid := range users[i].RoleIDs {
			if name, ok := byID[rid]; ok {
				names = append(names, name)
			}
		}
		users[i].RoleNames = names
	}
}

// GetUser 用户详情（含角色与岗位）。
func (s *Service) GetUser(ctx context.Context, userID uint64, p *principal.Info) (*UserVO, error) {
	u, err := s.userOfTenant(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	vo := &UserVO{User: *u}
	dept, _ := s.Repo.FindDeptByID(ctx, u.DeptID)
	if dept != nil {
		vo.DeptName = dept.Name
	}
	vo.RoleIDs, _ = s.Repo.RoleIDsOfUser(ctx, u.ID)
	vo.PostIDs, _ = s.Repo.PostIDsOfUser(ctx, u.ID)
	users := []UserVO{*vo}
	s.fillRoleNames(ctx, users)
	vo.RoleNames = users[0].RoleNames
	return vo, nil
}

// CreateUser 新增用户。
func (s *Service) CreateUser(ctx context.Context, req *CreateUserReq, p *principal.Info) (uint64, error) {
	existing, err := s.Repo.FindUserByUsername(ctx, p.TenantID, req.Username)
	if err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if existing != nil {
		return 0, errs.ErrDuplicate
	}

	pwd := req.Password
	if pwd == "" {
		pwd = "Aa123456" // 默认密码，建议首次登录强制修改
	}
	if err := crypto.CheckStrength(pwd, s.Cfg.Auth.PwdMinLen); err != nil {
		return 0, errs.InvalidErr(err)
	}
	hash, err := crypto.HashPassword(pwd)
	if err != nil {
		return 0, errs.ErrInternal.WithErr(err)
	}

	now := time.Now()
	user := &entity.User{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   p.UserID,
			CreateTime: now,
			UpdateBy:   p.UserID,
			UpdateTime: &now,
			Remark:     req.Remark,
		},
		Username:      req.Username,
		Nickname:      req.Nickname,
		Password:      hash,
		DeptID:        req.DeptID.Uint64(),
		Mobile:        req.Mobile,
		Email:         req.Email,
		Sex:           req.Sex,
		Status:        req.Status,
		PwdUpdateTime: &now,
		TenantID:      p.TenantID,
	}
	if user.Nickname == "" {
		user.Nickname = req.Username
	}
	// 用户 + 角色 + 岗位 一并事务写入，避免"用户建了但关联丢了"
	if err := s.Repo.CreateUserWithRelations(ctx, user, req.RoleIDs.ToUint64(), req.PostIDs.ToUint64()); err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return user.ID, nil
}

// UpdateUser 修改用户。
func (s *Service) UpdateUser(ctx context.Context, userID uint64, req *UpdateUserReq, p *principal.Info) error {
	u, err := s.userOfTenant(ctx, userID, p)
	if err != nil {
		return err
	}
	if u.IsSuper && !p.IsSuper {
		return errs.ErrCannotOperateSup
	}

	fields := map[string]any{
		"nickname":    req.Nickname,
		"dept_id":     req.DeptID.Uint64(),
		"mobile":      req.Mobile,
		"email":       req.Email,
		"sex":         req.Sex,
		"status":      req.Status,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}
	if req.Remark != nil {
		fields["remark"] = *req.Remark
	}
	var roleIDs, postIDs []uint64
	if req.RoleIDs != nil {
		roleIDs = req.RoleIDs.ToUint64()
	}
	if req.PostIDs != nil {
		postIDs = req.PostIDs.ToUint64()
	}
	if err := s.Repo.UpdateUserWithRelations(ctx, userID, fields, roleIDs, postIDs); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.invalidatePrincipal(ctx, userID)
	return nil
}

// DeleteUsers 批量软删除。
func (s *Service) DeleteUsers(ctx context.Context, ids []uint64, p *principal.Info) error {
	for _, targetID := range ids {
		if targetID == p.UserID {
			return errs.ErrCannotOperateOwn
		}
		u, err := s.userOfTenant(ctx, targetID, p)
		if err != nil {
			if errors.Is(err, errs.ErrNotFound) {
				continue // 不存在 / 不属于当前租户：直接跳过
			}
			return err
		}
		if u.IsSuper {
			return errs.ErrCannotOperateSup
		}
		if err := s.Repo.SoftDeleteUser(ctx, targetID); err != nil {
			return errs.ErrDB.WithErr(err)
		}
		s.revokeUserSessions(ctx, targetID)
	}
	return nil
}

// ChangeUserStatus 启用/停用。
func (s *Service) ChangeUserStatus(ctx context.Context, userID uint64, status int, p *principal.Info) error {
	u, err := s.userOfTenant(ctx, userID, p)
	if err != nil {
		return err
	}
	// 兼容前端的 1=启用 / 2=停用 与后端的 1=启用 / 0=停用
	if status == 2 {
		status = 0
	}
	if status != 0 && status != 1 {
		return errs.ErrBadRequest
	}
	if u.IsSuper && status != 1 {
		return errs.ErrCannotOperateSup
	}
	if userID == p.UserID && status != 1 {
		return errs.ErrCannotOperateOwn
	}
	if err := s.Repo.UpdateUserFields(ctx, userID, map[string]any{"status": status, "update_by": p.UserID}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if status == 0 {
		s.revokeUserSessions(ctx, userID) // 停用后立即踢下线
		return nil
	}
	s.invalidatePrincipal(ctx, userID)
	return nil
}

// ResetPassword 重置密码，返回新密码明文（仅本次返回）。
func (s *Service) ResetPassword(ctx context.Context, userID uint64, newPassword string, p *principal.Info) (string, error) {
	u, err := s.userOfTenant(ctx, userID, p)
	if err != nil {
		return "", err
	}
	if u.IsSuper && !p.IsSuper {
		return "", errs.ErrCannotOperateSup
	}
	if newPassword == "" {
		newPassword = crypto.RandomPassword(12)
	}
	if err := crypto.CheckStrength(newPassword, s.Cfg.Auth.PwdMinLen); err != nil {
		return "", errs.InvalidErr(err)
	}
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return "", errs.ErrInternal.WithErr(err)
	}
	now := time.Now()
	if err := s.Repo.UpdateUserFields(ctx, userID, map[string]any{
		"password":        hash,
		"pwd_update_time": now,
		"fail_count":      0,
		"locked_until":    nil,
		"update_by":       p.UserID,
	}); err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	s.revokeUserSessions(ctx, userID) // 重置密码后旧会话全部失效
	return newPassword, nil
}

// AssignUserRoles 分配角色。
func (s *Service) AssignUserRoles(ctx context.Context, userID uint64, roleIDs []uint64, p *principal.Info) error {
	u, err := s.userOfTenant(ctx, userID, p)
	if err != nil {
		return err
	}
	if u.IsSuper && !p.IsSuper {
		return errs.ErrCannotOperateSup
	}
	if err := s.Repo.SetUserRoles(ctx, userID, roleIDs); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.invalidatePrincipal(ctx, userID)
	return nil
}

// Profile 个人资料。
type Profile struct {
	ID       uint64   `json:"id,string"`
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Avatar   string   `json:"avatar"`
	Mobile   string   `json:"mobile"`
	Email    string   `json:"email"`
	Sex      int      `json:"sex"`
	DeptName string   `json:"deptName"`
	Roles    []string `json:"roles"`
}

// MyProfile 当前登录用户资料。
func (s *Service) MyProfile(ctx context.Context, p *principal.Info) (*Profile, error) {
	u, err := s.Repo.FindUserByID(ctx, p.UserID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if u == nil {
		return nil, errs.ErrNotFound
	}
	return &Profile{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Mobile:   u.Mobile,
		Email:    u.Email,
		Sex:      u.Sex,
		DeptName: p.DeptName,
		Roles:    p.Roles,
	}, nil
}

// UpdateMyProfile 修改个人资料。
func (s *Service) UpdateMyProfile(ctx context.Context, nickname, avatar, mobile, email string, sex int, p *principal.Info) error {
	fields := map[string]any{
		"nickname":    strings.TrimSpace(nickname),
		"avatar":      avatar,
		"mobile":      mobile,
		"email":       email,
		"sex":         sex,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}
	if err := s.Repo.UpdateUserFields(ctx, p.UserID, fields); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.invalidatePrincipal(ctx, p.UserID)
	return nil
}

// ChangeMyPassword 修改自己的密码。
func (s *Service) ChangeMyPassword(ctx context.Context, oldPwd, newPwd string, p *principal.Info) error {
	u, err := s.Repo.FindUserByID(ctx, p.UserID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if u == nil {
		return errs.ErrNotFound
	}
	if !crypto.VerifyPassword(u.Password, oldPwd) {
		return errs.ErrPasswordWrong
	}
	if err := crypto.CheckStrength(newPwd, s.Cfg.Auth.PwdMinLen); err != nil {
		return errs.InvalidErr(err)
	}
	hash, err := crypto.HashPassword(newPwd)
	if err != nil {
		return errs.ErrInternal.WithErr(err)
	}
	if err := s.Repo.UpdateUserFields(ctx, p.UserID, map[string]any{
		"password":        hash,
		"pwd_update_time": time.Now(),
		"update_by":       p.UserID,
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 改密后所有旧令牌立即失效（前端应引导用户重新登录）
	s.revokeUserSessions(ctx, p.UserID)
	return nil
}
