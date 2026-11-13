package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"device-platform/internal/model/entity"
	"device-platform/internal/principal"
	"device-platform/internal/repository"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
	"device-platform/pkg/jsonx"
)

// DeptNode 部门树节点。
type DeptNode struct {
	entity.Dept
	Children []*DeptNode `json:"children,omitempty"`
}

// DeptReq 部门新增/修改入参。
type DeptReq struct {
	ParentID jsonx.Uint64 `json:"parentId"`
	Name     string       `json:"name" binding:"required,max=64"`
	Code     string       `json:"code" binding:"max=64"`
	Leader   string       `json:"leader" binding:"max=32"`
	Phone    string       `json:"phone" binding:"max=20"`
	Email    string       `json:"email" binding:"omitempty,email"`
	Sort     int          `json:"sort"`
	Status   int          `json:"status"`
	Remark   string       `json:"remark"`
}

// DeptTree 部门树。
func (s *Service) DeptTree(ctx context.Context, p *principal.Info) ([]*DeptNode, error) {
	list, err := s.Repo.ListDepts(ctx, p.TenantID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	nodes := make(map[uint64]*DeptNode, len(list))
	order := make([]uint64, 0, len(list))
	for i := range list {
		n := &DeptNode{Dept: list[i]}
		nodes[list[i].ID] = n
		order = append(order, list[i].ID)
	}
	roots := make([]*DeptNode, 0, 8)
	for _, did := range order {
		n := nodes[did]
		if n.ParentID == 0 {
			roots = append(roots, n)
			continue
		}
		if parent, ok := nodes[n.ParentID]; ok {
			parent.Children = append(parent.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots, nil
}

// CreateDept 新增部门（自动计算 ancestors 冗余路径）。
func (s *Service) CreateDept(ctx context.Context, req *DeptReq, p *principal.Info) (uint64, error) {
	ancestors := "0"
	if req.ParentID > 0 {
		parent, err := s.Repo.FindDeptByID(ctx, req.ParentID.Uint64())
		if err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
		if parent == nil {
			return 0, errs.ErrNotFound
		}
		ancestors = fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID)
	}
	now := time.Now()
	dept := &entity.Dept{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   p.UserID,
			CreateTime: now,
			UpdateBy:   p.UserID,
			UpdateTime: &now,
			Remark:     req.Remark,
		},
		ParentID:  req.ParentID.Uint64(),
		Ancestors: ancestors,
		Name:      req.Name,
		Code:      req.Code,
		Leader:    req.Leader,
		Phone:     req.Phone,
		Email:     req.Email,
		Sort:      req.Sort,
		Status:    req.Status,
		TenantID:  p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(dept).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return dept.ID, nil
}

// UpdateDept 修改部门（父级变更时级联修正子树 ancestors）。
func (s *Service) UpdateDept(ctx context.Context, deptID uint64, req *DeptReq, p *principal.Info) error {
	dept, err := s.Repo.FindDeptByID(ctx, deptID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if dept == nil {
		return errs.ErrNotFound
	}
	// 租户校验：只能改本租户的部门
	if p != nil && p.TenantID != 0 && dept.TenantID != p.TenantID {
		return errs.ErrNotFound
	}
	if req.ParentID.Uint64() == deptID {
		return errs.ErrBadRequest
	}

	oldPath := fmt.Sprintf("%s,%d", dept.Ancestors, dept.ID)
	ancestors := "0"
	if req.ParentID > 0 {
		parent, err := s.Repo.FindDeptByID(ctx, req.ParentID.Uint64())
		if err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if parent == nil {
			return errs.ErrNotFound
		}
		// 防环：新父级不能是自己的子孙（否则部门树成环，递归查询会死循环）
		if strings.HasPrefix(fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID), oldPath+",") || fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID) == oldPath {
			return errs.InvalidErr(fmt.Errorf("不能把部门移动到自己的子部门下"))
		}
		ancestors = fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID)
	}
	newPath := fmt.Sprintf("%s,%d", ancestors, deptID)

	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.UpdateByID(ctx, tx, &entity.Dept{}, deptID, map[string]any{
			"parent_id":   req.ParentID.Uint64(),
			"ancestors":   ancestors,
			"name":        req.Name,
			"code":        req.Code,
			"leader":      req.Leader,
			"phone":       req.Phone,
			"email":       req.Email,
			"sort":        req.Sort,
			"status":      req.Status,
			"remark":      req.Remark,
			"update_by":   p.UserID,
			"update_time": time.Now(),
		}); err != nil {
			return err
		}
		if oldPath != newPath {
			// 子树 ancestors 前缀替换：直接子部门的 ancestors 恰好等于 oldPath，
			// 孙辈是 "oldPath,..."，两个条件都要覆盖，否则移动后部门树断裂。
			// SQLite 用 substr(x, n)（1 基）与 || 拼接，语义等价于 MySQL 的 SUBSTRING/CONCAT。
			return tx.Exec("UPDATE sys_dept SET ancestors = ? || substr(ancestors, ?) WHERE ancestors = ? OR ancestors LIKE ?",
				newPath, len(oldPath)+1, oldPath, oldPath+",%").Error
		}
		return nil
	})
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeptDetail 部门详情（用于前端编辑回显与删除后存在性校验）。
func (s *Service) DeptDetail(ctx context.Context, deptID uint64, p *principal.Info) (*entity.Dept, error) {
	d, err := s.Repo.FindDeptByID(ctx, deptID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if d == nil || (p.TenantID != 0 && d.TenantID != p.TenantID) {
		return nil, errs.ErrNotFound
	}
	return d, nil
}

// DeleteDept 删除部门（有子部门或用户时拒绝）。
func (s *Service) DeleteDept(ctx context.Context, deptID uint64, p *principal.Info) error {
	// 存在性 + 租户校验（不存在/跨租户一律 404）
	if _, err := s.DeptDetail(ctx, deptID, p); err != nil {
		return err
	}
	hasChild, err := s.Repo.DeptHasChildren(ctx, deptID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if hasChild {
		return errs.ErrHasChildren
	}
	hasUser, err := s.Repo.DeptHasUsers(ctx, deptID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if hasUser {
		return errs.ErrRecordUsed
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Dept{}, deptID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}
