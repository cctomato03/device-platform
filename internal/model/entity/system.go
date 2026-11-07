package entity

// DictType 字典类型 sys_dict_type
type DictType struct {
	BaseLite
	Name      string `gorm:"column:name" json:"name"`
	Code      string `gorm:"column:code" json:"code"`
	Status    int    `gorm:"column:status" json:"status"`
	IsBuiltin bool   `gorm:"column:is_builtin" json:"isBuiltin"`
}

func (DictType) TableName() string { return "sys_dict_type" }

// DictData 字典数据 sys_dict_data
type DictData struct {
	BaseLite
	DictCode  string `gorm:"column:dict_code" json:"dictCode"`
	Label     string `gorm:"column:label" json:"label"`
	Value     string `gorm:"column:value" json:"value"`
	Sort      int    `gorm:"column:sort" json:"sort"`
	TagType   string `gorm:"column:tag_type" json:"tagType"`
	CssClass  string `gorm:"column:css_class" json:"cssClass"`
	IsDefault bool   `gorm:"column:is_default" json:"isDefault"`
	Status    int    `gorm:"column:status" json:"status"`
}

func (DictData) TableName() string { return "sys_dict_data" }

// Config 系统参数 sys_config（key 为保留字，列名用 config_key）
type Config struct {
	Base
	Name      string `gorm:"column:name" json:"name"`
	Key       string `gorm:"column:config_key" json:"key"`
	Value     string `gorm:"column:value" json:"value"`
	ValueType string `gorm:"column:value_type" json:"valueType"`
	IsBuiltin bool   `gorm:"column:is_builtin" json:"isBuiltin"`
	IsPublic  bool   `gorm:"column:is_public" json:"isPublic"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (Config) TableName() string { return "sys_config" }

// File 文件元数据 sys_file
type File struct {
	Base
	StorageType string `gorm:"column:storage_type" json:"storageType"`
	Bucket      string `gorm:"column:bucket" json:"bucket"`
	ObjectKey   string `gorm:"column:object_key" json:"objectKey"`
	OriginName  string `gorm:"column:origin_name" json:"originName"`
	Ext         string `gorm:"column:ext" json:"ext"`
	Mime        string `gorm:"column:mime" json:"mime"`
	Size        int64  `gorm:"column:size" json:"size"`
	Hash        string `gorm:"column:hash" json:"hash"`
	URL         string `gorm:"column:url" json:"url"`
	IsPrivate   bool   `gorm:"column:is_private" json:"isPrivate"`
	BizType     string `gorm:"column:biz_type" json:"bizType"`
	BizID       uint64 `gorm:"column:biz_id" json:"bizId,string"`
	Status      int    `gorm:"column:status" json:"status"`
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

func (File) TableName() string { return "sys_file" }

// Job 定时任务 sys_job
type Job struct {
	Base
	Name       string `gorm:"column:name" json:"name"`
	Handler    string `gorm:"column:handler" json:"handler"`
	JobType    int    `gorm:"column:job_type" json:"jobType"`
	Target     string `gorm:"column:target" json:"target"`
	HTTPMethod string `gorm:"column:http_method" json:"httpMethod"`
	Cron       string `gorm:"column:cron" json:"cron"`
	Params     string `gorm:"column:params" json:"params"`
	Concurrent int    `gorm:"column:concurrent" json:"concurrent"`
	TimeoutSec int    `gorm:"column:timeout_sec" json:"timeoutSec"`
	RetryTimes int    `gorm:"column:retry_times" json:"retryTimes"`
	Status     int    `gorm:"column:status" json:"status"`
	NextRunAt  string `gorm:"column:next_run_at" json:"nextRunAt"`
}

func (Job) TableName() string { return "sys_job" }

// JobLog 任务执行日志 sys_job_log
type JobLog struct {
	ID          uint64 `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	JobID       uint64 `gorm:"column:job_id" json:"jobId,string"`
	JobName     string `gorm:"column:job_name" json:"jobName"`
	TriggerType int    `gorm:"column:trigger_type" json:"triggerType"`
	Status      int    `gorm:"column:status" json:"status"`
	StartTime   string `gorm:"column:start_time" json:"startTime"`
	EndTime     string `gorm:"column:end_time" json:"endTime"`
	CostMS      int    `gorm:"column:cost_ms" json:"costMs"`
	Result      string `gorm:"column:result" json:"result"`
	CreateTime  string `gorm:"column:create_time" json:"createTime"`
}

func (JobLog) TableName() string { return "sys_job_log" }

// LoginLog 登录日志 sys_login_log
type LoginLog struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	Username  string `gorm:"column:username" json:"username"`
	LoginType int    `gorm:"column:login_type" json:"loginType"`
	Status    int    `gorm:"column:status" json:"status"`
	Msg       string `gorm:"column:msg" json:"msg"`
	IP        string `gorm:"column:ip" json:"ip"`
	Location  string `gorm:"column:location" json:"location"`
	UserAgent string `gorm:"column:user_agent" json:"userAgent"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	LoginTime string `gorm:"column:login_time" json:"loginTime"`
}

func (LoginLog) TableName() string { return "sys_login_log" }

// OperLog 操作日志 sys_oper_log
type OperLog struct {
	ID         uint64 `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	TraceID    string `gorm:"column:trace_id" json:"traceId"`
	Title      string `gorm:"column:title" json:"title"`
	Method     string `gorm:"column:method" json:"method"`
	RequestURI string `gorm:"column:request_uri" json:"requestUri"`
	HTTPMethod string `gorm:"column:http_method" json:"httpMethod"`
	UserID     uint64 `gorm:"column:oper_user_id" json:"userId,string"`
	TenantID   uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	UserName   string `gorm:"column:oper_name" json:"userName"`
	DeptName   string `gorm:"column:dept_name" json:"deptName"`
	IP         string `gorm:"column:oper_ip" json:"ip"`
	Location   string `gorm:"column:oper_location" json:"location"`
	Params     string `gorm:"column:params" json:"params"`
	Diff       string `gorm:"column:diff" json:"diff"` // 字段级变更差异（JSON 数组）
	Result     string `gorm:"column:result" json:"result"`
	Status     int    `gorm:"column:status" json:"status"`
	ErrorMsg   string `gorm:"column:error_msg" json:"errorMsg"`
	CostMS     int    `gorm:"column:cost_ms" json:"costMs"`
	OperTime   string `gorm:"column:oper_time" json:"operTime"`
}

func (OperLog) TableName() string { return "sys_oper_log" }
