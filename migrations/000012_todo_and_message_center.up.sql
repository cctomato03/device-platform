-- 待办 / 通知已读 / 消息幂等键 / 菜单英文名
--
-- 设计说明：
--   * 待办（sys_todo）、通知已读（sys_notice_read）面向「每个用户」，与工作台页签配合使用；
--   * 站内信与待办都带 biz_type/biz_key，便于业务系统推送时按幂等键去重；
--   * 菜单名支持双语（name_en），前端按当前语言取值，缺省回落中文。
--
-- 说明：DDL 依赖 schema_migrations 版本表保证只执行一次，因此使用普通 ALTER（不重复执行即幂等）。
-- ---------- 1. 待办 sys_todo ----------
CREATE TABLE IF NOT EXISTS sys_todo (
  id            INTEGER NOT NULL,
  tenant_id     INTEGER NOT NULL DEFAULT 0,
  user_id       INTEGER NOT NULL,
  title         TEXT    NOT NULL,
  content       TEXT   NOT NULL DEFAULT '',
  priority      INTEGER         NOT NULL DEFAULT 2,
  status        INTEGER         NOT NULL DEFAULT 1,
  due_time      DATETIME        NULL,
  complete_time DATETIME        NULL,
  link_url      TEXT    NOT NULL DEFAULT '',
  biz_type      TEXT     NOT NULL DEFAULT '',
  biz_id        TEXT     NOT NULL DEFAULT '',
  biz_key       TEXT    NULL,
  source        INTEGER         NOT NULL DEFAULT 1,
  create_by     INTEGER NOT NULL DEFAULT 0,
  create_time   DATETIME        NOT NULL DEFAULT (datetime('now','localtime')),
  update_by     INTEGER NOT NULL DEFAULT 0,
  update_time   DATETIME        NULL,
  deleted       INTEGER          NOT NULL DEFAULT 0,
  remark        TEXT    NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
-- biz_key 为 NULL 时唯一索引允许多行，手工建的待办不会互相冲突
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_todo_uk_biz ON sys_todo (tenant_id, biz_key);
CREATE  INDEX IF NOT EXISTS idx_sys_todo_idx_user_status ON sys_todo (user_id, status);
CREATE  INDEX IF NOT EXISTS idx_sys_todo_idx_tenant ON sys_todo (tenant_id);

-- ---------- 2. 通知已读 sys_notice_read ----------
CREATE TABLE IF NOT EXISTS sys_notice_read (
  notice_id   INTEGER NOT NULL,
  user_id     INTEGER NOT NULL,
  read_time   DATETIME        NOT NULL DEFAULT (datetime('now','localtime')),
  PRIMARY KEY (notice_id, user_id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_notice_read_idx_user ON sys_notice_read (user_id);

-- ---------- 3. 站内信补幂等键（业务系统推送去重） ----------
ALTER TABLE sys_message ADD COLUMN biz_type TEXT  NOT NULL DEFAULT '';
ALTER TABLE sys_message ADD COLUMN biz_key  TEXT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_message_uk_biz ON sys_message (tenant_id, biz_key);

-- ---------- 4. 菜单英文名（前端按当前语言取 name / name_en） ----------
ALTER TABLE sys_menu ADD COLUMN name_en TEXT NOT NULL DEFAULT '';

UPDATE sys_menu SET name_en = 'Dashboard'         WHERE id = 2000 AND name_en = '';
UPDATE sys_menu SET name_en = 'Organization'      WHERE id = 1000 AND name_en = '';
UPDATE sys_menu SET name_en = 'Users'             WHERE id = 1010 AND name_en = '';
UPDATE sys_menu SET name_en = 'Roles'             WHERE id = 1020 AND name_en = '';
UPDATE sys_menu SET name_en = 'Menus'             WHERE id = 1030 AND name_en = '';
UPDATE sys_menu SET name_en = 'Departments'       WHERE id = 1040 AND name_en = '';
UPDATE sys_menu SET name_en = 'Positions'         WHERE id = 1050 AND name_en = '';
UPDATE sys_menu SET name_en = 'Profile'           WHERE id = 1110 AND name_en = '';
UPDATE sys_menu SET name_en = 'Monitoring'        WHERE id = 1200 AND name_en = '';
UPDATE sys_menu SET name_en = 'Online Users'      WHERE id = 1210 AND name_en = '';
UPDATE sys_menu SET name_en = 'Server Info'       WHERE id = 1220 AND name_en = '';
UPDATE sys_menu SET name_en = 'Login Logs'        WHERE id = 1230 AND name_en = '';
UPDATE sys_menu SET name_en = 'Operation Logs'    WHERE id = 1240 AND name_en = '';
UPDATE sys_menu SET name_en = 'System Settings'   WHERE id = 1300 AND name_en = '';
UPDATE sys_menu SET name_en = 'Dictionaries'      WHERE id = 1060 AND name_en = '';
UPDATE sys_menu SET name_en = 'Parameters'        WHERE id = 1070 AND name_en = '';
UPDATE sys_menu SET name_en = 'Tenants'           WHERE id = 1080 AND name_en = '';
UPDATE sys_menu SET name_en = 'Files'             WHERE id = 1090 AND name_en = '';
UPDATE sys_menu SET name_en = 'Scheduled Jobs'    WHERE id = 1100 AND name_en = '';
UPDATE sys_menu SET name_en = 'Notice Management' WHERE id = 1120 AND name_en = '';
UPDATE sys_menu SET name_en = 'Code Generator'    WHERE id = 1140 AND name_en = '';
UPDATE sys_menu SET name_en = 'Site Settings'     WHERE id = 1150 AND name_en = '';

-- ---------- 5. 菜单：消息与待办（手工推送 / 对接调试入口） ----------
UPDATE sys_menu SET name = '通知公告管理' WHERE id = 1120;

INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES

  (1500, 1300, '消息与待办', 'Message & Task Push',  2, 'system:push:list',       'push',       'system/push/index',        'ri:send-plane-2-line',     7, 1, 1, 1, 1, datetime('now','localtime'), '手工发起通知/消息/待办，或对接业务系统');


-- 授权：超管隐式全量，这里补普通角色可见范围
INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1500);
