package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/internal/syscmd"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// 本文件：策略路由（ip rule + 路由表）。
//
// 约定：平台只管理优先级 10000-10999 段内的规则，下发时先清理该段的平台规则再整体重建，
// 不会碰系统默认的 32766/32767 等规则，避免把管理口路由搞坏。

// policyPriorityMin/Max 平台独占的 ip rule 优先级区间。
const (
	policyPriorityMin = 10000
	policyPriorityMax = 10999
)

// PolicyRouteReq 策略路由入参。
type PolicyRouteReq struct {
	ID       uint64 `json:"id,string"`
	Name     string `json:"name" binding:"required,max=64"`
	Priority int    `json:"priority"`
	FromAddr string `json:"fromAddr"`
	ToAddr   string `json:"toAddr"`
	FwMark   int    `json:"fwmark"`
	Iif      string `json:"iif"`
	Oif      string `json:"oif"`
	Table    string `json:"table"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

// ListPolicyRoutes 策略路由列表。
func (s *Service) ListPolicyRoutes(ctx context.Context) ([]entity.PolicyRoute, error) {
	var rows []entity.PolicyRoute
	if err := s.Repo.DB.WithContext(ctx).Order("priority asc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// SavePolicyRoute 新建/修改策略路由。
func (s *Service) SavePolicyRoute(ctx context.Context, req *PolicyRouteReq) (*entity.PolicyRoute, error) {
	if req.Priority == 0 {
		req.Priority = policyPriorityMin
	}
	if req.Priority < policyPriorityMin || req.Priority > policyPriorityMax {
		return nil, errs.Invalid(fmt.Sprintf("优先级需在 %d-%d 之间（平台独占区间，避免与系统规则冲突）", policyPriorityMin, policyPriorityMax))
	}
	if req.FromAddr != "" {
		if err := validateCIDR(req.FromAddr); err != nil {
			return nil, errs.Invalid("源地址非法：" + err.Error())
		}
	}
	if req.ToAddr != "" {
		if err := validateCIDR(req.ToAddr); err != nil {
			return nil, errs.Invalid("目的地址非法：" + err.Error())
		}
	}
	if req.Iif != "" && !ifaceRe.MatchString(req.Iif) {
		return nil, errs.Invalid("入接口名非法")
	}
	if req.Oif != "" && !ifaceRe.MatchString(req.Oif) {
		return nil, errs.Invalid("出接口名非法")
	}
	table := defaultStr(req.Table, "main")
	if !ifaceRe.MatchString(table) {
		return nil, errs.Invalid("路由表名非法（例：main、100、mgmt）")
	}
	// 优先级唯一：ip rule 同优先级会互相影响，不允许重复
	var dup entity.PolicyRoute
	err := s.Repo.DB.WithContext(ctx).Where("priority = ? AND id <> ?", req.Priority, req.ID).First(&dup).Error
	if err == nil {
		return nil, errs.Invalid(fmt.Sprintf("优先级 %d 已被规则「%s」占用", req.Priority, dup.Name))
	}

	row := entity.PolicyRoute{
		Base:     entity.Base{ID: id.Next(), Remark: req.Remark},
		Name:     req.Name,
		Priority: req.Priority,
		FromAddr: req.FromAddr,
		ToAddr:   req.ToAddr,
		FwMark:   req.FwMark,
		Iif:      req.Iif,
		Oif:      req.Oif,
		Table:    table,
		Enabled:  req.Enabled,
		TenantID: s.Cfg.Tenant.DefaultID,
	}
	row.CreateTime = nowTime()
	if req.ID == 0 {
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
		return &row, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.PolicyRoute{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "priority": row.Priority, "from_addr": row.FromAddr, "to_addr": row.ToAddr,
		"fwmark": row.FwMark, "iif": row.Iif, "oif": row.Oif, "table_name": row.Table,
		"enabled": row.Enabled, "remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, errs.ErrNotFound
	}
	return &row, nil
}

// DeletePolicyRoute 删除策略路由。
func (s *Service) DeletePolicyRoute(ctx context.Context, routeID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", routeID).Delete(&entity.PolicyRoute{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ApplyPolicyRoutes 下发全部策略路由（先清理平台段规则，再按优先级重建）。
func (s *Service) ApplyPolicyRoutes(ctx context.Context) ([]ApplyResult, error) {
	if err := syscmd.RequireLinux("策略路由下发"); err != nil {
		return []ApplyResult{{Item: "ip rule", OK: false, Message: err.Error()}}, nil
	}
	if !isRoot() || !syscmd.Exists("ip") {
		return []ApplyResult{{Item: "ip rule", OK: false, Message: "需要 root 权限与 iproute2"}}, nil
	}

	results := make([]ApplyResult, 0, 8)

	// 1) 清理平台段内的既有规则
	rules, err := syscmd.Run(ctx, 10*time.Second, "ip", "rule", "show")
	if err != nil {
		return []ApplyResult{mustResultFail("ip rule show", fmt.Errorf("%s", firstNonEmpty(rules.Combined(), err.Error())))}, nil
	}
	removed := 0
	for _, line := range strings.Split(rules.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// 注意：ip rule show 的优先级字段带冒号（如 "10010:"），必须先去掉再解析，
		// 否则清理逻辑会静默跳过，导致「删了配置但内核规则还在」。
		prioStr := strings.TrimSuffix(fields[0], ":")
		prio, perr := strconv.Atoi(prioStr)
		if perr != nil || prio < policyPriorityMin || prio > policyPriorityMax {
			continue
		}
		if _, derr := syscmd.Run(ctx, 10*time.Second, "ip", "rule", "del", "priority", prioStr); derr == nil {
			removed++
		}
	}
	results = append(results, ApplyResult{
		Item: "清理平台旧规则", OK: true,
		Message: fmt.Sprintf("已清理 %d 条（优先级 %d-%d）", removed, policyPriorityMin, policyPriorityMax),
	})

	// 2) 按期望态重建
	var rows []entity.PolicyRoute
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Order("priority asc").Find(&rows).Error; err != nil {
		return results, errs.ErrDB.WithErr(err)
	}
	if len(rows) == 0 {
		results = append(results, ApplyResult{Item: "策略路由", OK: true, Message: "没有启用的规则，已保持为空"})
		return results, nil
	}
	for _, r := range rows {
		args := []string{"rule", "add"}
		if r.FromAddr != "" {
			args = append(args, "from", r.FromAddr)
		}
		if r.ToAddr != "" {
			args = append(args, "to", r.ToAddr)
		}
		if r.FwMark > 0 {
			args = append(args, "fwmark", strconv.Itoa(r.FwMark))
		}
		if r.Iif != "" {
			args = append(args, "iif", r.Iif)
		}
		if r.Oif != "" {
			args = append(args, "oif", r.Oif)
		}
		args = append(args, "lookup", r.Table, "priority", strconv.Itoa(r.Priority))

		res, runErr := syscmd.Run(ctx, 10*time.Second, "ip", args...)
		ok := runErr == nil
		msg := "已下发"
		if !ok {
			msg = firstNonEmpty(res.Combined(), runErr.Error())
		}
		results = append(results, ApplyResult{Item: fmt.Sprintf("%s（prio %d）", r.Name, r.Priority), OK: ok, Message: msg})
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.PolicyRoute{}).Where("id = ?", r.ID).Update("applied", ok).Error
	}

	// 3) 提醒：策略路由只决定「查哪张表」，表里还要有路由（静态路由页维护）
	results = append(results, ApplyResult{
		Item: "提示", OK: true,
		Message: "策略路由只是选表；请在「静态路由」页为对应的 table 配置路由（如 default via <网关> table " + rows[0].Table + "）",
	})
	return results, nil
}

// PolicyRuleStatus 当前内核里的策略规则（只读展示）。
func (s *Service) PolicyRuleStatus(ctx context.Context) ([]string, error) {
	if !syscmd.IsLinux() || !syscmd.Exists("ip") {
		return nil, nil
	}
	res, err := syscmd.Run(ctx, 8*time.Second, "ip", "rule", "show")
	if err != nil {
		return nil, nil
	}
	return splitLines(res.Stdout), nil
}
