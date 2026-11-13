package service

import (
	"context"
	"fmt"
	"strings"

	"device-platform/internal/model/entity"
)

// 本文件：策略冲突检测。
//
// 设备上的安全策略一旦写错，常见后果是「自己把自己挡在外面」或「策略看起来生效了其实被前面规则覆盖」。
// 平台在保存与下发时做静态检查，把问题以「可读提示」返回给用户，而不是等现场出故障。

// policyConflicts 检测安全策略冲突（保存某个策略时调用）。
//
// 索引到的冲突类型：
//  1. 优先级重复：ip rule / iptables 里同优先级规则顺序不确定，行为不可预期；
//  2. 完全重复：五元组与动作都一样，属于冗余规则；
//  3. 动作冲突：匹配条件相同但动作相反（更危险，容易误判）；
//  4. 被前序规则覆盖：更靠前（priority 更小）的同类规则已经覆盖当前匹配，当前规则不会生效。
func (s *Service) policyConflicts(ctx context.Context, req *PolicyReq) []string {
	var rows []entity.SecurePolicy
	if err := s.Repo.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil
	}
	warnings := make([]string, 0, 4)
	for _, r := range rows {
		if r.ID == req.ID {
			continue // 编辑自己时跳过自身
		}
		// 1. 优先级重复（同方向下）
		if r.Priority == req.Priority && r.Direction == req.Direction {
			warnings = append(warnings,
				fmt.Sprintf("优先级重复：规则「%s」也使用优先级 %d（同方向同优先级时生效顺序不确定，建议改成不同值）", r.Name, r.Priority))
		}
		if !sameMatch(&r, req) {
			continue
		}
		// 2. 完全重复
		if r.Action == req.Action {
			warnings = append(warnings, fmt.Sprintf("重复规则：与「%s」匹配条件与动作完全相同，属于冗余配置", r.Name))
			continue
		}
		// 3. 动作冲突
		warnings = append(warnings, fmt.Sprintf("动作冲突：与「%s」匹配条件相同但动作不同（%s / %s），请确认哪条应生效",
			r.Name, r.Action, req.Action))
	}

	// 4. 被更靠前的规则覆盖（只看已启用的同方向规则）
	for _, r := range rows {
		if r.ID == req.ID || !r.Enabled || r.Direction != req.Direction {
			continue
		}
		if r.Priority < req.Priority && matchesSuperset(&r, req) {
			warnings = append(warnings,
				fmt.Sprintf("可能被覆盖：「%s」（优先级 %d，动作 %s）的匹配范围已覆盖当前规则，且优先级更靠前，当前规则可能永不命中",
					r.Name, r.Priority, r.Action))
		}
	}
	return warnings
}

// natConflicts 检测 NAT 冲突（同一入向匹配映射到不同目标）。
func (s *Service) natConflicts(ctx context.Context, req *NatReq) []string {
	var rows []entity.NATRule
	if err := s.Repo.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil
	}
	warnings := make([]string, 0, 2)
	for _, r := range rows {
		if r.ID == req.ID || r.Type != req.Type {
			continue
		}
		sameDst := r.DstAddr == req.DstAddr && r.DstPort == req.DstPort && r.Protocol == req.Protocol
		if !sameDst {
			continue
		}
		if r.ToAddr != req.ToAddr || r.ToPort != req.ToPort {
			warnings = append(warnings, fmt.Sprintf(
				"目标冲突：规则「%s」已把相同匹配转发到 %s:%d，当前规则转发到 %s:%d，只有一条会生效",
				r.Name, r.ToAddr, r.ToPort, req.ToAddr, req.ToPort))
		}
	}
	return warnings
}

// qosConflicts 检测限速冲突（同网卡同方向重复限速会让实际带宽不可预期）。
func (s *Service) qosConflicts(ctx context.Context, req *QosReq) []string {
	var rows []entity.QosRule
	if err := s.Repo.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil
	}
	warnings := make([]string, 0, 2)
	for _, r := range rows {
		if r.ID == req.ID {
			continue
		}
		if r.Iface == req.Iface && r.Direction == req.Direction {
			warnings = append(warnings, fmt.Sprintf(
				"限速叠加：网卡 %s 的 %s 方向已有规则「%s」（%d Kbps），同一方向多条限速会叠加生效，请确认是否符合预期",
				r.Iface, r.Direction, r.Name, r.RateKbps))
		}
	}
	return warnings
}

// sameMatch 判断两条安全策略的匹配条件是否一致。
func sameMatch(a *entity.SecurePolicy, b *PolicyReq) bool {
	return a.Direction == b.Direction &&
		a.Protocol == b.Protocol &&
		strings.TrimSpace(a.SrcAddr) == strings.TrimSpace(b.SrcAddr) &&
		strings.TrimSpace(a.DstAddr) == strings.TrimSpace(b.DstAddr) &&
		a.Port == b.Port
}

// matchesSuperset 判断已有规则 r 的匹配范围是否覆盖候选规则（r 更宽或相等）。
func matchesSuperset(r *entity.SecurePolicy, req *PolicyReq) bool {
	if r.Protocol != "any" && r.Protocol != req.Protocol {
		return false
	}
	// 地址：r 为空表示任意，否则要求完全相同（不做网段包含计算，避免误报）
	if strings.TrimSpace(r.SrcAddr) != "" && strings.TrimSpace(r.SrcAddr) != strings.TrimSpace(req.SrcAddr) {
		return false
	}
	if strings.TrimSpace(r.DstAddr) != "" && strings.TrimSpace(r.DstAddr) != strings.TrimSpace(req.DstAddr) {
		return false
	}
	if r.Port != 0 && r.Port != req.Port {
		return false
	}
	return true
}

// ApplyPolicyWarnings 下发前汇总所有策略级风险，作为一条结果回显给用户。
func (s *Service) applyPolicyWarnings(ctx context.Context) []string {
	var rows []entity.SecurePolicy
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Order("priority asc").Find(&rows).Error; err != nil {
		return nil
	}
	warnings := make([]string, 0, 4)
	seen := map[int]string{}
	for _, r := range rows {
		if other, ok := seen[r.Priority]; ok {
			warnings = append(warnings, fmt.Sprintf("优先级 %d 被「%s」与「%s」同时使用（同优先级顺序不确定）", r.Priority, other, r.Name))
		} else {
			seen[r.Priority] = r.Name
		}
	}
	// 全量放通 + 后续拒绝的常见误配：前面 accept 覆盖后面 drop
	for i, r := range rows {
		if r.Action != "accept" {
			continue
		}
		for j := i + 1; j < len(rows); j++ {
			n := rows[j]
			if n.Action == "accept" || n.Direction != r.Direction {
				continue
			}
			req := &PolicyReq{
				Name: n.Name, Direction: n.Direction, Action: n.Action, Protocol: n.Protocol,
				SrcAddr: n.SrcAddr, DstAddr: n.DstAddr, Port: n.Port, Priority: n.Priority, Enabled: n.Enabled,
			}
			if matchesSuperset(&r, req) {
				warnings = append(warnings, fmt.Sprintf(
					"「%s」（允许，优先级 %d）覆盖了「%s」（%s，优先级 %d），后者不会生效",
					r.Name, r.Priority, n.Name, n.Action, n.Priority))
			}
		}
	}
	return warnings
}
