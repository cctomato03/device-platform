-- 代码生成器菜单（系统设置 → 代码生成）
INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1140, 1300, '代码生成', 2, 'tool:gen:list',     'gen', 'system/gen/index', 'ep:magic-stick', 7, 1, 1, 1, 1, datetime('now','localtime'), '读取 information_schema 生成 CRUD 骨架'),
  (1141, 1140, '预览',     3, 'tool:gen:preview', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime'), ''),
  (1142, 1140, '下载',     3, 'tool:gen:download', '', '', '', 2, 1, 1, 1, 1, datetime('now','localtime'), '');

INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1140), (1, 1141), (1, 1142);
