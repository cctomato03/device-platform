-- =============================================================================
-- 000014_platform_menu : 设备平台菜单与权限点
-- 说明：菜单同时承载「路由 + 权限点」，type 1目录 2页面 3按钮
-- 权限码约定：模块:资源:动作（与后端 middleware.Perm 一一对应）
-- =============================================================================

-- ---------------------------- 主机管理 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3000, 0,    '主机管理', 'Host',     1, '',                     '/host',   'Layout',                 'ep:cpu',            5,  1, 1, 1, 1, datetime('now','localtime'), '系统概览、硬件状态、监控与流量'),
(3010, 3000, '系统概览', 'Overview', 2, 'host:overview:view',   'overview','host/overview/index',    'ri:dashboard-3-line',1, 1, 1, 1, 1, datetime('now','localtime'), '设备整体运行状态'),
(3020, 3000, '硬件状态', 'Hardware', 2, 'host:hardware:view',   'hardware','host/hardware/index',    'ep:monitor',        2,  1, 1, 1, 1, datetime('now','localtime'), 'CPU/内存/磁盘/温度明细'),
(3030, 3000, '系统监控', 'Monitor',  2, 'host:monitor:view',    'monitor', 'host/monitor/index',     'ri:line-chart-line',3,  1, 1, 1, 1, datetime('now','localtime'), '指标趋势曲线'),
(3040, 3000, '网络流量', 'Traffic',  2, 'host:traffic:view',    'traffic', 'host/traffic/index',     'ri:exchange-line',  4,  1, 1, 1, 1, datetime('now','localtime'), '出入流量趋势与明细');

-- ---------------------------- 网络管理 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3100, 0,    '网络管理', 'Network',      1, '',                        '/network', 'Layout',                    'ep:connection',      6,  1, 1, 1, 1, datetime('now','localtime'), '网卡、路由、DNS'),
(3110, 3100, '网络接口', 'Interfaces',   2, 'network:iface:list',      'iface',    'network/iface/index',        'ri:link',            1,  1, 1, 1, 1, datetime('now','localtime'), '网卡状态与地址'),
(3120, 3100, '静态路由', 'Static Route', 2, 'network:route:list',      'route',    'network/route/index',        'ri:route-line',      2,  1, 1, 1, 1, datetime('now','localtime'), '静态路由配置与下发'),
(3130, 3100, 'DNS 配置', 'DNS',         2, 'network:dns:list',        'dns',      'network/dns/index',          'ri:global-line',     3,  1, 1, 1, 1, datetime('now','localtime'), 'DNS 服务器与解析配置');

