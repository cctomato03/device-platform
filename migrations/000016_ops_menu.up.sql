-- =============================================================================
-- 000016_ops_menu.up.sql : 新增运维能力菜单与权限点
--   · 网络管理：网卡配置、策略路由、DHCP 服务、DNS 服务、动态路由、网桥与聚合
--   · 设备管理：SNMP 服务
--   · 插件市场：插件源管理
-- =============================================================================

-- ---------------------------- 网络管理新增菜单 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3115, 3100, '网卡配置', 'Interface Config', 2, 'network:ifacecfg:list', 'iface-config', 'network/iface-config/index', 'ri:settings-4-line', 5, 1, 1, 1, 1, datetime('now','localtime'), '静态地址/DHCP/MTU（netplan）'),
(3140, 3100, '策略路由', 'Policy Route',     2, 'network:policyroute:list', 'policy-route', 'network/policy-route/index', 'ri:git-branch-line', 6, 1, 1, 1, 1, datetime('now','localtime'), 'ip rule + 路由表'),
(3150, 3100, 'DHCP 服务', 'DHCP Server',     2, 'network:dhcp:list', 'dhcp', 'network/dhcp/index', 'ri:server-line', 7, 1, 1, 1, 1, datetime('now','localtime'), 'dnsmasq 地址池与静态绑定'),
(3160, 3100, 'DNS 服务', 'DNS Server',       2, 'network:dnsserver:list', 'dns-server', 'network/dns-server/index', 'ri:cloud-line', 8, 1, 1, 1, 1, datetime('now','localtime'), 'dnsmasq 本地解析与上游转发'),
(3170, 3100, '动态路由', 'Dynamic Routing',  2, 'network:routing:list', 'routing', 'network/routing/index', 'ri:route-line', 9, 1, 1, 1, 1, datetime('now','localtime'), 'BGP / OSPF（FRR）'),
(3180, 3100, '网桥与聚合', 'Bridge & Bond',  2, 'network:bridge:list', 'bridge', 'network/bridge/index', 'ri:links-line', 10, 1, 1, 1, 1, datetime('now','localtime'), 'Linux 网桥与端口聚合（netplan）');

INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3116, 3115, '保存',     'Save',   3, 'network:ifacecfg:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3117, 3115, '下发',     'Apply',  3, 'network:ifacecfg:apply',  '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3141, 3140, '新增',     'Create', 3, 'network:policyroute:create', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3142, 3140, '修改',     'Update', 3, 'network:policyroute:update', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3143, 3140, '删除',     'Delete', 3, 'network:policyroute:delete', '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3144, 3140, '下发',     'Apply',  3, 'network:policyroute:apply',  '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime')),
(3151, 3150, '保存',     'Save',   3, 'network:dhcp:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3152, 3150, '下发',     'Apply',  3, 'network:dhcp:apply',  '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3153, 3150, '安装依赖', 'Install',3, 'network:dhcp:install','', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3161, 3160, '保存',     'Save',   3, 'network:dnsserver:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3162, 3160, '下发',     'Apply',  3, 'network:dnsserver:apply',  '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3163, 3160, '安装依赖', 'Install',3, 'network:dnsserver:install','', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3171, 3170, '保存',     'Save',   3, 'network:routing:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3172, 3170, '下发',     'Apply',  3, 'network:routing:apply',  '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3173, 3170, '安装依赖', 'Install',3, 'network:routing:install','', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3181, 3180, '新增',     'Create', 3, 'network:bridge:create', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3182, 3180, '修改',     'Update', 3, 'network:bridge:update', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3183, 3180, '删除',     'Delete', 3, 'network:bridge:delete', '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3184, 3180, '下发',     'Apply',  3, 'network:bridge:apply',  '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 设备管理：SNMP ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3260, 3200, 'SNMP 服务', 'SNMP', 2, 'device:snmp:list', 'snmp', 'device/snmp/index', 'ri:radar-line', 6, 1, 1, 1, 1, datetime('now','localtime'), 'net-snmp 采集配置与状态');

INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3261, 3260, '保存',     'Save',   3, 'device:snmp:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3262, 3260, '下发',     'Apply',  3, 'device:snmp:apply',  '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3263, 3260, '安装依赖', 'Install',3, 'device:snmp:install','', '', '', 3, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 插件市场：插件源 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3420, 3400, '插件源', 'Sources', 2, 'plugin:source:list', 'source', 'plugin/source/index', 'ri:cloud-line', 2, 1, 1, 1, 1, datetime('now','localtime'), '在线插件索引与签名校验');

INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3421, 3420, '新增', 'Create', 3, 'plugin:source:create', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3422, 3420, '修改', 'Update', 3, 'plugin:source:update', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3423, 3420, '删除', 'Delete', 3, 'plugin:source:delete', '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 授权给内置超管角色 ----------------------------
INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id IN (
  3115,3116,3117,
  3140,3141,3142,3143,3144,
  3150,3151,3152,3153,
  3160,3161,3162,3163,
  3170,3171,3172,3173,
  3180,3181,3182,3183,3184,
  3260,3261,3262,3263,
  3420,3421,3422,3423
);
