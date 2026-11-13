-- =============================================================================
-- 000001_init_schema : 初始化表结构
-- 兼容 MySQL 5.7 / 8.0+：显式指定 utf8mb4_general_ci，禁用 CTE/窗口函数/函数索引
-- =============================================================================

-- ---------------------------- 租户 ----------------------------
CREATE TABLE IF NOT EXISTS sys_tenant (
  id            INTEGER NOT NULL,
  name          TEXT NOT NULL,
  code          TEXT  NOT NULL,
  contact_name  TEXT  NOT NULL DEFAULT '',
  contact_phone TEXT  NOT NULL DEFAULT '',
  max_users     INTEGER          NOT NULL DEFAULT 0,
  expire_at     TEXT  NOT NULL DEFAULT '',
  status        INTEGER      NOT NULL DEFAULT 1,
  create_by     INTEGER NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by     INTEGER NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL,
  deleted       INTEGER       NOT NULL DEFAULT 0,
  remark        TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_tenant_uniq_code ON sys_tenant (code, deleted);

-- ---------------------------- 部门 ----------------------------
CREATE TABLE IF NOT EXISTS sys_dept (
  id          INTEGER NOT NULL,
  parent_id   INTEGER NOT NULL DEFAULT 0,
  ancestors   TEXT NOT NULL DEFAULT '0',
  name        TEXT  NOT NULL,
  code        TEXT  NOT NULL DEFAULT '',
  leader      TEXT  NOT NULL DEFAULT '',
  phone       TEXT  NOT NULL DEFAULT '',
  email       TEXT NOT NULL DEFAULT '',
  sort        INTEGER          NOT NULL DEFAULT 0,
  status      INTEGER      NOT NULL DEFAULT 1,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_dept_idx_parent ON sys_dept (parent_id);
CREATE  INDEX IF NOT EXISTS idx_sys_dept_idx_ancestors ON sys_dept (ancestors);
CREATE  INDEX IF NOT EXISTS idx_sys_dept_idx_tenant ON sys_dept (tenant_id);

-- ---------------------------- 用户 ----------------------------
CREATE TABLE IF NOT EXISTS sys_user (
  id              INTEGER NOT NULL,
  username        TEXT  NOT NULL,
  nickname        TEXT  NOT NULL DEFAULT '',
  password        TEXT NOT NULL,
  salt            TEXT  NOT NULL DEFAULT '',
  dept_id         INTEGER NOT NULL DEFAULT 0,
  mobile          TEXT  NOT NULL DEFAULT '',
  email           TEXT NOT NULL DEFAULT '',
  sex             INTEGER      NOT NULL DEFAULT 2,
  avatar          TEXT NOT NULL DEFAULT '',
  status          INTEGER      NOT NULL DEFAULT 1,
  last_login_ip   TEXT  NOT NULL DEFAULT '',
  last_login_time DATETIME     NULL,
  pwd_update_time DATETIME     NULL,
  fail_count      INTEGER          NOT NULL DEFAULT 0,
  locked_until    DATETIME     NULL,
  is_super        INTEGER   NOT NULL DEFAULT 0,
  tenant_id       INTEGER NOT NULL DEFAULT 0,
  create_by       INTEGER NOT NULL DEFAULT 0,
  create_time     DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by       INTEGER NOT NULL DEFAULT 0,
  update_time     DATETIME     NULL,
  deleted         INTEGER       NOT NULL DEFAULT 0,
  remark          TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_user_uniq_username ON sys_user (username, deleted);
CREATE  INDEX IF NOT EXISTS idx_sys_user_idx_dept ON sys_user (dept_id);
CREATE  INDEX IF NOT EXISTS idx_sys_user_idx_mobile ON sys_user (mobile);
CREATE  INDEX IF NOT EXISTS idx_sys_user_idx_tenant ON sys_user (tenant_id);
CREATE  INDEX IF NOT EXISTS idx_sys_user_idx_status ON sys_user (status);

-- ---------------------------- 角色 ----------------------------
CREATE TABLE IF NOT EXISTS sys_role (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  code        TEXT NOT NULL,
  sort        INTEGER         NOT NULL DEFAULT 0,
  data_scope  INTEGER     NOT NULL DEFAULT 4,
  status      INTEGER     NOT NULL DEFAULT 1,
  is_builtin  INTEGER  NOT NULL DEFAULT 0,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME    NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME    NULL,
  deleted     INTEGER      NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_role_uniq_code ON sys_role (code, deleted);
CREATE  INDEX IF NOT EXISTS idx_sys_role_idx_tenant ON sys_role (tenant_id);

-- ---------------------------- 菜单与权限点 ----------------------------
CREATE TABLE IF NOT EXISTS sys_menu (
  id          INTEGER NOT NULL,
  parent_id   INTEGER NOT NULL DEFAULT 0,
  name        TEXT  NOT NULL,
  type        INTEGER      NOT NULL DEFAULT 1,
  permission  TEXT NOT NULL DEFAULT '',
  path        TEXT NOT NULL DEFAULT '',
  component   TEXT NOT NULL DEFAULT '',
  redirect    TEXT NOT NULL DEFAULT '',
  icon        TEXT  NOT NULL DEFAULT '',
  sort        INTEGER          NOT NULL DEFAULT 0,
  visible     INTEGER   NOT NULL DEFAULT 1,
  keep_alive  INTEGER   NOT NULL DEFAULT 1,
  affix       INTEGER   NOT NULL DEFAULT 0,
  is_frame    INTEGER   NOT NULL DEFAULT 0,
  is_public   INTEGER   NOT NULL DEFAULT 0,
  api_method  TEXT  NOT NULL DEFAULT '',
  api_path    TEXT NOT NULL DEFAULT '',
  status      INTEGER      NOT NULL DEFAULT 1,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_menu_idx_parent ON sys_menu (parent_id);
CREATE  INDEX IF NOT EXISTS idx_sys_menu_idx_permission ON sys_menu (permission);
CREATE  INDEX IF NOT EXISTS idx_sys_menu_idx_type ON sys_menu (type);

-- ---------------------------- 关联表 ----------------------------
CREATE TABLE IF NOT EXISTS sys_user_role (
  user_id INTEGER NOT NULL,
  role_id INTEGER NOT NULL,
  PRIMARY KEY (user_id, role_id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_user_role_idx_role ON sys_user_role (role_id);

CREATE TABLE IF NOT EXISTS sys_role_menu (
  role_id INTEGER NOT NULL,
  menu_id INTEGER NOT NULL,
  PRIMARY KEY (role_id, menu_id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_role_menu_idx_menu ON sys_role_menu (menu_id);

CREATE TABLE IF NOT EXISTS sys_role_dept (
  role_id INTEGER NOT NULL,
  dept_id INTEGER NOT NULL,
  PRIMARY KEY (role_id, dept_id)
);

-- ---------------------------- 岗位 ----------------------------
CREATE TABLE IF NOT EXISTS sys_post (
  id          INTEGER NOT NULL,
  code        TEXT NOT NULL,
  name        TEXT NOT NULL,
  sort        INTEGER         NOT NULL DEFAULT 0,
  status      INTEGER     NOT NULL DEFAULT 1,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME    NOT NULL DEFAULT (datetime('now','localtime')),
  update_time DATETIME    NULL,
  deleted     INTEGER      NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_post_uniq_code ON sys_post (code, deleted);

CREATE TABLE IF NOT EXISTS sys_user_post (
  user_id INTEGER NOT NULL,
  post_id INTEGER NOT NULL,
  PRIMARY KEY (user_id, post_id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_user_post_idx_post ON sys_user_post (post_id);

-- ---------------------------- 字典 ----------------------------
CREATE TABLE IF NOT EXISTS sys_dict_type (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  code        TEXT NOT NULL,
  status      INTEGER      NOT NULL DEFAULT 1,
  is_builtin  INTEGER   NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_dict_type_uniq_code ON sys_dict_type (code, deleted);

CREATE TABLE IF NOT EXISTS sys_dict_data (
  id          INTEGER NOT NULL,
  dict_code   TEXT NOT NULL,
  label       TEXT NOT NULL,
  value       TEXT NOT NULL,
  sort        INTEGER          NOT NULL DEFAULT 0,
  tag_type    TEXT  NOT NULL DEFAULT '',
  css_class   TEXT NOT NULL DEFAULT '',
  is_default  INTEGER   NOT NULL DEFAULT 0,
  status      INTEGER      NOT NULL DEFAULT 1,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_dict_data_idx_dict_code ON sys_dict_data (dict_code);

-- ---------------------------- 参数 ----------------------------
CREATE TABLE IF NOT EXISTS sys_config (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  config_key  TEXT NOT NULL,
  value       TEXT         NOT NULL,
  value_type  TEXT  NOT NULL DEFAULT 'string',
  is_builtin  INTEGER   NOT NULL DEFAULT 0,
  is_public   INTEGER   NOT NULL DEFAULT 0,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_config_uniq_key ON sys_config (config_key, deleted);

-- ---------------------------- 文件 ----------------------------
CREATE TABLE IF NOT EXISTS sys_file (
  id           INTEGER NOT NULL,
  storage_type TEXT  NOT NULL DEFAULT 'local',
  bucket       TEXT NOT NULL DEFAULT '',
  object_key   TEXT NOT NULL,
  origin_name  TEXT NOT NULL,
  ext          TEXT  NOT NULL DEFAULT '',
  mime         TEXT NOT NULL DEFAULT '',
  size         INTEGER       NOT NULL DEFAULT 0,
  hash         TEXT     NOT NULL DEFAULT '',
  url          TEXT NOT NULL DEFAULT '',
  is_private   INTEGER   NOT NULL DEFAULT 0,
  biz_type     TEXT  NOT NULL DEFAULT '',
  biz_id       INTEGER NOT NULL DEFAULT 0,
  status       INTEGER      NOT NULL DEFAULT 1,
  tenant_id    INTEGER NOT NULL DEFAULT 0,
  create_by    INTEGER NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by    INTEGER NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL,
  deleted      INTEGER       NOT NULL DEFAULT 0,
  remark       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_file_idx_hash ON sys_file (hash);
CREATE  INDEX IF NOT EXISTS idx_sys_file_idx_biz ON sys_file (biz_type, biz_id);
CREATE  INDEX IF NOT EXISTS idx_sys_file_idx_create_time ON sys_file (create_time);

-- ---------------------------- 定时任务 ----------------------------
CREATE TABLE IF NOT EXISTS sys_job (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  handler     TEXT NOT NULL DEFAULT '',
  job_type    INTEGER      NOT NULL DEFAULT 1,
  target      TEXT NOT NULL DEFAULT '',
  http_method TEXT  NOT NULL DEFAULT 'POST',
  cron        TEXT  NOT NULL,
  params      TEXT         NOT NULL,
  concurrent  INTEGER      NOT NULL DEFAULT 1,
  timeout_sec INTEGER          NOT NULL DEFAULT 300,
  retry_times INTEGER          NOT NULL DEFAULT 0,
  status      INTEGER      NOT NULL DEFAULT 1,
  next_run_at TEXT  NOT NULL DEFAULT '',
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME     NULL,
  deleted     INTEGER       NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sys_job_log (
  id           INTEGER NOT NULL,
  job_id       INTEGER NOT NULL,
  job_name     TEXT NOT NULL DEFAULT '',
  trigger_type INTEGER      NOT NULL DEFAULT 1,
  status       INTEGER      NOT NULL DEFAULT 0,
  start_time   TEXT  NOT NULL DEFAULT '',
  end_time     TEXT  NOT NULL DEFAULT '',
  cost_ms      INTEGER          NOT NULL DEFAULT 0,
  result       TEXT         NOT NULL,
  create_time  DATETIME     NOT NULL DEFAULT (datetime('now','localtime')),
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_job_log_idx_job ON sys_job_log (job_id, create_time);

-- ---------------------------- 日志 ----------------------------
CREATE TABLE IF NOT EXISTS sys_login_log (
  id         INTEGER NOT NULL,
  username   TEXT  NOT NULL DEFAULT '',
  login_type INTEGER      NOT NULL DEFAULT 1,
  status     INTEGER      NOT NULL DEFAULT 1,
  msg        TEXT NOT NULL DEFAULT '',
  ip         TEXT  NOT NULL DEFAULT '',
  location   TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  tenant_id  INTEGER NOT NULL DEFAULT 0,
  login_time TEXT  NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_login_log_idx_login_time ON sys_login_log (login_time);
CREATE  INDEX IF NOT EXISTS idx_sys_login_log_idx_username ON sys_login_log (username);

CREATE TABLE IF NOT EXISTS sys_oper_log (
  id            INTEGER NOT NULL,
  trace_id      TEXT  NOT NULL DEFAULT '',
  title         TEXT NOT NULL DEFAULT '',
  method        TEXT NOT NULL DEFAULT '',
  request_uri   TEXT NOT NULL DEFAULT '',
  http_method   TEXT  NOT NULL DEFAULT '',
  oper_user_id  INTEGER NOT NULL DEFAULT 0,
  tenant_id     INTEGER NOT NULL DEFAULT 0,
  oper_name     TEXT  NOT NULL DEFAULT '',
  dept_name     TEXT  NOT NULL DEFAULT '',
  oper_ip       TEXT  NOT NULL DEFAULT '',
  oper_location TEXT NOT NULL DEFAULT '',
  params        TEXT         NOT NULL,
  result        TEXT         NOT NULL,
  status        INTEGER      NOT NULL DEFAULT 1,
  error_msg     TEXT NOT NULL DEFAULT '',
  cost_ms       INTEGER          NOT NULL DEFAULT 0,
  oper_time     TEXT  NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE  INDEX IF NOT EXISTS idx_sys_oper_log_idx_oper_time ON sys_oper_log (oper_time);
CREATE  INDEX IF NOT EXISTS idx_sys_oper_log_idx_user ON sys_oper_log (oper_user_id);
CREATE  INDEX IF NOT EXISTS idx_sys_oper_log_idx_tenant ON sys_oper_log (tenant_id);
