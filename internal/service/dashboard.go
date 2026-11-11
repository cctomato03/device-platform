package service

import (
	"context"
	"time"
)

// DashboardTrendPoint 折线图数据点。
type DashboardTrendPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// DashboardRecentLogin 最近登录记录（首页展示用精简结构）。
type DashboardRecentLogin struct {
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Status    int    `json:"status"`
	LoginTime string `json:"loginTime"`
}

// DashboardRecentOper 最近操作记录。
type DashboardRecentOper struct {
	Title    string `json:"title"`
	UserName string `json:"userName"`
	Method   string `json:"requestUri"`
	HTTPVerb string `json:"httpMethod"`
	Status   int    `json:"status"`
	CostMS   int    `json:"costMs"`
	OperTime string `json:"operTime"`
}

// DashboardSummary 首页概览。
type DashboardSummary struct {
	Users        int64                  `json:"users"`
	Roles        int64                  `json:"roles"`
	Depts        int64                  `json:"depts"`
	Posts        int64                  `json:"posts"`
	Files        int64                  `json:"files"`
	FileSize     int64                  `json:"fileSize"`
	Tenants      int64                  `json:"tenants"`
	TodayLogins  int64                  `json:"todayLogins"`
	TodayOpers   int64                  `json:"todayOpers"`
	OnlineCount  int                    `json:"onlineCount"`
	LoginTrend   []DashboardTrendPoint  `json:"loginTrend"`
	RecentLogins []DashboardRecentLogin `json:"recentLogins"`
	RecentOpers  []DashboardRecentOper  `json:"recentOpers"`
	GeneratedAt  string                 `json:"generatedAt"`
}

// Dashboard 首页数据汇总。
func (s *Service) Dashboard(ctx context.Context, tenantID uint64) (*DashboardSummary, error) {
	counts, err := s.Repo.DashboardCounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	trend, err := s.Repo.LoginTrend(ctx, 7)
	if err != nil {
		return nil, err
	}
	points := make([]DashboardTrendPoint, 0, len(trend))
	for _, p := range trend {
		points = append(points, DashboardTrendPoint{Date: p.Date, Count: p.Count})
	}

	logins, err := s.Repo.RecentLogins(ctx, 8)
	if err != nil {
		return nil, err
	}
	recentLogins := make([]DashboardRecentLogin, 0, len(logins))
	for _, l := range logins {
		recentLogins = append(recentLogins, DashboardRecentLogin{
			Username:  l.Username,
			IP:        l.IP,
			Status:    l.Status,
			LoginTime: l.LoginTime,
		})
	}

	opers, err := s.Repo.RecentOpers(ctx, 8)
	if err != nil {
		return nil, err
	}
	recentOpers := make([]DashboardRecentOper, 0, len(opers))
	for _, o := range opers {
		recentOpers = append(recentOpers, DashboardRecentOper{
			Title:    o.Title,
			UserName: o.UserName,
			Method:   o.RequestURI,
			HTTPVerb: o.HTTPMethod,
			Status:   o.Status,
			CostMS:   o.CostMS,
			OperTime: o.OperTime,
		})
	}

	// 只取数量：避免为了一个数字把在线会话与用户表全查一遍
	online := s.OnlineCount(ctx)

	return &DashboardSummary{
		Users:        counts.Users,
		Roles:        counts.Roles,
		Depts:        counts.Depts,
		Posts:        counts.Posts,
		Files:        counts.Files,
		FileSize:     counts.FileSize,
		Tenants:      counts.Tenants,
		TodayLogins:  counts.TodayLogins,
		TodayOpers:   counts.TodayOpers,
		OnlineCount:  online,
		LoginTrend:   points,
		RecentLogins: recentLogins,
		RecentOpers:  recentOpers,
		GeneratedAt:  time.Now().Format(time.RFC3339),
	}, nil
}
