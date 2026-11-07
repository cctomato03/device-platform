-- 站点个性化配置（单行表：id 恒为 1）+ 菜单 + 授权
-- 支持：网站名称、logo、登录页背景图、登录页描述/副标题、注册开关

CREATE TABLE IF NOT EXISTS `sys_site_config` (
  `id`               BIGINT UNSIGNED NOT NULL COMMENT '主键（固定为 1）',
  `site_name`        VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '网站名称',
  `site_logo`        VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '站点 logo 地址（空表示用内置图标）',
  `login_bg`         VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '登录页背景图地址（空表示用默认插画）',
  `login_desc`       VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '登录页文字描述',
  `login_sub_title` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '登录页副标题（留空用默认文案）',
  `register_enabled` TINYINT         NOT NULL DEFAULT 0 COMMENT '是否开放注册：1 开放 0 关闭',
  `create_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `create_time`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `update_time`      DATETIME        NULL DEFAULT NULL,
  `deleted`          BIGINT          NOT NULL DEFAULT 0,
  `remark`           VARCHAR(255)    NOT NULL DEFAULT '',
  PRIMARY KEY (`id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = '站点个性化配置';

INSERT IGNORE INTO `sys_site_config`
  (`id`, `site_name`, `site_logo`, `login_bg`, `login_desc`, `register_enabled`, `remark`)
VALUES
  (1, 'Web Basic Admin', '', '', '一款开箱即用的 Go + Vue3 后台管理脚手架', 0, '站点个性化配置（单行）');

INSERT IGNORE INTO `sys_site_config` (`id`) VALUES (1);
UPDATE `sys_site_config` SET `login_sub_title` = '美观实用的界面，经过视觉优化，确保卓越的用户体验' WHERE `id` = 1 AND `login_sub_title` = '';

-- 菜单：系统设置 → 站点设置
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (1150, 1300, '站点设置', 2, 'system:site:list',   'site', 'system/site/index', 'ri:palette-line', 8, 1, 1, 1, 1, NOW(), '网站名称 / logo / 登录页 / 注册开关'),
  (1151, 1150, '保存配置', 3, 'system:site:update', '', '', '', 1, 1, 1, 1, 1, NOW(), '');

INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (1, 1150), (1, 1151);
