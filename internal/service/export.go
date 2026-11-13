// Excel 导出 / 导入。
//
// 设计取舍：
//   - 导出复用列表查询条件，但绕过分页（有上限 maxExportRows），避免前端拉全量数据；
//   - 导入逐行校验并汇总错误（第 N 行：原因），单行失败不影响其它行，返回成功/失败明细；
//   - 导入走的是与手工新增同一个 service 方法，权限、唯一键、bcrypt 加密等规则完全一致。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"device-platform/internal/excel"
	"device-platform/internal/model/entity"
	"device-platform/internal/model/scope"
	"device-platform/internal/principal"
	"device-platform/internal/repository"
	"device-platform/pkg/errs"
	"device-platform/pkg/jsonx"
)

// maxExportRows 单次导出上限（防止一次导出把内存打满）。
const maxExportRows = 50000

// ExportUsers 导出用户列表为 xlsx。
func (s *Service) ExportUsers(ctx context.Context, q *repository.UserQuery, p *principal.Info) ([]byte, string, error) {
	var deptIDs []uint64
	if q.DeptID > 0 {
		ids, err := s.Repo.SubtreeDeptIDs(ctx, q.DeptID)
		if err != nil {
			return nil, "", errs.ErrDB.WithErr(err)
		}
		deptIDs = ids
	}
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.User{}).Scopes(scope.Tenant(p.TenantID))
	if q.Username != "" {
		tx = tx.Where("username LIKE ?", q.Username+"%")
	}
	if q.Mobile != "" {
		tx = tx.Where("mobile LIKE ?", q.Mobile+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if len(deptIDs) > 0 {
		tx = tx.Where("dept_id IN ?", deptIDs)
	}
	users := make([]entity.User, 0, 256)
	if err := tx.Order("id ASC").Limit(maxExportRows).Find(&users).Error; err != nil {
		return nil, "", errs.ErrDB.WithErr(err)
	}

	deptNames, _ := s.deptNameMap(ctx, p.TenantID)
	roleNames, _ := s.userRoleNames(ctx, users)

	rows := make([][]any, 0, len(users))
	for _, u := range users {
		rows = append(rows, []any{
			u.Username, u.Nickname, u.Mobile, u.Email, sexText(u.Sex),
			deptNames[u.DeptID], strings.Join(roleNames[u.ID], ","), statusText(u.Status),
			formatTimePtr(u.CreateTime), u.Remark,
		})
	}
	headers := []string{"用户名", "昵称", "手机号", "邮箱", "性别", "部门", "角色", "状态", "创建时间", "备注"}
	data, err := excel.Build("用户", headers, rows)
	if err != nil {
		return nil, "", errs.ErrInternal.WithErr(err)
	}
	return data, fmt.Sprintf("用户列表_%s.xlsx", time.Now().Format("20060102_150405")), nil
}

// UserImportResult 导入结果。
type UserImportResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// ImportUsers 从 xlsx 导入用户。
// 表头顺序：用户名* | 昵称 | 手机号 | 邮箱 | 性别 | 部门编码 | 角色编码 | 状态 | 初始密码 | 备注
func (s *Service) ImportUsers(ctx context.Context, r io.Reader, p *principal.Info) (*UserImportResult, error) {
	rows, err := excel.Read(r, 2000)
	if err != nil {
		return nil, errs.InvalidErr(fmt.Errorf("文件解析失败：%v", err))
	}
	if len(rows) <= 1 {
		return nil, errs.InvalidErr(fmt.Errorf("文件没有数据行"))
	}
	deptCodes, _ := s.deptCodeMap(ctx, p.TenantID)
	roleCodes, _ := s.roleCodeMap(ctx, p.TenantID)

	res := &UserImportResult{Errors: []string{}}
	for i, row := range rows[1:] {
		lineNo := i + 2
		res.Total++
		cell := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		username := cell(0)
		if username == "" {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：用户名为空", lineNo))
			continue
		}
		req := &CreateUserReq{
			Username: username,
			Nickname: cell(1),
			Mobile:   cell(2),
			Email:    cell(3),
			Sex:      parseSex(cell(4)),
			Status:   parseStatus(cell(7)),
			Remark:   cell(9),
		}
		if deptCode := cell(5); deptCode != "" {
			if id, ok := deptCodes[deptCode]; ok {
				req.DeptID = jsonx.Uint64(id)
			} else {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：部门编码 %s 不存在", lineNo, deptCode))
				continue
			}
		}
		if rc := cell(6); rc != "" {
			ids, missing := resolveCodes(rc, roleCodes)
			if len(missing) > 0 {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：角色编码 %s 不存在", lineNo, strings.Join(missing, ",")))
				continue
			}
			req.RoleIDs = jsonx.Uint64Slice(ids)
		}
		if pwd := cell(8); pwd != "" {
			req.Password = pwd
		}
		if _, err := s.CreateUser(ctx, req, p); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", lineNo, errText(err)))
			continue
		}
		res.Success++
	}
	return res, nil
}

