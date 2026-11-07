-- =============================================================================
-- 000001_init_schema : 初始化表结构
-- 兼容 MySQL 5.7 / 8.0+：显式指定 utf8mb4_general_ci，禁用 CTE/窗口函数/函数索引
-- =============================================================================

SET NAMES utf8mb4;

-- ---------------------------- 租户 ----------------------------
CREATE TABLE IF NOT EXISTS sys_tenant (
  id            BIGINT UNSIGNED NOT NULL COMMENT '主键',
  name          VARCHAR(100) NOT NULL COMMENT '租户名称',
  code          VARCHAR(64)  NOT NULL COMMENT '租户标识',
  contact_name  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '联系人',
  contact_phone VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '联系电话',
  max_users     INT          NOT NULL DEFAULT 0 COMMENT '用户数上限,0不限',
  expire_at     VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '到期时间',
  status        TINYINT      NOT NULL DEFAULT 1 COMMENT '状态 1正常 0停用',
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_code (code, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='租户';

-- ---------------------------- 部门 ----------------------------
CREATE TABLE IF NOT EXISTS sys_dept (
  id          BIGINT UNSIGNED NOT NULL COMMENT '主键',
  parent_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上级部门,0为顶级',
  ancestors   VARCHAR(500) NOT NULL DEFAULT '0' COMMENT '祖级路径,如 0,1,3',
  name        VARCHAR(64)  NOT NULL COMMENT '部门/公司名称',
  code        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '部门编码',
  leader      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '负责人',
  phone       VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '联系电话',
  email       VARCHAR(128) NOT NULL DEFAULT '' COMMENT '邮箱',
  sort        INT          NOT NULL DEFAULT 0 COMMENT '显示顺序',
  status      TINYINT      NOT NULL DEFAULT 1 COMMENT '状态 1正常 0停用',
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  create_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_parent (parent_id),
  KEY idx_ancestors (ancestors(191)),
  KEY idx_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='部门/公司';

-- ---------------------------- 用户 ----------------------------
CREATE TABLE IF NOT EXISTS sys_user (
  id              BIGINT UNSIGNED NOT NULL COMMENT '主键',
  username        VARCHAR(64)  NOT NULL COMMENT '登录账号',
  nickname        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '昵称/姓名',
  password        VARCHAR(100) NOT NULL COMMENT 'bcrypt 密码哈希',
  salt            VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '预留盐',
  dept_id         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属部门',
  mobile          VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '手机号',
  email           VARCHAR(128) NOT NULL DEFAULT '' COMMENT '邮箱',
  sex             TINYINT      NOT NULL DEFAULT 2 COMMENT '性别 1男 2女 0未知',
  avatar          VARCHAR(500) NOT NULL DEFAULT '' COMMENT '头像',
  status          TINYINT      NOT NULL DEFAULT 1 COMMENT '状态 1正常 0停用',
  last_login_ip   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '最后登录IP',
  last_login_time DATETIME     NULL COMMENT '最后登录时间',
  pwd_update_time DATETIME     NULL COMMENT '密码更新时间',
  fail_count      INT          NOT NULL DEFAULT 0 COMMENT '连续失败次数',
  locked_until    DATETIME     NULL COMMENT '锁定截止时间',
  is_super        TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否超管',
  tenant_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  create_by       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time     DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted         BIGINT       NOT NULL DEFAULT 0,
  remark          VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_username (username, deleted),
  KEY idx_dept (dept_id),
  KEY idx_mobile (mobile),
  KEY idx_tenant (tenant_id),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户';

-- ---------------------------- 角色 ----------------------------
CREATE TABLE IF NOT EXISTS sys_role (
  id          BIGINT UNSIGNED NOT NULL,
  name        VARCHAR(64) NOT NULL COMMENT '角色名称',
  code        VARCHAR(64) NOT NULL COMMENT '角色标识',
  sort        INT         NOT NULL DEFAULT 0,
  data_scope  TINYINT     NOT NULL DEFAULT 4 COMMENT '数据范围 1全部 2本部门 3本部门及以下 4仅本人 5自定义',
  status      TINYINT     NOT NULL DEFAULT 1,
  is_builtin  TINYINT(1)  NOT NULL DEFAULT 0 COMMENT '内置角色不可删除',
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time DATETIME    NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT      NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_code (code, deleted),
  KEY idx_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色';

-- ---------------------------- 菜单与权限点 ----------------------------
CREATE TABLE IF NOT EXISTS sys_menu (
  id          BIGINT UNSIGNED NOT NULL,
  parent_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  name        VARCHAR(64)  NOT NULL COMMENT '名称',
  type        TINYINT      NOT NULL DEFAULT 1 COMMENT '1目录 2菜单 3按钮 4接口',
  permission  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '权限码',
  path        VARCHAR(200) NOT NULL DEFAULT '' COMMENT '路由地址',
  component   VARCHAR(200) NOT NULL DEFAULT '' COMMENT '组件路径',
  redirect    VARCHAR(200) NOT NULL DEFAULT '',
  icon        VARCHAR(64)  NOT NULL DEFAULT '',
  sort        INT          NOT NULL DEFAULT 0,
  visible     TINYINT(1)   NOT NULL DEFAULT 1,
  keep_alive  TINYINT(1)   NOT NULL DEFAULT 1,
  affix       TINYINT(1)   NOT NULL DEFAULT 0,
  is_frame    TINYINT(1)   NOT NULL DEFAULT 0,
  is_public   TINYINT(1)   NOT NULL DEFAULT 0,
  api_method  VARCHAR(10)  NOT NULL DEFAULT '',
  api_path    VARCHAR(200) NOT NULL DEFAULT '',
  status      TINYINT      NOT NULL DEFAULT 1,
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_parent (parent_id),
  KEY idx_permission (permission),
  KEY idx_type (type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='菜单与权限点';

-- ---------------------------- 关联表 ----------------------------
CREATE TABLE IF NOT EXISTS sys_user_role (
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, role_id),
  KEY idx_role (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户角色关联';

CREATE TABLE IF NOT EXISTS sys_role_menu (
  role_id BIGINT UNSIGNED NOT NULL,
  menu_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, menu_id),
  KEY idx_menu (menu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色权限关联';

CREATE TABLE IF NOT EXISTS sys_role_dept (
  role_id BIGINT UNSIGNED NOT NULL,
  dept_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, dept_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色自定义数据权限';

-- ---------------------------- 岗位 ----------------------------
CREATE TABLE IF NOT EXISTS sys_post (
  id          BIGINT UNSIGNED NOT NULL,
  code        VARCHAR(64) NOT NULL COMMENT '岗位编码',
  name        VARCHAR(64) NOT NULL COMMENT '岗位名称',
  sort        INT         NOT NULL DEFAULT 0,
  status      TINYINT     NOT NULL DEFAULT 1,
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME    NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT      NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_code (code, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='岗位';

CREATE TABLE IF NOT EXISTS sys_user_post (
  user_id BIGINT UNSIGNED NOT NULL,
  post_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, post_id),
  KEY idx_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户岗位关联';

-- ---------------------------- 字典 ----------------------------
CREATE TABLE IF NOT EXISTS sys_dict_type (
  id          BIGINT UNSIGNED NOT NULL,
  name        VARCHAR(100) NOT NULL COMMENT '字典名称',
  code        VARCHAR(100) NOT NULL COMMENT '字典编码',
  status      TINYINT      NOT NULL DEFAULT 1,
  is_builtin  TINYINT(1)   NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_code (code, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='字典类型';

CREATE TABLE IF NOT EXISTS sys_dict_data (
  id          BIGINT UNSIGNED NOT NULL,
  dict_code   VARCHAR(100) NOT NULL COMMENT '所属字典编码',
  label       VARCHAR(100) NOT NULL COMMENT '标签',
  value       VARCHAR(100) NOT NULL COMMENT '值',
  sort        INT          NOT NULL DEFAULT 0,
  tag_type    VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '标签颜色',
  css_class   VARCHAR(100) NOT NULL DEFAULT '',
  is_default  TINYINT(1)   NOT NULL DEFAULT 0,
  status      TINYINT      NOT NULL DEFAULT 1,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_dict_code (dict_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='字典数据';

-- ---------------------------- 参数 ----------------------------
CREATE TABLE IF NOT EXISTS sys_config (
  id          BIGINT UNSIGNED NOT NULL,
  name        VARCHAR(100) NOT NULL COMMENT '参数名称',
  config_key  VARCHAR(100) NOT NULL COMMENT '参数键',
  value       TEXT         NOT NULL COMMENT '参数值',
  value_type  VARCHAR(20)  NOT NULL DEFAULT 'string' COMMENT 'string/int/bool/json',
  is_builtin  TINYINT(1)   NOT NULL DEFAULT 0,
  is_public   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否允许未登录读取',
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uniq_key (config_key, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='系统参数';

-- ---------------------------- 文件 ----------------------------
CREATE TABLE IF NOT EXISTS sys_file (
  id           BIGINT UNSIGNED NOT NULL,
  storage_type VARCHAR(20)  NOT NULL DEFAULT 'local',
  bucket       VARCHAR(100) NOT NULL DEFAULT '',
  object_key   VARCHAR(500) NOT NULL,
  origin_name  VARCHAR(255) NOT NULL,
  ext          VARCHAR(20)  NOT NULL DEFAULT '',
  mime         VARCHAR(100) NOT NULL DEFAULT '',
  size         BIGINT       NOT NULL DEFAULT 0,
  hash         CHAR(64)     NOT NULL DEFAULT '' COMMENT 'SHA-256 秒传',
  url          VARCHAR(1000) NOT NULL DEFAULT '',
  is_private   TINYINT(1)   NOT NULL DEFAULT 0,
  biz_type     VARCHAR(50)  NOT NULL DEFAULT '',
  biz_id       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1临时 2已确认',
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_hash (hash),
  KEY idx_biz (biz_type, biz_id),
  KEY idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='文件';

-- ---------------------------- 定时任务 ----------------------------
CREATE TABLE IF NOT EXISTS sys_job (
  id          BIGINT UNSIGNED NOT NULL,
  name        VARCHAR(100) NOT NULL,
  handler     VARCHAR(100) NOT NULL DEFAULT '' COMMENT '内部Handler名',
  job_type    TINYINT      NOT NULL DEFAULT 1 COMMENT '1内部方法 2HTTP',
  target      VARCHAR(500) NOT NULL DEFAULT '',
  http_method VARCHAR(10)  NOT NULL DEFAULT 'POST',
  cron        VARCHAR(64)  NOT NULL,
  params      TEXT         NOT NULL,
  concurrent  TINYINT      NOT NULL DEFAULT 1 COMMENT '1禁止 2允许 3丢弃',
  timeout_sec INT          NOT NULL DEFAULT 300,
  retry_times INT          NOT NULL DEFAULT 0,
  status      TINYINT      NOT NULL DEFAULT 1,
  next_run_at VARCHAR(30)  NOT NULL DEFAULT '',
  tenant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='定时任务';

CREATE TABLE IF NOT EXISTS sys_job_log (
  id           BIGINT UNSIGNED NOT NULL,
  job_id       BIGINT UNSIGNED NOT NULL,
  job_name     VARCHAR(100) NOT NULL DEFAULT '',
  trigger_type TINYINT      NOT NULL DEFAULT 1 COMMENT '1自动 2手动',
  status       TINYINT      NOT NULL DEFAULT 0 COMMENT '0执行中 1成功 2失败 3超时',
  start_time   VARCHAR(30)  NOT NULL DEFAULT '',
  end_time     VARCHAR(30)  NOT NULL DEFAULT '',
  cost_ms      INT          NOT NULL DEFAULT 0,
  result       TEXT         NOT NULL,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_job (job_id, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='任务执行日志';

-- ---------------------------- 日志 ----------------------------
CREATE TABLE IF NOT EXISTS sys_login_log (
  id         BIGINT UNSIGNED NOT NULL,
  username   VARCHAR(64)  NOT NULL DEFAULT '',
  login_type TINYINT      NOT NULL DEFAULT 1 COMMENT '1账号密码 2短信 3第三方',
  status     TINYINT      NOT NULL DEFAULT 1 COMMENT '1成功 0失败',
  msg        VARCHAR(500) NOT NULL DEFAULT '',
  ip         VARCHAR(64)  NOT NULL DEFAULT '',
  location   VARCHAR(100) NOT NULL DEFAULT '',
  user_agent VARCHAR(500) NOT NULL DEFAULT '',
  tenant_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  login_time VARCHAR(30)  NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_login_time (login_time),
  KEY idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='登录日志';

CREATE TABLE IF NOT EXISTS sys_oper_log (
  id            BIGINT UNSIGNED NOT NULL,
  trace_id      VARCHAR(64)  NOT NULL DEFAULT '',
  title         VARCHAR(100) NOT NULL DEFAULT '',
  method        VARCHAR(200) NOT NULL DEFAULT '',
  request_uri   VARCHAR(500) NOT NULL DEFAULT '',
  http_method   VARCHAR(10)  NOT NULL DEFAULT '',
  oper_user_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  oper_name     VARCHAR(64)  NOT NULL DEFAULT '',
  dept_name     VARCHAR(64)  NOT NULL DEFAULT '',
  oper_ip       VARCHAR(64)  NOT NULL DEFAULT '',
  oper_location VARCHAR(100) NOT NULL DEFAULT '',
  params        TEXT         NOT NULL,
  result        TEXT         NOT NULL,
  status        TINYINT      NOT NULL DEFAULT 1,
  error_msg     VARCHAR(2000) NOT NULL DEFAULT '',
  cost_ms       INT          NOT NULL DEFAULT 0,
  oper_time     VARCHAR(30)  NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_oper_time (oper_time),
  KEY idx_user (oper_user_id),
  KEY idx_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='操作日志';
