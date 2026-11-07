-- 通知公告 + 站内信
-- sys_notice：平台/租户内发布的公告（草稿 → 发布 → 关闭）
-- sys_message：站内信（公告发布时按用户扇出，用户可已读/删除）

CREATE TABLE IF NOT EXISTS sys_notice (
  id            BIGINT UNSIGNED NOT NULL COMMENT '主键（雪花 ID）',
  title         VARCHAR(200)    NOT NULL COMMENT '标题',
  type          TINYINT         NOT NULL DEFAULT 1 COMMENT '类型：1 通知 2 公告 3 维护',
  level         TINYINT         NOT NULL DEFAULT 1 COMMENT '级别：1 普通 2 重要 3 紧急',
  content       MEDIUMTEXT      NULL COMMENT '正文（纯文本/Markdown）',
  status        TINYINT         NOT NULL DEFAULT 0 COMMENT '状态：0 草稿 1 已发布 2 已关闭',
  publisher_id  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发布人 ID',
  publisher     VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '发布人名称',
  publish_time  DATETIME        NULL COMMENT '发布时间',
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户 ID',
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME        NULL,
  deleted       BIGINT          NOT NULL DEFAULT 0 COMMENT '软删除时间戳（0 未删除）',
  remark        VARCHAR(500)    NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_tenant_status (tenant_id, status),
  KEY idx_publish_time (publish_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知公告';

CREATE TABLE IF NOT EXISTS sys_message (
  id          BIGINT UNSIGNED NOT NULL COMMENT '主键（雪花 ID）',
  user_id     BIGINT UNSIGNED NOT NULL COMMENT '接收人 ID',
  notice_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联公告 ID（0 表示系统消息）',
  title       VARCHAR(200)    NOT NULL DEFAULT '' COMMENT '标题',
  content     TEXT            NULL COMMENT '内容',
  type        TINYINT         NOT NULL DEFAULT 1 COMMENT '类型：1 系统 2 公告 3 任务',
  is_read     TINYINT         NOT NULL DEFAULT 0 COMMENT '是否已读',
  read_time   DATETIME        NULL COMMENT '阅读时间',
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户 ID',
  create_time DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted     BIGINT          NOT NULL DEFAULT 0 COMMENT '软删除时间戳（0 未删除）',
  remark      VARCHAR(500)    NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_user_read (user_id, is_read),
  KEY idx_notice (notice_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站内信';

-- ---------- 菜单：通知公告（系统设置目录下）+ 我的消息 ----------
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1120, 1300, '通知公告', 2, 'system:notice:list',  'notice',  'system/notice/index',  'ep:bell',        5, 1, 1, 1, 1, NOW(), '公告发布与站内信扇出'),
  (1121, 1120, '新增',     3, 'system:notice:create','',        '',                     '',               1, 1, 1, 1, 1, NOW(), ''),
  (1122, 1120, '修改',     3, 'system:notice:update','',        '',                     '',               2, 1, 1, 1, 1, NOW(), ''),
  (1123, 1120, '删除',     3, 'system:notice:delete','',        '',                     '',               3, 1, 1, 1, 1, NOW(), ''),
  (1124, 1120, '发布',     3, 'system:notice:publish','',       '',                     '',               4, 1, 1, 1, 1, NOW(), '');

-- 授权：仅超管可见（公告管理属于后台管理能力）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id IN (1120, 1121, 1122, 1123, 1124);

