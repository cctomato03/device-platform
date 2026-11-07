-- =============================================================================
-- 000002_seed_data : 初始化种子数据（可重复执行，全部 INSERT IGNORE）
-- 超管账号由程序首次启动时创建（密码取环境变量或随机生成并打印到日志）
-- =============================================================================

SET NAMES utf8mb4;

-- ---------- 租户 ----------
INSERT IGNORE INTO sys_tenant (id, name, code, contact_name, status, create_time, remark)
VALUES (1, '平台默认租户', 'default', '管理员', 1, NOW(), '系统初始化创建');

-- ---------- 部门 ----------
INSERT IGNORE INTO sys_dept (id, parent_id, ancestors, name, code, leader, sort, status, tenant_id, create_time, remark)
VALUES (1, 0, '0', '总公司', 'HQ', '管理员', 1, 1, 1, NOW(), '系统初始化创建');

-- ---------- 角色 ----------
INSERT IGNORE INTO sys_role (id, name, code, sort, data_scope, status, is_builtin, tenant_id, create_time, remark)
VALUES (1, '超级管理员', 'super_admin', 1, 1, 1, 1, 1, NOW(), '内置角色，拥有全部权限'),
       (2, '普通用户', 'common', 2, 4, 1, 1, 1, NOW(), '仅本人数据');

-- ---------- 岗位 ----------
INSERT IGNORE INTO sys_post (id, code, name, sort, status, tenant_id, create_time, remark)
VALUES (1, 'ceo', '董事长', 1, 1, 1, NOW(), ''),
       (2, 'pm', '项目经理', 2, 1, 1, NOW(), ''),
       (3, 'dev', '开发工程师', 3, 1, 1, NOW(), '');

