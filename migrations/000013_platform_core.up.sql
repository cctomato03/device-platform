-- =============================================================================
-- 000013_platform_core : 设备平台核心表（插件 / 告警 / 指标 / 日志 / 网络策略）
-- 方言：SQLite（无 AUTO_INCREMENT，主键由雪花 ID 生成）
-- =============================================================================

-- ---------------------------- 插件实例 ----------------------------
CREATE TABLE IF NOT EXISTS sys_plugin (
  id           INTEGER NOT NULL,
  key          TEXT NOT NULL,
  name         TEXT NOT NULL,
  version      TEXT NOT NULL DEFAULT '',
  vendor       TEXT NOT NULL DEFAULT '',
  category     TEXT NOT NULL DEFAULT '',
  description  TEXT NOT NULL DEFAULT '',
  icon         TEXT NOT NULL DEFAULT '',
  runtime      TEXT NOT NULL DEFAULT 'process',
  status       TEXT NOT NULL DEFAULT 'installed',
  enabled      INTEGER NOT NULL DEFAULT 1,
  auto_start   INTEGER NOT NULL DEFAULT 1,
  config       TEXT NOT NULL DEFAULT '{}',
  manifest     TEXT NOT NULL DEFAULT '{}',
  install_path TEXT NOT NULL DEFAULT '',
  data_path    TEXT NOT NULL DEFAULT '',
  pid          INTEGER NOT NULL DEFAULT 0,
  last_error   TEXT NOT NULL DEFAULT '',
  started_at   DATETIME NULL,
  installed_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  size_bytes   INTEGER NOT NULL DEFAULT 0,
  tenant_id    INTEGER NOT NULL DEFAULT 0,
  create_by    INTEGER NOT NULL DEFAULT 0,
  create_time  DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by    INTEGER NOT NULL DEFAULT 0,
  update_time  DATETIME NULL,
  deleted      INTEGER NOT NULL DEFAULT 0,
  remark       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_plugin_uk_key ON sys_plugin (key, deleted);
CREATE INDEX IF NOT EXISTS idx_sys_plugin_status ON sys_plugin (status);

-- ---------------------------- 告警规则 ----------------------------
CREATE TABLE IF NOT EXISTS sys_alert_rule (
  id           INTEGER NOT NULL,
  name         TEXT NOT NULL,
  metric       TEXT NOT NULL DEFAULT 'cpu',
  operator     TEXT NOT NULL DEFAULT '>',
  threshold    REAL NOT NULL DEFAULT 80,
  duration_sec INTEGER NOT NULL DEFAULT 60,
  level        INTEGER NOT NULL DEFAULT 2,
  enabled      INTEGER NOT NULL DEFAULT 1,
  silence_sec  INTEGER NOT NULL DEFAULT 600,
  tenant_id    INTEGER NOT NULL DEFAULT 0,
  create_by    INTEGER NOT NULL DEFAULT 0,
  create_time  DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by    INTEGER NOT NULL DEFAULT 0,
  update_time  DATETIME NULL,
  deleted      INTEGER NOT NULL DEFAULT 0,
  remark       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_alert_rule_metric ON sys_alert_rule (metric);

-- ---------------------------- 告警记录 ----------------------------
CREATE TABLE IF NOT EXISTS sys_alert_record (
  id         INTEGER NOT NULL,
  rule_id    INTEGER NOT NULL DEFAULT 0,
  rule_name  TEXT NOT NULL DEFAULT '',
  metric     TEXT NOT NULL DEFAULT '',
  level      INTEGER NOT NULL DEFAULT 1,
  value      REAL NOT NULL DEFAULT 0,
  threshold  REAL NOT NULL DEFAULT 0,
  message    TEXT NOT NULL DEFAULT '',
  status     INTEGER NOT NULL DEFAULT 1,
  fired_at   TEXT NOT NULL DEFAULT '',
  recover_at TEXT NOT NULL DEFAULT '',
  tenant_id  INTEGER NOT NULL DEFAULT 0,
  create_by  INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by  INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME NULL,
  deleted    INTEGER NOT NULL DEFAULT 0,
  remark     TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_alert_record_time ON sys_alert_record (fired_at);
CREATE INDEX IF NOT EXISTS idx_sys_alert_record_status ON sys_alert_record (status);

-- ---------------------------- 指标历史 ----------------------------
CREATE TABLE IF NOT EXISTS sys_metric_sample (
  id           INTEGER NOT NULL,
  sampled_at   TEXT NOT NULL DEFAULT '',
  cpu_percent  REAL NOT NULL DEFAULT 0,
  mem_percent  REAL NOT NULL DEFAULT 0,
  disk_percent REAL NOT NULL DEFAULT 0,
  net_in_bps   REAL NOT NULL DEFAULT 0,
  net_out_bps  REAL NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_metric_sample_time ON sys_metric_sample (sampled_at);

-- ---------------------------- 访问日志 ----------------------------
CREATE TABLE IF NOT EXISTS sys_access_log (
  id          INTEGER NOT NULL,
  trace_id    TEXT NOT NULL DEFAULT '',
  ip          TEXT NOT NULL DEFAULT '',
  method      TEXT NOT NULL DEFAULT '',
  path        TEXT NOT NULL DEFAULT '',
  status      INTEGER NOT NULL DEFAULT 0,
  cost_ms     INTEGER NOT NULL DEFAULT 0,
  bytes_out   INTEGER NOT NULL DEFAULT 0,
  user_agent  TEXT NOT NULL DEFAULT '',
  user_id     INTEGER NOT NULL DEFAULT 0,
  access_time TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_access_log_time ON sys_access_log (access_time);
CREATE INDEX IF NOT EXISTS idx_sys_access_log_ip ON sys_access_log (ip);

-- ---------------------------- 流量日志 ----------------------------
CREATE TABLE IF NOT EXISTS sys_traffic_log (
  id          INTEGER NOT NULL,
  iface       TEXT NOT NULL DEFAULT '',
  bytes_in    INTEGER NOT NULL DEFAULT 0,
  bytes_out   INTEGER NOT NULL DEFAULT 0,
  packets_in  INTEGER NOT NULL DEFAULT 0,
  packets_out INTEGER NOT NULL DEFAULT 0,
  in_bps      REAL NOT NULL DEFAULT 0,
  out_bps     REAL NOT NULL DEFAULT 0,
  logged_at   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_traffic_log_time ON sys_traffic_log (logged_at);

-- ---------------------------- 静态路由 ----------------------------
CREATE TABLE IF NOT EXISTS sys_static_route (
  id          INTEGER NOT NULL,
  destination TEXT NOT NULL,
  gateway     TEXT NOT NULL DEFAULT '',
  device      TEXT NOT NULL DEFAULT '',
  metric      INTEGER NOT NULL DEFAULT 0,
  table_name  TEXT NOT NULL DEFAULT 'main',
  enabled     INTEGER NOT NULL DEFAULT 1,
  applied     INTEGER NOT NULL DEFAULT 0,
  tenant_id   INTEGER NOT NULL DEFAULT 0,
  create_by   INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by   INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME NULL,
  deleted     INTEGER NOT NULL DEFAULT 0,
  remark      TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_static_route_uk ON sys_static_route (destination, gateway, deleted);

-- ---------------------------- NAT 策略 ----------------------------
CREATE TABLE IF NOT EXISTS sys_nat_rule (
  id         INTEGER NOT NULL,
  name       TEXT NOT NULL,
  rule_type  TEXT NOT NULL DEFAULT 'snat',
  protocol   TEXT NOT NULL DEFAULT 'tcp',
  src_iface  TEXT NOT NULL DEFAULT '',
  dst_iface  TEXT NOT NULL DEFAULT '',
  src_addr   TEXT NOT NULL DEFAULT '',
  dst_addr   TEXT NOT NULL DEFAULT '',
  dst_port   INTEGER NOT NULL DEFAULT 0,
  to_addr    TEXT NOT NULL DEFAULT '',
  to_port    INTEGER NOT NULL DEFAULT 0,
  enabled    INTEGER NOT NULL DEFAULT 1,
  applied    INTEGER NOT NULL DEFAULT 0,
  tenant_id  INTEGER NOT NULL DEFAULT 0,
  create_by  INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by  INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME NULL,
  deleted    INTEGER NOT NULL DEFAULT 0,
  remark     TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);

-- ---------------------------- 安全策略 ----------------------------
CREATE TABLE IF NOT EXISTS sys_secure_policy (
  id         INTEGER NOT NULL,
  name       TEXT NOT NULL,
  direction  TEXT NOT NULL DEFAULT 'in',
  action     TEXT NOT NULL DEFAULT 'accept',
  protocol   TEXT NOT NULL DEFAULT 'tcp',
  src_addr   TEXT NOT NULL DEFAULT '',
  dst_addr   TEXT NOT NULL DEFAULT '',
  port       INTEGER NOT NULL DEFAULT 0,
  priority   INTEGER NOT NULL DEFAULT 100,
  enabled    INTEGER NOT NULL DEFAULT 1,
  applied    INTEGER NOT NULL DEFAULT 0,
  tenant_id  INTEGER NOT NULL DEFAULT 0,
  create_by  INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by  INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME NULL,
  deleted    INTEGER NOT NULL DEFAULT 0,
  remark     TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_sys_secure_policy_priority ON sys_secure_policy (priority);

-- ---------------------------- QoS 规则 ----------------------------
CREATE TABLE IF NOT EXISTS sys_qos_rule (
  id         INTEGER NOT NULL,
  name       TEXT NOT NULL,
  iface      TEXT NOT NULL DEFAULT '',
  direction  TEXT NOT NULL DEFAULT 'egress',
  rate_kbps  INTEGER NOT NULL DEFAULT 0,
  burst_kb   INTEGER NOT NULL DEFAULT 0,
  target     TEXT NOT NULL DEFAULT '',
  enabled    INTEGER NOT NULL DEFAULT 1,
  applied    INTEGER NOT NULL DEFAULT 0,
  tenant_id  INTEGER NOT NULL DEFAULT 0,
  create_by  INTEGER NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by  INTEGER NOT NULL DEFAULT 0,
  update_time DATETIME NULL,
  deleted    INTEGER NOT NULL DEFAULT 0,
  remark     TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);

-- ---------------------------- 设备本地设置（键值） ----------------------------
CREATE TABLE IF NOT EXISTS sys_device_setting (
  setting_key   TEXT NOT NULL PRIMARY KEY,
  setting_value TEXT NOT NULL DEFAULT '',
  updated_at    DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);

INSERT OR IGNORE INTO sys_device_setting (setting_key, setting_value) VALUES
  ('device.name',        ''),
  ('device.timezone',    'Asia/Shanghai'),
  ('device.ntp_enabled', '1'),
  ('device.ntp_servers', 'ntp.aliyun.com,pool.ntp.org'),
  ('device.dns',         '223.5.5.5,114.114.114.114'),
  ('device.sniff_enabled', '0'),
  ('device.sniff_iface', ''),
  ('device.remark',      '');
