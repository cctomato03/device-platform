package entity

import "time"

// Plugin 插件实例（每种插件在设备上只有一个实例，配置存在 Config 里）。
//
// 字段说明：
//   - Key        插件唯一标识（manifest.id），也是目录名 plugins/<key> 与 plugin-data/<key>
//   - Status     生命周期状态：installed/stopped/running/error
//   - Enabled    是否启用（禁用后不允许启动，重启平台不自动拉起）
//   - AutoStart  平台启动时是否自动拉起
//   - Manifest   插件包清单（JSON 原文），升级/回滚时用于比对
//   - Config     用户配置（JSON，结构化表单值）
type Plugin struct {
	Base
	Key         string     `gorm:"column:key" json:"key"`
	Name        string     `gorm:"column:name" json:"name"`
	Version     string     `gorm:"column:version" json:"version"`
	Vendor      string     `gorm:"column:vendor" json:"vendor"`
	Category    string     `gorm:"column:category" json:"category"`
	Description string     `gorm:"column:description" json:"description"`
	Icon        string     `gorm:"column:icon" json:"icon"`
	Runtime     string     `gorm:"column:runtime" json:"runtime"` // process | docker
	Status      string     `gorm:"column:status" json:"status"`   // installed | stopped | running | error
	Enabled     bool       `gorm:"column:enabled" json:"enabled"`
	AutoStart   bool       `gorm:"column:auto_start" json:"autoStart"`
	Config      string     `gorm:"column:config" json:"config"`
	Manifest    string     `gorm:"column:manifest" json:"manifest"`
	InstallPath string     `gorm:"column:install_path" json:"installPath"`
	DataPath    string     `gorm:"column:data_path" json:"dataPath"`
	PID         int        `gorm:"column:pid" json:"pid"`
	LastError   string     `gorm:"column:last_error" json:"lastError"`
	StartedAt   *time.Time `gorm:"column:started_at" json:"startedAt"`
	InstalledAt time.Time  `gorm:"column:installed_at" json:"installedAt"`
	SizeBytes   int64      `gorm:"column:size_bytes" json:"sizeBytes"`
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (Plugin) TableName() string { return "sys_plugin" }

// AlertRule 告警规则（针对 CPU / 内存 / 磁盘 / 网络）。
type AlertRule struct {
	Base
	Name        string `gorm:"column:name" json:"name"`
	Metric      string `gorm:"column:metric" json:"metric"` // cpu | mem | disk | net_in | net_out
	Operator    string `gorm:"column:operator" json:"operator"`
	Threshold   float64 `gorm:"column:threshold" json:"threshold"`
	DurationSec int    `gorm:"column:duration_sec" json:"durationSec"` // 持续超过阈值多少秒才告警
	Level       int    `gorm:"column:level" json:"level"`              // 1 提示 2 警告 3 严重
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	SilenceSec  int    `gorm:"column:silence_sec" json:"silenceSec"` // 同类告警静默窗口
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (AlertRule) TableName() string { return "sys_alert_rule" }

// AlertRecord 告警记录。
type AlertRecord struct {
	Base
	RuleID    uint64  `gorm:"column:rule_id" json:"ruleId,string"`
	RuleName  string  `gorm:"column:rule_name" json:"ruleName"`
	Metric    string  `gorm:"column:metric" json:"metric"`
	Level     int     `gorm:"column:level" json:"level"`
	Value     float64 `gorm:"column:value" json:"value"`
	Threshold float64 `gorm:"column:threshold" json:"threshold"`
	Message   string  `gorm:"column:message" json:"message"`
	Status    int     `gorm:"column:status" json:"status"` // 1 触发中 2 已恢复 3 已确认
	FiredAt   string  `gorm:"column:fired_at" json:"firedAt"`
	RecoverAt string  `gorm:"column:recover_at" json:"recoverAt"`
	TenantID  uint64  `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (AlertRecord) TableName() string { return "sys_alert_record" }

// MetricSample 指标历史采样（用于趋势图，按保留时长清理）。
type MetricSample struct {
	ID          uint64  `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	SampledAt   string  `gorm:"column:sampled_at" json:"sampledAt"`
	CPUPercent  float64 `gorm:"column:cpu_percent" json:"cpuPercent"`
	MemPercent  float64 `gorm:"column:mem_percent" json:"memPercent"`
	DiskPercent float64 `gorm:"column:disk_percent" json:"diskPercent"`
	NetInBps    float64 `gorm:"column:net_in_bps" json:"netInBps"`
	NetOutBps   float64 `gorm:"column:net_out_bps" json:"netOutBps"`
}

// TableName 表名。
func (MetricSample) TableName() string { return "sys_metric_sample" }

// AccessLog HTTP 访问日志（比操作日志更细，记录所有请求）。
type AccessLog struct {
	ID         uint64 `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	TraceID    string `gorm:"column:trace_id" json:"traceId"`
	IP         string `gorm:"column:ip" json:"ip"`
	Method     string `gorm:"column:method" json:"method"`
	Path       string `gorm:"column:path" json:"path"`
	Status     int    `gorm:"column:status" json:"status"`
	CostMS     int    `gorm:"column:cost_ms" json:"costMs"`
	BytesOut   int    `gorm:"column:bytes_out" json:"bytesOut"`
	UserAgent  string `gorm:"column:user_agent" json:"userAgent"`
	UserID     uint64 `gorm:"column:user_id" json:"userId,string"`
	AccessTime string `gorm:"column:access_time" json:"accessTime"`
}

// TableName 表名。
func (AccessLog) TableName() string { return "sys_access_log" }

// TrafficLog 网络流量采样日志（按周期汇总，用于流量日志与趋势分析）。
type TrafficLog struct {
	ID         uint64  `gorm:"column:id;primaryKey;autoIncrement:false" json:"id,string"`
	Iface      string  `gorm:"column:iface" json:"iface"`
	BytesIn    uint64  `gorm:"column:bytes_in" json:"bytesIn"`
	BytesOut   uint64  `gorm:"column:bytes_out" json:"bytesOut"`
	PacketsIn  uint64  `gorm:"column:packets_in" json:"packetsIn"`
	PacketsOut uint64  `gorm:"column:packets_out" json:"packetsOut"`
	InBps      float64 `gorm:"column:in_bps" json:"inBps"`
	OutBps     float64 `gorm:"column:out_bps" json:"outBps"`
	LoggedAt   string  `gorm:"column:logged_at" json:"loggedAt"`
}

// TableName 表名。
func (TrafficLog) TableName() string { return "sys_traffic_log" }

// StaticRoute 静态路由（保存期望状态，由平台下发到内核）。
type StaticRoute struct {
	Base
	Destination string `gorm:"column:destination" json:"destination"` // 10.0.0.0/8
	Gateway     string `gorm:"column:gateway" json:"gateway"`
	Device      string `gorm:"column:device" json:"device"`
	Metric      int    `gorm:"column:metric" json:"metric"`
	Table       string `gorm:"column:table_name" json:"table"`
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	Applied     bool   `gorm:"column:applied" json:"applied"`
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (StaticRoute) TableName() string { return "sys_static_route" }

// NATRule NAT 策略（SNAT/DNAT，保存期望状态，由平台下发到防火墙）。
type NATRule struct {
	Base
	Name        string `gorm:"column:name" json:"name"`
	Type        string `gorm:"column:rule_type" json:"type"` // snat | dnat
	Protocol    string `gorm:"column:protocol" json:"protocol"`
	SrcIface    string `gorm:"column:src_iface" json:"srcIface"`
	DstIface    string `gorm:"column:dst_iface" json:"dstIface"`
	SrcAddr     string `gorm:"column:src_addr" json:"srcAddr"`
	DstAddr     string `gorm:"column:dst_addr" json:"dstAddr"`
	DstPort     int    `gorm:"column:dst_port" json:"dstPort"`
	ToAddr      string `gorm:"column:to_addr" json:"toAddr"`
	ToPort      int    `gorm:"column:to_port" json:"toPort"`
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	Applied     bool   `gorm:"column:applied" json:"applied"`
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (NATRule) TableName() string { return "sys_nat_rule" }

// SecurePolicy 一体化的安全策略（状态化访问控制，落到 firewalld/nftables）。
type SecurePolicy struct {
	Base
	Name      string `gorm:"column:name" json:"name"`
	Direction string `gorm:"column:direction" json:"direction"` // in | out | forward
	Action    string `gorm:"column:action" json:"action"`       // accept | drop | reject
	Protocol  string `gorm:"column:protocol" json:"protocol"`   // tcp | udp | icmp | any
	SrcAddr   string `gorm:"column:src_addr" json:"srcAddr"`
	DstAddr   string `gorm:"column:dst_addr" json:"dstAddr"`
	Port      int    `gorm:"column:port" json:"port"`
	Priority  int    `gorm:"column:priority" json:"priority"`
	Enabled   bool   `gorm:"column:enabled" json:"enabled"`
	Applied   bool   `gorm:"column:applied" json:"applied"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (SecurePolicy) TableName() string { return "sys_secure_policy" }

// QosRule 带宽限速规则（tc 下发）。
type QosRule struct {
	Base
	Name      string `gorm:"column:name" json:"name"`
	Iface     string `gorm:"column:iface" json:"iface"`
	Direction string `gorm:"column:direction" json:"direction"` // egress | ingress
	RateKbps  int    `gorm:"column:rate_kbps" json:"rateKbps"`
	BurstKb   int    `gorm:"column:burst_kb" json:"burstKb"`
	Target    string `gorm:"column:target" json:"target"` // 目标地址/网段，空表示整卡
	Enabled   bool   `gorm:"column:enabled" json:"enabled"`
	Applied   bool   `gorm:"column:applied" json:"applied"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (QosRule) TableName() string { return "sys_qos_rule" }