-- 网络管理按钮
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3121, 3120, '新增', 'Create', 3, 'network:route:create', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3122, 3120, '修改', 'Update', 3, 'network:route:update', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3123, 3120, '删除', 'Delete', 3, 'network:route:delete', '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3124, 3120, '下发', 'Apply',  3, 'network:route:apply',  '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime')),
(3131, 3130, '保存', 'Update', 3, 'network:dns:update',   '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 设备管理 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3200, 0,    '设备管理', 'Device',       1, '',                        '/device',  'Layout',                  'ep:setting',         7,  1, 1, 1, 1, datetime('now','localtime'), '本地设置、告警、排查工具'),
(3210, 3200, '本地设置', 'Local Settings',2, 'device:setting:list',    'setting',  'device/setting/index',    'ri:settings-3-line', 1,  1, 1, 1, 1, datetime('now','localtime'), '时间/NTP/设备名/DNS'),
(3220, 3200, '运行告警', 'Alerts',       2, 'device:alert:list',      'alert',    'device/alert/index',      'ri:alert-line',      2,  1, 1, 1, 1, datetime('now','localtime'), '告警规则与记录'),
(3230, 3200, '排查工具', 'Diagnostics',  2, 'device:tool:list',       'tools',    'device/tools/index',      'ri:terminal-box-line',3, 1, 1, 1, 1, datetime('now','localtime'), 'ping/traceroute/tcpdump/netstat'),
(3240, 3200, '网络状态', 'Net Status',   2, 'device:netstatus:view',  'netstatus','device/netstatus/index',  'ri:signal-tower-line',4, 1, 1, 1, 1, datetime('now','localtime'), '网卡实时状态'),
(3250, 3200, '关于系统', 'About',        2, 'device:about:view',      'about',    'device/about/index',      'ri:information-line',5,  1, 1, 1, 1, datetime('now','localtime'), '版本与授权信息');

-- 设备管理按钮
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3211, 3210, '保存',   'Save',   3, 'device:setting:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3221, 3220, '新增',   'Create', 3, 'device:alert:create',   '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3222, 3220, '修改',   'Update', 3, 'device:alert:update',   '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3223, 3220, '删除',   'Delete', 3, 'device:alert:delete',   '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3231, 3230, '执行',   'Run',    3, 'device:tool:run',       '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 安全策略 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3300, 0,    '安全策略', 'Security',     1, '',                        '/security', 'Layout',                  'ep:lock',           8,  1, 1, 1, 1, datetime('now','localtime'), '防火墙、策略、NAT、QoS'),
(3310, 3300, '防火墙',   'Firewall',     2, 'security:firewall:list',  'firewall',  'security/firewall/index', 'ri:shield-check-line',1, 1, 1, 1, 1, datetime('now','localtime'), 'firewalld 与规则查看'),
(3320, 3300, '安全策略', 'Policies',     2, 'security:policy:list',    'policy',    'security/policy/index',   'ri:file-shield-2-line',2,1, 1, 1, 1, datetime('now','localtime'), '一体化访问控制策略'),
(3330, 3300, 'NAT 策略', 'NAT',          2, 'security:nat:list',       'nat',       'security/nat/index',      'ri:swap-box-line',   3,  1, 1, 1, 1, datetime('now','localtime'), 'SNAT/DNAT 转发'),
(3340, 3300, '带宽管理', 'QoS',          2, 'security:qos:list',       'qos',       'security/qos/index',      'ri:speed-up-line',   4,  1, 1, 1, 1, datetime('now','localtime'), '接口限速与优先级');

-- 安全策略按钮
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3311, 3310, '安装', 'Install', 3, 'security:firewall:install', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3321, 3320, '新增', 'Create',  3, 'security:policy:create',    '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3322, 3320, '修改', 'Update',  3, 'security:policy:update',    '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3323, 3320, '删除', 'Delete',  3, 'security:policy:delete',    '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3324, 3320, '下发', 'Apply',   3, 'security:policy:apply',     '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime')),
(3331, 3330, '新增', 'Create',  3, 'security:nat:create',       '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3332, 3330, '修改', 'Update',  3, 'security:nat:update',       '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3333, 3330, '删除', 'Delete',  3, 'security:nat:delete',       '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3334, 3330, '下发', 'Apply',   3, 'security:nat:apply',        '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime')),
(3341, 3340, '新增', 'Create',  3, 'security:qos:create',       '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3342, 3340, '修改', 'Update',  3, 'security:qos:update',       '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3343, 3340, '删除', 'Delete',  3, 'security:qos:delete',       '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3344, 3340, '下发', 'Apply',   3, 'security:qos:apply',        '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 插件市场 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3400, 0,    '插件市场', 'Plugins',  1, '',                       '/plugin', 'Layout',               'ep:box',              9,  1, 1, 1, 1, datetime('now','localtime'), '插件安装与生命周期管理'),
(3410, 3400, '插件市场', 'Market',   2, 'plugin:market:list',     'market',  'plugin/market/index',  'ri:apps-2-line',      1,  1, 1, 1, 1, datetime('now','localtime'), '已安装与可安装插件');

INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(3411, 3410, '安装',   'Install',   3, 'plugin:market:install',   '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime')),
(3412, 3410, '卸载',   'Uninstall', 3, 'plugin:market:uninstall', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime')),
(3413, 3410, '启停',   'Lifecycle', 3, 'plugin:lifecycle:update', '', '', '', 3, 1, 1, 1, 1, datetime('now','localtime')),
(3414, 3410, '配置',   'Config',    3, 'plugin:config:update',    '', '', '', 4, 1, 1, 1, 1, datetime('now','localtime')),
(3415, 3410, '日志',   'Logs',      3, 'plugin:log:view',         '', '', '', 5, 1, 1, 1, 1, datetime('now','localtime'));

-- ---------------------------- 日志审计 ----------------------------
INSERT OR IGNORE INTO sys_menu (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark) VALUES
(3500, 0,    '日志审计', 'Log Audit',   1, '',                          '/log',    'Layout',                'ep:document',      11, 1, 1, 1, 1, datetime('now','localtime'), '访问/系统/流量日志'),
(3510, 3500, '访问日志', 'Access Log',  2, 'monitor:access-log:list',   'access',  'log/access/index',      'ri:file-list-3-line',1, 1, 1, 1, 1, datetime('now','localtime'), '所有 HTTP 访问记录'),
(3520, 3500, '系统日志', 'System Log',  2, 'monitor:system-log:list',   'system',  'log/system/index',      'ri:file-text-line',   2, 1, 1, 1, 1, datetime('now','localtime'), '平台运行日志'),
(3530, 3500, '流量日志', 'Traffic Log', 2, 'monitor:traffic-log:list',  'traffic', 'log/traffic/index',     'ri:bar-chart-2-line', 3, 1, 1, 1, 1, datetime('now','localtime'), '出入流量采样记录');

-- ---------------------------- 授权：内置「超级管理员」角色 ----------------------------
INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id IN (
  3000,3010,3020,3030,3040,
  3100,3110,3120,3121,3122,3123,3124,3130,3131,
  3200,3210,3211,3220,3221,3222,3223,3230,3231,3240,3250,
  3300,3310,3311,3320,3321,3322,3323,3324,3330,3331,3332,3333,3334,3340,3341,3342,3343,3344,
  3400,3410,3411,3412,3413,3414,3415,
  3500,3510,3520,3530
);

-- ---------------------------- 平台品牌信息（沿用脚手架种子，改为设备平台口径） ----------------------------
UPDATE sys_config SET value = '设备管理平台' WHERE config_key = 'sys.name';
UPDATE sys_config SET value = 'Copyright © 2026 设备管理平台' WHERE config_key = 'sys.copyright';
UPDATE sys_config SET value = '设备管理平台 · 主机、网络、插件一体化运维' WHERE config_key = 'sys.login.subtitle';
UPDATE sys_site_config SET
  site_name      = '设备管理平台',
  login_desc     = '主机、网络、插件一体化的设备运维平台',
  login_sub_title = '开箱即用的单机设备管理底座'
WHERE id = 1;
INSERT OR IGNORE INTO sys_config (id, name, config_key, value, value_type, is_builtin, is_public, tenant_id, create_time, remark)
VALUES (20, '磁盘使用率告警阈值(%)', 'sys.alert.disk', '90', 'int', 1, 0, 1, datetime('now','localtime'), '设备磁盘使用率超过该值时触发告警');
