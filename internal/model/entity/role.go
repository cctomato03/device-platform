package entity

// 数据权限范围
const (
	DataScopeAll        = 1 // 全部数据
	DataScopeDept       = 2 // 本部门
	DataScopeDeptAndSub = 3 // 本部门及以下
	DataScopeSelf       = 4 // 仅本人
	DataScopeCustom     = 5 // 自定义部门
)

// 菜单类型
const (
	MenuTypeDir    = 1 // 目录
	MenuTypeMenu   = 2 // 菜单
	MenuTypeButton = 3 // 按钮/操作点
	MenuTypeAPI    = 4 // 接口权限点
)

// Role 角色表 sys_role
type Role struct {
	Base
	Name      string `gorm:"column:name" json:"name"`
	Code      string `gorm:"column:code" json:"code"`
	Sort      int    `gorm:"column:sort" json:"sort"`
	DataScope int    `gorm:"column:data_scope" json:"dataScope"`
	Status    int    `gorm:"column:status" json:"status"`
	IsBuiltin bool   `gorm:"column:is_builtin" json:"isBuiltin"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Role) TableName() string { return "sys_role" }

// Menu 菜单/权限点 sys_menu（目录/菜单/按钮/接口四类合一）
type Menu struct {
	Base
	ParentID   uint64 `gorm:"column:parent_id" json:"parentId,string"`
	Name       string `gorm:"column:name" json:"name"`
	NameEn     string `gorm:"column:name_en" json:"nameEn"`
	Type       int    `gorm:"column:type" json:"type"`
	Permission string `gorm:"column:permission" json:"permission"`
	Path       string `gorm:"column:path" json:"path"`
	Component  string `gorm:"column:component" json:"component"`
	Redirect   string `gorm:"column:redirect" json:"redirect"`
	Icon       string `gorm:"column:icon" json:"icon"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Visible    bool   `gorm:"column:visible" json:"visible"`
	KeepAlive  bool   `gorm:"column:keep_alive" json:"keepAlive"`
	Affix      bool   `gorm:"column:affix" json:"affix"`
	IsFrame    bool   `gorm:"column:is_frame" json:"isFrame"`
	IsPublic   bool   `gorm:"column:is_public" json:"isPublic"`
	APIMethod  string `gorm:"column:api_method" json:"apiMethod"`
	APIPath    string `gorm:"column:api_path" json:"apiPath"`
	Status     int    `gorm:"column:status" json:"status"`
	TenantID   uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Menu) TableName() string { return "sys_menu" }

// RoleMenu 角色-权限点关联 sys_role_menu
type RoleMenu struct {
	RoleID uint64 `gorm:"column:role_id;primaryKey" json:"roleId,string"`
	MenuID uint64 `gorm:"column:menu_id;primaryKey" json:"menuId,string"`
}

func (RoleMenu) TableName() string { return "sys_role_menu" }

// RoleDept 角色自定义数据权限部门 sys_role_dept
type RoleDept struct {
	RoleID uint64 `gorm:"column:role_id;primaryKey" json:"roleId,string"`
	DeptID uint64 `gorm:"column:dept_id;primaryKey" json:"deptId,string"`
}

func (RoleDept) TableName() string { return "sys_role_dept" }
