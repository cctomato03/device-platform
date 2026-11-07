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
  id            BIGINT UNSIGNED NOT NULL COMMENT '主键（雪花 ID）',
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户 ID',
  user_id       BIGINT UNSIGNED NOT NULL COMMENT '负责人 ID',
  title         VARCHAR(200)    NOT NULL COMMENT '标题',
  content       VARCHAR(2000)   NOT NULL DEFAULT '' COMMENT '内容',
  priority      TINYINT         NOT NULL DEFAULT 2 COMMENT '优先级：1 低 2 普通 3 高 4 紧急',
  status        TINYINT         NOT NULL DEFAULT 1 COMMENT '状态：1 待处理 2 已完成 3 已取消',
  due_time      DATETIME        NULL COMMENT '截止时间',
  complete_time DATETIME        NULL COMMENT '完成时间',
  link_url      VARCHAR(500)    NOT NULL DEFAULT '' COMMENT '关联业务地址（点击跳转）',
  biz_type      VARCHAR(32)     NOT NULL DEFAULT '' COMMENT '业务类型（如 order/approval）',
  biz_id        VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '业务主键',
  biz_key       VARCHAR(128)    NULL COMMENT '幂等键（业务方唯一标识，重复推送不重复建单）',
  source        TINYINT         NOT NULL DEFAULT 1 COMMENT '来源：1 手工 2 系统推送 3 开放接口',
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME        NULL,
  deleted       BIGINT          NOT NULL DEFAULT 0 COMMENT '软删除时间戳（0 未删除）',
  remark        VARCHAR(500)    NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_user_status (user_id, status),
  KEY idx_tenant (tenant_id),
  -- biz_key 为 NULL 时 MySQL 允许多行，手工建的待办不会互相冲突
  UNIQUE KEY uk_biz (tenant_id, biz_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='待办事项';

-- ---------- 2. 通知已读 sys_notice_read ----------
CREATE TABLE IF NOT EXISTS sys_notice_read (
  notice_id   BIGINT UNSIGNED NOT NULL COMMENT '公告 ID',
  user_id     BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
  read_time   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '阅读时间',
  PRIMARY KEY (notice_id, user_id),
  KEY idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知已读记录';

-- ---------- 3. 站内信补幂等键（业务系统推送去重） ----------
ALTER TABLE sys_message
  ADD COLUMN biz_type VARCHAR(32)  NOT NULL DEFAULT '' AFTER `type`,
  ADD COLUMN biz_key  VARCHAR(128) NULL AFTER biz_type,
  ADD UNIQUE KEY uk_biz (tenant_id, biz_key);

-- ---------- 4. 菜单英文名（前端按当前语言取 name / name_en） ----------
ALTER TABLE sys_menu ADD COLUMN name_en VARCHAR(64) NOT NULL DEFAULT '' AFTER name;

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

INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES

  (1500, 1300, '消息与待办', 'Message & Task Push',  2, 'system:push:list',       'push',       'system/push/index',        'ri:send-plane-2-line',     7, 1, 1, 1, 1, NOW(), '手工发起通知/消息/待办，或对接业务系统');


-- 授权：超管隐式全量，这里补普通角色可见范围
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1500);
