package entity

// Dept 部门/公司 sys_dept
type Dept struct {
	Base
	ParentID  uint64 `gorm:"column:parent_id" json:"parentId,string"`
	Ancestors string `gorm:"column:ancestors" json:"ancestors"`
	Name      string `gorm:"column:name" json:"name"`
	Code      string `gorm:"column:code" json:"code"`
	Leader    string `gorm:"column:leader" json:"leader"`
	Phone     string `gorm:"column:phone" json:"phone"`
	Email     string `gorm:"column:email" json:"email"`
	Sort      int    `gorm:"column:sort" json:"sort"`
	Status    int    `gorm:"column:status" json:"status"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Dept) TableName() string { return "sys_dept" }

// Post 岗位 sys_post
type Post struct {
	BaseLite
	Code     string `gorm:"column:code" json:"code"`
	Name     string `gorm:"column:name" json:"name"`
	Sort     int    `gorm:"column:sort" json:"sort"`
	Status   int    `gorm:"column:status" json:"status"`
	TenantID uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Post) TableName() string { return "sys_post" }

// Tenant 租户 sys_tenant
type Tenant struct {
	Base
	Name         string `gorm:"column:name" json:"name"`
	Code         string `gorm:"column:code" json:"code"`
	ContactName  string `gorm:"column:contact_name" json:"contactName"`
	ContactPhone string `gorm:"column:contact_phone" json:"contactPhone"`
	MaxUsers     int    `gorm:"column:max_users" json:"maxUsers"`
	ExpireAt     string `gorm:"column:expire_at" json:"expireAt"`
	Status       int    `gorm:"column:status" json:"status"`
}

func (Tenant) TableName() string { return "sys_tenant" }
