-- 个人中心：头像下拉菜单入口（隐藏菜单，不出现在侧边栏）
-- 说明：/system/user-center 是每个登录用户都能访问的自身资料/密码维护页，
--       因此不挂业务权限码，只需注册为隐藏路由（visible=0）。
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1110, 1000, '个人中心', 2, '', '/system/user-center', 'system/user-center/index', 'user', 99, 0, 1, 1, 1, NOW(), '当前登录用户的资料与密码维护');

-- 授予全部现有角色（该页面不涉及业务数据权限）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) SELECT id, 1110 FROM sys_role;
