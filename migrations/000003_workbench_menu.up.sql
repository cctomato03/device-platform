-- 工作台（首页）菜单：对接 /api/v1/dashboard/summary
INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (2000, 0, '工作台', 2, 'dashboard:view', '/dashboard', 'dashboard/workbench/index', 'home', 0, 1, 1, 1, 1, datetime('now','localtime'), '数据概览首页');

-- 授予内置角色（超级管理员 / 普通用户）
INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 2000), (2, 2000);

-- 补全文件上传按钮权限（后端 /files/upload 已强制校验 system:file:upload）
INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time)
VALUES
  (1092, 1090, '上传', 3, 'system:file:upload', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime'));

