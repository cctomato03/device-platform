package entity

import "time"

// User 用户表 sys_user
type User struct {
	Base
	Username      string     `gorm:"column:username" json:"username"`
	Nickname      string     `gorm:"column:nickname" json:"nickname"`
	Password      string     `gorm:"column:password" json:"-"`
	Salt          string     `gorm:"column:salt" json:"-"`
	DeptID        uint64     `gorm:"column:dept_id" json:"deptId,string"`
	Mobile        string     `gorm:"column:mobile" json:"mobile"`
	Email         string     `gorm:"column:email" json:"email"`
	Sex           int        `gorm:"column:sex" json:"sex"`
	Avatar        string     `gorm:"column:avatar" json:"avatar"`
	Status        int        `gorm:"column:status" json:"status"`
	LastLoginIP   string     `gorm:"column:last_login_ip" json:"lastLoginIp"`
	LastLoginTime *time.Time `gorm:"column:last_login_time" json:"lastLoginTime"`
	PwdUpdateTime *time.Time `gorm:"column:pwd_update_time" json:"pwdUpdateTime"`
	FailCount     int        `gorm:"column:fail_count" json:"-"`
	LockedUntil   *time.Time `gorm:"column:locked_until" json:"-"`
	IsSuper       bool       `gorm:"column:is_super" json:"isSuper"`
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (User) TableName() string { return "sys_user" }

// UserRole 用户-角色关联 sys_user_role
type UserRole struct {
	UserID uint64 `gorm:"column:user_id;primaryKey" json:"userId,string"`
	RoleID uint64 `gorm:"column:role_id;primaryKey" json:"roleId,string"`
}

func (UserRole) TableName() string { return "sys_user_role" }

// UserPost 用户-岗位关联 sys_user_post
type UserPost struct {
	UserID uint64 `gorm:"column:user_id;primaryKey" json:"userId,string"`
	PostID uint64 `gorm:"column:post_id;primaryKey" json:"postId,string"`
}

func (UserPost) TableName() string { return "sys_user_post" }
