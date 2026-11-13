-- 站点个性化配置（单行表：id 恒为 1）+ 菜单 + 授权
-- 支持：网站名称、logo、登录页背景图、登录页描述/副标题、注册开关

CREATE TABLE IF NOT EXISTS sys_site_config (
  `id`               INTEGER NOT NULL,
  `site_name`        TEXT     NOT NULL DEFAULT '',
  `site_logo`        TEXT    NOT NULL DEFAULT '',
  `login_bg`         TEXT    NOT NULL DEFAULT '',
  `login_desc`       TEXT    NOT NULL DEFAULT '',
  `login_sub_title` TEXT    NOT NULL DEFAULT '',
  `register_enabled` INTEGER         NOT NULL DEFAULT 0,
  `create_by`        INTEGER NOT NULL DEFAULT 0,
  `create_time`      DATETIME        NOT NULL DEFAULT (datetime('now','localtime')),
  `update_by`        INTEGER NOT NULL DEFAULT 0,
  `update_time`      DATETIME        NULL DEFAULT NULL,
  `deleted`          INTEGER          NOT NULL DEFAULT 0,
  `remark`           TEXT    NOT NULL DEFAULT '',
  PRIMARY KEY (`id`)
);

INSERT OR IGNORE INTO `sys_site_config`
  (`id`, `site_name`, `site_logo`, `login_bg`, `login_desc`, `register_enabled`, `remark`)
VALUES
  (1, 'Web Basic Admin', '', '', '一款开箱即用的 Go + Vue3 后台管理脚手架', 0, '站点个性化配置（单行）');

INSERT OR IGNORE INTO `sys_site_config` (`id`) VALUES (1);
UPDATE `sys_site_config` SET `login_sub_title` = '美观实用的界面，经过视觉优化，确保卓越的用户体验' WHERE `id` = 1 AND `login_sub_title` = '';

-- 菜单：系统设置 → 站点设置
INSERT OR IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1150, 1300, '站点设置', 2, 'system:site:list',   'site', 'system/site/index', 'ri:palette-line', 8, 1, 1, 1, 1, datetime('now','localtime'), '网站名称 / logo / 登录页 / 注册开关'),
  (1151, 1150, '保存配置', 3, 'system:site:update', '', '', '', 1, 1, 1, 1, 1, datetime('now','localtime'), '');

INSERT OR IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1150), (1, 1151);