// UserImportTemplate 生成导入模板（含表头 + 一行示例）。
func (s *Service) UserImportTemplate() ([]byte, error) {
	headers := []string{"用户名", "昵称", "手机号", "邮箱", "性别", "部门编码", "角色编码", "状态", "初始密码", "备注"}
	rows := [][]any{
		{"zhangsan", "张三", "13800000001", "zhangsan@example.com", "男", "", "", "正常", "Abc123456", "示例行，导入前请删除"},
	}
	data, err := excel.Build("用户导入模板", headers, rows)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return data, nil
}

// ExportOperLogs 导出操作日志为 xlsx。
func (s *Service) ExportOperLogs(ctx context.Context, q *LogQuery, p *principal.Info) ([]byte, string, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.OperLog{})
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if q.Username != "" {
		tx = tx.Where("oper_name LIKE ?", q.Username+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.StartAt != "" {
		tx = tx.Where("oper_time >= ?", q.StartAt)
	}
	if q.EndAt != "" {
		tx = tx.Where("oper_time <= ?", q.EndAt)
	}
	logs := make([]entity.OperLog, 0, 512)
	if err := tx.Order("id DESC").Limit(maxExportRows).Find(&logs).Error; err != nil {
		return nil, "", errs.ErrDB.WithErr(err)
	}
	rows := make([][]any, 0, len(logs))
	for _, l := range logs {
		st := "成功"
		if l.Status != 1 {
			st = "失败"
		}
		rows = append(rows, []any{
			l.OperTime, l.Title, l.UserName, l.HTTPMethod + " " + l.RequestURI,
			l.IP, l.CostMS, st, l.ErrorMsg, l.Diff,
		})
	}
	headers := []string{"操作时间", "操作模块", "操作人", "请求", "IP", "耗时(ms)", "状态", "错误信息", "字段变更"}
	data, err := excel.Build("操作日志", headers, rows)
	if err != nil {
		return nil, "", errs.ErrInternal.WithErr(err)
	}
	return data, fmt.Sprintf("操作日志_%s.xlsx", time.Now().Format("20060102_150405")), nil
}

// ---------- 内部工具 ----------

// userRoleNames 批量取用户角色名（一次查询，避免 N+1）。
func (s *Service) userRoleNames(ctx context.Context, users []entity.User) (map[uint64][]string, error) {
	out := make(map[uint64][]string, len(users))
	if len(users) == 0 {
		return out, nil
	}
	ids := make([]uint64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	var rows []struct {
		UserID uint64
		Name   string
	}
	if err := s.Repo.DB.WithContext(ctx).Table("sys_user_role ur").
		Select("ur.user_id AS user_id, r.name AS name").
		Joins("JOIN sys_role r ON r.id = ur.role_id AND r.deleted = 0").
		Where("ur.user_id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], r.Name)
	}
	return out, nil
}

// deptCodeMap 部门编码 → ID。
func (s *Service) deptCodeMap(ctx context.Context, tenantID uint64) (map[string]uint64, error) {
	var rows []struct {
		ID   uint64
		Code string
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Dept{}).
		Scopes(scope.Tenant(tenantID)).Select("id, code").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]uint64, len(rows))
	for _, r := range rows {
		m[r.Code] = r.ID
	}
	return m, nil
}

// roleCodeMap 角色编码 → ID。
func (s *Service) roleCodeMap(ctx context.Context, tenantID uint64) (map[string]uint64, error) {
	var rows []struct {
		ID   uint64
		Code string
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).
		Scopes(scope.Tenant(tenantID)).Select("id, code").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]uint64, len(rows))
	for _, r := range rows {
		m[r.Code] = r.ID
	}
	return m, nil
}

// resolveCodes 逗号分隔的编码 → ID 列表，返回缺失编码。
func resolveCodes(codes string, dict map[string]uint64) ([]uint64, []string) {
	parts := strings.FieldsFunc(codes, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == ' ' })
	ids := make([]uint64, 0, len(parts))
	missing := make([]string, 0)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if id, ok := dict[p]; ok {
			ids = append(ids, id)
		} else {
			missing = append(missing, p)
		}
	}
	return ids, missing
}

func sexText(sex int) string {
	switch sex {
	case 1:
		return "男"
	case 2:
		return "女"
	default:
		return "未知"
	}
}

func parseSex(s string) int {
	switch strings.TrimSpace(s) {
	case "男", "1":
		return 1
	case "女", "2":
		return 2
	default:
		return 0
	}
}

func statusText(status int) string {
	if status == 1 {
		return "正常"
	}
	return "停用"
}

func parseStatus(s string) int {
	switch strings.TrimSpace(s) {
	case "停用", "禁用", "0":
		return 0
	default:
		return 1
	}
}

// formatTimePtr 时间 → 字符串（导出用）。
func formatTimePtr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// errText 提取业务错误文案（导入明细展示用）。
func errText(err error) string {
	var ae *errs.AppError
	if errors.As(err, &ae) {
		// 优先用底层原因（如"密码强度不足：需包含字母和数字"），比笼统的"参数校验失败"更有用
		if ae.Err != nil {
			return ae.Err.Error()
		}
		if ae.Msg != "" {
			return ae.Msg
		}
	}
	return err.Error()
}