-- ---------- 菜单：系统管理 ----------
INSERT IGNORE INTO sys_menu (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time) VALUES
(1000, 0,    '系统管理', 1, '',                    '/system', 'Layout',              'settings', 10, 1, 1, 1, 1, NOW()),
(1010, 1000, '用户管理', 2, 'system:user:list',    'user',    'system/user/index',   'user',     1,  1, 1, 1, 1, NOW()),
(1011, 1010, '新增',     3, 'system:user:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1012, 1010, '修改',     3, 'system:user:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1013, 1010, '删除',     3, 'system:user:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1014, 1010, '重置密码', 3, 'system:user:reset-pwd','',       '',                    '',         4,  1, 1, 1, 1, NOW()),
(1015, 1010, '分配角色', 3, 'system:user:assign',  '',        '',                    '',         5,  1, 1, 1, 1, NOW()),
(1020, 1000, '角色管理', 2, 'system:role:list',    'role',    'system/role/index',   'safe',     2,  1, 1, 1, 1, NOW()),
(1021, 1020, '新增',     3, 'system:role:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1022, 1020, '修改',     3, 'system:role:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1023, 1020, '删除',     3, 'system:role:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1024, 1020, '分配权限', 3, 'system:role:grant',   '',        '',                    '',         4,  1, 1, 1, 1, NOW()),
(1030, 1000, '菜单管理', 2, 'system:menu:list',    'menu',    'system/menu/index',   'menu',     3,  1, 1, 1, 1, NOW()),
(1031, 1030, '新增',     3, 'system:menu:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1032, 1030, '修改',     3, 'system:menu:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1033, 1030, '删除',     3, 'system:menu:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1040, 1000, '部门管理', 2, 'system:dept:list',    'dept',    'system/dept/index',   'cluster',  4,  1, 1, 1, 1, NOW()),
(1041, 1040, '新增',     3, 'system:dept:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1042, 1040, '修改',     3, 'system:dept:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1043, 1040, '删除',     3, 'system:dept:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1050, 1000, '岗位管理', 2, 'system:post:list',    'post',    'system/post/index',   'id-card',  5,  1, 1, 1, 1, NOW()),
(1051, 1050, '新增',     3, 'system:post:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1052, 1050, '修改',     3, 'system:post:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1053, 1050, '删除',     3, 'system:post:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1060, 1000, '字典管理', 2, 'system:dict:list',    'dict',    'system/dict/index',   'book',     6,  1, 1, 1, 1, NOW()),
(1061, 1060, '新增',     3, 'system:dict:create',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1062, 1060, '修改',     3, 'system:dict:update',  '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1063, 1060, '删除',     3, 'system:dict:delete',  '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1070, 1000, '参数配置', 2, 'system:config:list',  'config',  'system/config/index', 'sliders',  7,  1, 1, 1, 1, NOW()),
(1071, 1070, '新增',     3, 'system:config:create','',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1072, 1070, '修改',     3, 'system:config:update','',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1073, 1070, '删除',     3, 'system:config:delete','',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1080, 1000, '租户管理', 2, 'system:tenant:list',  'tenant',  'system/tenant/index', 'flag',     8,  1, 1, 1, 1, NOW()),
(1081, 1080, '新增',     3, 'system:tenant:create','',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1082, 1080, '修改',     3, 'system:tenant:update','',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1083, 1080, '删除',     3, 'system:tenant:delete','',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1090, 1000, '文件管理', 2, 'system:file:list',    'file',    'system/file/index',   'folder',   9,  1, 1, 1, 1, NOW()),
(1091, 1090, '删除',     3, 'system:file:delete',  '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1100, 1000, '定时任务', 2, 'system:job:list',     'job',     'system/job/index',    'clock',    10, 1, 1, 1, 1, NOW()),
(1101, 1100, '新增',     3, 'system:job:create',   '',        '',                    '',         1,  1, 1, 1, 1, NOW()),
(1102, 1100, '修改',     3, 'system:job:update',   '',        '',                    '',         2,  1, 1, 1, 1, NOW()),
(1103, 1100, '删除',     3, 'system:job:delete',   '',        '',                    '',         3,  1, 1, 1, 1, NOW()),
(1104, 1100, '手动执行', 3, 'system:job:run',      '',        '',                    '',         4,  1, 1, 1, 1, NOW()),
(1200, 0,    '系统监控', 1, '',                    '/monitor', 'Layout',             'monitor',  20, 1, 1, 1, 1, NOW()),
(1210, 1200, '在线用户', 2, 'monitor:online:list', 'online',  'monitor/online/index','online',   1,  1, 1, 1, 1, NOW()),
(1220, 1200, '服务器监控', 2, 'monitor:server:view','server', 'monitor/server/index','cpu',      2,  1, 1, 1, 1, NOW()),
(1230, 1200, '登录日志', 2, 'monitor:login-log:list','login-log','monitor/login-log/index','login', 3, 1, 1, 1, 1, NOW()),
(1240, 1200, '操作日志', 2, 'monitor:oper-log:list','oper-log','monitor/oper-log/index','record', 4,  1, 1, 1, 1, NOW());

-- ---------- 字典 ----------
INSERT IGNORE INTO sys_dict_type (id, name, code, status, is_builtin, create_time, remark) VALUES
(1, '用户性别', 'sys_user_sex', 1, 1, NOW(), ''),
(2, '通用状态', 'sys_normal_disable', 1, 1, NOW(), ''),
(3, '菜单类型', 'sys_menu_type', 1, 1, NOW(), ''),
(4, '是否', 'sys_yes_no', 1, 1, NOW(), ''),
(5, '数据范围', 'sys_data_scope', 1, 1, NOW(), ''),
(6, '任务并发策略', 'sys_job_concurrent', 1, 1, NOW(), '');

INSERT IGNORE INTO sys_dict_data (id, dict_code, label, value, sort, tag_type, is_default, status, create_time) VALUES
(101, 'sys_user_sex', '未知', '0', 1, '', 1, 1, NOW()),
(102, 'sys_user_sex', '男', '1', 2, '', 0, 1, NOW()),
(103, 'sys_user_sex', '女', '2', 3, '', 0, 1, NOW()),
(201, 'sys_normal_disable', '正常', '1', 1, 'success', 1, 1, NOW()),
(202, 'sys_normal_disable', '停用', '0', 2, 'danger', 0, 1, NOW()),
(301, 'sys_menu_type', '目录', '1', 1, 'primary', 1, 1, NOW()),
(302, 'sys_menu_type', '菜单', '2', 2, 'success', 0, 1, NOW()),
(303, 'sys_menu_type', '按钮', '3', 3, 'warning', 0, 1, NOW()),
(304, 'sys_menu_type', '接口', '4', 4, 'info', 0, 1, NOW()),
(401, 'sys_yes_no', '是', '1', 1, 'success', 1, 1, NOW()),
(402, 'sys_yes_no', '否', '0', 2, 'info', 0, 1, NOW()),
(501, 'sys_data_scope', '全部数据', '1', 1, 'danger', 1, 1, NOW()),
(502, 'sys_data_scope', '本部门数据', '2', 2, 'warning', 0, 1, NOW()),
(503, 'sys_data_scope', '本部门及以下', '3', 3, 'primary', 0, 1, NOW()),
(504, 'sys_data_scope', '仅本人数据', '4', 4, 'info', 0, 1, NOW()),
(505, 'sys_data_scope', '自定义部门', '5', 5, 'info', 0, 1, NOW()),
(601, 'sys_job_concurrent', '禁止并发', '1', 1, 'warning', 1, 1, NOW()),
(602, 'sys_job_concurrent', '允许并发', '2', 2, 'success', 0, 1, NOW()),
(603, 'sys_job_concurrent', '丢弃后续', '3', 3, 'info', 0, 1, NOW());

-- ---------- 系统参数 ----------
INSERT IGNORE INTO sys_config (id, name, config_key, value, value_type, is_builtin, is_public, tenant_id, create_time, remark) VALUES
(1, '系统名称', 'sys.name', 'Web Basic 管理平台', 'string', 1, 1, 0, NOW(), '登录页与浏览器标题'),
(2, '系统Logo', 'sys.logo', '/logo.svg', 'string', 1, 1, 0, NOW(), ''),
(3, '版权信息', 'sys.copyright', 'Copyright © 2026 Web Basic', 'string', 1, 1, 0, NOW(), ''),
(4, '是否开启验证码', 'sys.login.captcha', 'true', 'bool', 1, 1, 0, NOW(), ''),
(5, '上传大小限制(MB)', 'sys.upload.max_size', '20', 'int', 1, 0, 0, NOW(), ''),
(6, '是否开启多租户', 'sys.tenant.enabled', 'true', 'bool', 1, 1, 0, NOW(), '');

-- ---------- 内置定时任务（默认停用） ----------
INSERT IGNORE INTO sys_job (id, name, handler, job_type, target, http_method, cron, params, concurrent, timeout_sec, retry_times, status, tenant_id, create_time, remark) VALUES
(1, '清理过期登录会话', 'cleanSession', 1, '', 'POST', '0 */10 * * * *', '{}', 1, 60, 0, 0, 0, NOW(), '清理 Redis 中过期的会话与令牌'),
(2, '清理临时文件', 'cleanTempFile', 1, '', 'POST', '0 0 3 * * *', '{"days":7}', 1, 300, 1, 0, 0, NOW(), '删除 7 天前未确认的临时文件'),
(3, '清理历史日志', 'cleanOperLog', 1, '', 'POST', '0 30 2 * * *', '{"days":90}', 1, 600, 1, 0, 0, NOW(), '归档/删除 90 天前的操作日志');
