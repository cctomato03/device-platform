-- =============================================================================
-- 000004_menu_structure_and_icons
--   1) 菜单树按「业务属性」重组：
--        工作台  /  组织与权限（用户/角色/菜单/部门/岗位）
--        系统设置（字典/参数/租户/文件/定时任务）  /  系统监控（在线/服务器/登录日志/操作日志）
--   2) 为所有目录与菜单补齐 Iconify 图标（ep:xxx），杜绝侧边栏空白图标
-- 说明：只调整 parent_id / name / icon / sort / path，菜单 id 与权限码保持不变，
--       因此角色授权（sys_role_menu）与后端 middleware.Perm 校验完全不受影响。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------- 1. 顶级目录：系统管理 → 组织与权限 ----------
UPDATE sys_menu
SET name = '组织与权限', icon = 'ep:management', sort = 10, remark = '用户/角色/菜单/部门/岗位'
WHERE id = 1000;

-- ---------- 2. 新增顶级目录：系统设置（承载系统级配置与运维） ----------
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1300, 0, '系统设置', 1, '', '/setting', 'Layout', 'ep:setting', 20, 1, 1, 1, 1, NOW(), '字典/参数/租户/文件/定时任务');

-- 把「配置类」菜单迁移到系统设置下（路径同步改为 /setting/xxx，语义更清晰）
UPDATE sys_menu SET parent_id = 1300, path = 'dict',   sort = 1, icon = 'ep:notebook'   WHERE id = 1060;
UPDATE sys_menu SET parent_id = 1300, path = 'config', sort = 2, icon = 'ep:operation'  WHERE id = 1070;
UPDATE sys_menu SET parent_id = 1300, path = 'tenant', sort = 3, icon = 'ep:box'        WHERE id = 1080;
UPDATE sys_menu SET parent_id = 1300, path = 'file',   sort = 4, icon = 'ep:folder'     WHERE id = 1090;
UPDATE sys_menu SET parent_id = 1300, path = 'job',    sort = 5, icon = 'ep:timer'      WHERE id = 1100;

-- ---------- 3. 系统监控：图标与排序 ----------
UPDATE sys_menu SET icon = 'ep:monitor', sort = 30 WHERE id = 1200;
UPDATE sys_menu SET icon = 'ep:connection'  , sort = 1 WHERE id = 1210;
UPDATE sys_menu SET icon = 'ep:cpu'         , sort = 2 WHERE id = 1220;
UPDATE sys_menu SET icon = 'ep:key'         , sort = 3 WHERE id = 1230;
UPDATE sys_menu SET icon = 'ep:tickets'     , sort = 4 WHERE id = 1240;

-- ---------- 4. 组织与权限：各菜单图标 + 排序 ----------
UPDATE sys_menu SET icon = 'ep:user'            , sort = 1 WHERE id = 1010;
UPDATE sys_menu SET icon = 'ep:avatar'          , sort = 2 WHERE id = 1020;
UPDATE sys_menu SET icon = 'ep:menu'            , sort = 3 WHERE id = 1030;
UPDATE sys_menu SET icon = 'ep:office-building' , sort = 4 WHERE id = 1040;
UPDATE sys_menu SET icon = 'ep:postcard'        , sort = 5 WHERE id = 1050;

-- ---------- 5. 工作台图标 ----------
UPDATE sys_menu SET icon = 'ep:home-filled' WHERE id = 2000;

-- ---------- 6. 授权：新目录授予内置角色（超管直通，普通用户保持可见工作台） ----------
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1300), (2, 1300);

-- 把字典/参数/租户/文件/定时任务（含子按钮）在新目录下的授权补齐给超管
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id IN (1060,1061,1062,1063,1070,1071,1072,1073,1080,1081,1082,1083,1090,1091,1092,1100,1101,1102,1103,1104);
