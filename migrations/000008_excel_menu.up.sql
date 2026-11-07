-- Excel 导出 / 导入按钮权限
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1016, 1010, '导出', 3, 'system:user:export', '', '', '', 6, 1, 1, 1, 1, NOW(), '导出用户 Excel'),
  (1017, 1010, '导入', 3, 'system:user:import', '', '', '', 7, 1, 1, 1, 1, NOW(), '从 Excel 导入用户'),
  (1241, 1240, '导出', 3, 'monitor:oper-log:export', '', '', '', 1, 1, 1, 1, 1, NOW(), '导出操作日志 Excel');

INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1016), (1, 1017), (1, 1241);
