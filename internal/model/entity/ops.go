package entity

// 本文件：网络/设备运维类实体（网卡配置、策略路由、DHCP、DNS 服务、动态路由、SNMP、插件源）。
//
// 统一设计：先存「期望态」，再由显式「下发」动作渲染成配置并调用系统命令，
// 每条下发都逐项回报 {item, ok, message}；平台只写自己命名的配置文件，不动系统既有配置。

// IfaceConfig 网卡配置（渲染为 netplan 片段：以太网 / 网桥 / 端口聚合）。
type IfaceConfig struct {
	Base
	// Name 网卡/网桥/聚合名（eth0、br0、bond0）
	Name string `gorm:"column:name" json:"name"`
	// Kind ethernet | bridge | bond
	Kind string `gorm:"column:kind" json:"kind"`
	// Dhcp4 是否由 DHCP 获取地址
	Dhcp4 bool `gorm:"column:dhcp4" json:"dhcp4"`
	// Addresses 静态地址，逗号分隔 CIDR，如 192.168.1.10/24,10.0.0.5/24
	Addresses string `gorm:"column:addresses" json:"addresses"`
	// Gateway 默认网关（静态地址时可选）
	Gateway string `gorm:"column:gateway" json:"gateway"`
	// MTU 链路 MTU，0 表示不设置
	MTU int `gorm:"column:mtu" json:"mtu"`
	// Members 桥/聚合成员网卡，逗号分隔
	Members string `gorm:"column:members" json:"members"`
	// BondMode 聚合模式：balance-rr / active-backup / 802.3ad / broadcast
	BondMode string `gorm:"column:bond_mode" json:"bondMode"`
	// STP 网桥是否开启生成树
	STP bool `gorm:"column:stp" json:"stp"`
	// Enabled 是否纳入平台渲染（关闭后该网卡不再被平台管理）
	Enabled bool `gorm:"column:enabled" json:"enabled"`
	// Applied 最近一次下发是否成功
	Applied  bool   `gorm:"column:applied" json:"applied"`
	TenantID uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (IfaceConfig) TableName() string { return "sys_iface_config" }

// PolicyRoute 策略路由（ip rule + 路由表）。
type PolicyRoute struct {
	Base
	Name string `gorm:"column:name" json:"name"`
	// Priority ip rule 优先级，平台约定使用 10000-10999 段
	Priority int `gorm:"column:priority" json:"priority"`
	// FromAddr 源地址/网段，空表示任意
	FromAddr string `gorm:"column:from_addr" json:"fromAddr"`
	// ToAddr 目的地址/网段，空表示任意
	ToAddr string `gorm:"column:to_addr" json:"toAddr"`
	// FwMark 防火墙标记（0 表示不匹配）
	FwMark int `gorm:"column:fwmark" json:"fwmark"`
	// Iif / Oif 入/出网卡
	Iif string `gorm:"column:iif" json:"iif"`
	Oif string `gorm:"column:oif" json:"oif"`
	// Table 目标路由表名（需在静态路由里维护同表路由）
	Table    string `gorm:"column:table_name" json:"table"`
	Enabled  bool   `gorm:"column:enabled" json:"enabled"`
	Applied  bool   `gorm:"column:applied" json:"applied"`
	TenantID uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (PolicyRoute) TableName() string { return "sys_policy_route" }

// DhcpConfig DHCP 服务配置（单行，id=1；底层用 dnsmasq 提供）。
type DhcpConfig struct {
	Base
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	Iface       string `gorm:"column:iface" json:"iface"`
	RangeStart  string `gorm:"column:range_start" json:"rangeStart"`
	RangeEnd    string `gorm:"column:range_end" json:"rangeEnd"`
	Netmask     string `gorm:"column:netmask" json:"netmask"`
	LeaseHours  int    `gorm:"column:lease_hours" json:"leaseHours"`
	Gateway     string `gorm:"column:gateway" json:"gateway"`
	DNS         string `gorm:"column:dns" json:"dns"`
	Domain      string `gorm:"column:domain" json:"domain"`
	// StaticBindings 静态绑定 JSON：[{"mac":"aa:bb:..","ip":"192.168.1.10","hostname":"printer"}]
	StaticBindings string `gorm:"column:static_bindings" json:"staticBindings"`
	Applied        bool   `gorm:"column:applied" json:"applied"`
	TenantID       uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (DhcpConfig) TableName() string { return "sys_dhcp_config" }

// DnsServerConfig DNS 服务配置（单行，id=1；dnsmasq 提供本地解析与上游转发）。
type DnsServerConfig struct {
	Base
	Enabled    bool   `gorm:"column:enabled" json:"enabled"`
	ListenAddr string `gorm:"column:listen_addr" json:"listenAddr"`
	// Upstreams 上游 DNS，逗号分隔
	Upstreams string `gorm:"column:upstreams" json:"upstreams"`
	// Records 本地解析记录 JSON：[{"name":"nas.local","ip":"192.168.1.20"}]
	Records   string `gorm:"column:records" json:"records"`
	CacheSize int    `gorm:"column:cache_size" json:"cacheSize"`
	Applied   bool   `gorm:"column:applied" json:"applied"`
	TenantID  uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (DnsServerConfig) TableName() string { return "sys_dns_server_config" }

// RoutingConfig 动态路由配置（protocol=bgp|ospf，底层 FRR）。
type RoutingConfig struct {
	Base
	// Protocol bgp | ospf
	Protocol string `gorm:"column:protocol" json:"protocol"`
	Enabled  bool   `gorm:"column:enabled" json:"enabled"`
	// ASN BGP 本地 AS 号
	ASN int `gorm:"column:asn" json:"asn"`
	// RouterID 路由器 ID
	RouterID string `gorm:"column:router_id" json:"routerId"`
	// Neighbors BGP 邻居 JSON：[{"addr":"10.0.0.2","remoteAs":65002}]
	Neighbors string `gorm:"column:neighbors" json:"neighbors"`
	// Networks 需要宣告/启用的网段，逗号分隔 CIDR
	Networks string `gorm:"column:networks" json:"networks"`
	// Area OSPF 区域，默认 0
	Area     string `gorm:"column:area" json:"area"`
	Applied  bool   `gorm:"column:applied" json:"applied"`
	TenantID uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (RoutingConfig) TableName() string { return "sys_routing_config" }

// SnmpConfig SNMP 服务配置（单行，id=1；底层 net-snmp snmpd）。
type SnmpConfig struct {
	Base
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	Version     string `gorm:"column:version" json:"version"` // 2c | 3
	Community   string `gorm:"column:community" json:"community"`
	V3User      string `gorm:"column:v3_user" json:"v3User"`
	V3Password  string `gorm:"column:v3_password" json:"v3Password"`
	Location    string `gorm:"column:location" json:"location"`
	Contact     string `gorm:"column:contact" json:"contact"`
	ListenAddr  string `gorm:"column:listen_addr" json:"listenAddr"`
	AllowedHost string `gorm:"column:allowed_host" json:"allowedHost"`
	Applied     bool   `gorm:"column:applied" json:"applied"`
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (SnmpConfig) TableName() string { return "sys_snmp_config" }

// PluginSource 插件市场源（在线索引）。
type PluginSource struct {
	Base
	Name string `gorm:"column:name" json:"name"`
	// URL 索引地址（HTTPS JSON）
	URL string `gorm:"column:url" json:"url"`
	// TrustKey 索引签名校验密钥（HMAC-SHA256）；为空表示不校验签名
	TrustKey    string `gorm:"column:trust_key" json:"-"`
	Enabled     bool   `gorm:"column:enabled" json:"enabled"`
	LastSyncAt  string `gorm:"column:last_sync_at" json:"lastSyncAt"`
	LastError   string `gorm:"column:last_error" json:"lastError"`
	ItemCount   int    `gorm:"column:item_count" json:"itemCount"`
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
}

// TableName 表名。
func (PluginSource) TableName() string { return "sys_plugin_source" }
