-- 通知公告 + 站内信
-- sys_notice：平台/租户内发布的公告（草稿 → 发布 → 关闭）
-- sys_message：站内信（公告发布时按用户扇出，用户可已读/删除）

CREATE TABLE IF NOT EXISTS sys_notice (
  id            INTEGER NOT NULL,
  title         TEXT    NOT NULL,
  type          INTEGER         NOT NULL DEFAULT 1,
  level         INTEGER         NOT NULL DEFAULT 1,
  content       TEXT      NULL,
  status        INTEGER         NOT NULL DEFAULT 0,
  publisher_id  INTEGER NOT NULL DEFAULT 0,
  publisher     TEXT     NOT NULL DEFAULT '',
  publish_time  DATETIME        NULL,
  tenant_id     INTEGER NOT NULL DEFAULT 0,
  create_by     INTEGER NOT NULL DEFAULT 0,
  create_time   DATETIME        NOT NULL DEFAULT (datetime('now','localtime')),
  update_by     INTEGER NOT NULL DEFAULT 0,
  update_time   DATETIME        NULL,
  deleted       INTEGER          NOT NULL DEFAULT 0,
  remark        TEXT    NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_notice_idx_tenant_status ON sys_notice (tenant_id, status);
CREATE  INDEX IF NOT EXISTS idx_sys_notice_idx_publish_time ON sys_notice (publish_time);

CREATE TABLE IF NOT EXISTS sys_message (
  id          INTEGER NOT NULL,
  user_id     INTEGER NOT NULL,
  notice_id   INTEGER NOT NULL DEFAULT 0,
  title       TEXT    NOT NULL DEFAULT '',
  content     TEXT            NULL,
  type        INTEGER         NOT NULL DEFAULT 1,
  is_read     INTEGER         NOT NULL DEFAULT 0,
  read_time   DATETIME        NULL,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME        NOT NULL DEFAULT (datetime('now','localtime')),
  deleted     INTEGER          NOT NULL DEFAULT 0,
  remark      TEXT    NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_message_idx_user_read ON sys_message (user_id, is_read);
CREATE  INDEX IF NOT EXISTS idx_sys_message_idx_notice ON sys_message (notice_id);

-- ---------- 菜单：通知公告（系统设置目录下）+ 我的消息 ----------
INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1120, 1300, '通知公告', 2, 'system:notice:list',  'notice',  'system/notice/index',  'ep:bell',        5, 1, 1, 1, 1, datetime('now','localtime'), '公告发布与站内信扇出'),
  (1121, 1120, '新增',     3, 'system:notice:create','',        '',                     '',               1, 1, 1, 1, 1, datetime('now','localtime'), ''),
  (1122, 1120, '修改',     3, 'system:notice:update','',        '',                     '',               2, 1, 1, 1, 1, datetime('now','localtime'), ''),
  (1123, 1120, '删除',     3, 'system:notice:delete','',        '',                     '',               3, 1, 1, 1, 1, datetime('now','localtime'), ''),
  (1124, 1120, '发布',     3, 'system:notice:publish','',       '',                     '',               4, 1, 1, 1, 1, datetime('now','localtime'), '');

-- 授权：仅超管可见（公告管理属于后台管理能力）
INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id IN (1120, 1121, 1122, 1123, 1124);

