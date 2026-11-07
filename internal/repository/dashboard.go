package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
)

// DashboardCounts 首页统计计数。
type DashboardCounts struct {
	Users       int64
	Roles       int64
	Depts       int64
	Posts       int64
	Files       int64
	Tenants     int64
	FileSize    int64
	TodayLogins int64
	TodayOpers  int64
}

// DayCount 按天统计。
type DayCount struct {
	Date  string
	Count int64
}

// DashboardCounts 汇总首页统计（租户维度的表按 tenantID 过滤，0 表示不限）。
func (r *Repo) DashboardCounts(ctx context.Context, tenantID uint64) (*DashboardCounts, error) {
	c := &DashboardCounts{}
	scoped := func(model interface{}) *gorm.DB {
		q := r.DB.WithContext(ctx).Model(model)
		if tenantID > 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		return q
	}

	counts := []struct {
		model interface{}
		dest  *int64
	}{
		{&entity.User{}, &c.Users},
		{&entity.Role{}, &c.Roles},
		{&entity.Dept{}, &c.Depts},
		{&entity.Post{}, &c.Posts},
		{&entity.File{}, &c.Files},
	}
	for _, item := range counts {
		if err := scoped(item.model).Count(item.dest).Error; err != nil {
			return nil, err
		}
	}

	// sys_tenant 自身没有 tenant_id 列，独立统计
	if err := r.DB.WithContext(ctx).Model(&entity.Tenant{}).Count(&c.Tenants).Error; err != nil {
		return nil, err
	}

	if err := scoped(&entity.File{}).Select("COALESCE(SUM(size),0)").Scan(&c.FileSize).Error; err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	loginQ := r.DB.WithContext(ctx).Model(&entity.LoginLog{}).Where("login_time LIKE ?", today+"%")
	operQ := r.DB.WithContext(ctx).Model(&entity.OperLog{}).Where("oper_time LIKE ?", today+"%")
	if tenantID > 0 {
		// 两张日志表都有 tenant_id，非超管只看本租户
		loginQ = loginQ.Where("tenant_id = ?", tenantID)
		operQ = operQ.Where("tenant_id = ?", tenantID)
	}
	_ = loginQ.Count(&c.TodayLogins).Error
	_ = operQ.Count(&c.TodayOpers).Error

	return c, nil
}

// LoginTrend 最近 n 天登录次数（登录日志表按天聚合）。
func (r *Repo) LoginTrend(ctx context.Context, days int) ([]DayCount, error) {
	if days <= 0 {
		days = 7
	}
	start := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")

	type row struct {
		Day   string
		Total int64
	}
	var rows []row
	err := r.DB.WithContext(ctx).Model(&entity.LoginLog{}).
		Select("substr(login_time,1,10) AS day, COUNT(*) AS total").
		Where("login_time >= ?", start).
		Group("day").Order("day ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	m := make(map[string]int64, len(rows))
	for _, it := range rows {
		m[it.Day] = it.Total
	}
	out := make([]DayCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, DayCount{Date: d, Count: m[d]})
	}
	return out, nil
}

// RecentLogins 最近登录日志。
func (r *Repo) RecentLogins(ctx context.Context, limit int) ([]entity.LoginLog, error) {
	var list []entity.LoginLog
	q := r.DB.WithContext(ctx).Model(&entity.LoginLog{}).Order("login_time DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&list).Error
	return list, err
}

// RecentOpers 最近操作日志。
func (r *Repo) RecentOpers(ctx context.Context, limit int) ([]entity.OperLog, error) {
	var list []entity.OperLog
	q := r.DB.WithContext(ctx).Model(&entity.OperLog{}).Order("oper_time DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&list).Error
	return list, err
}
