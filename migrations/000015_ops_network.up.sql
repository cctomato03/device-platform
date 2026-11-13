-- =============================================================================
-- 000015_ops_network.up.sql : 网络/设备运维能力落库
--   · 网卡配置（netplan：以太网 / 网桥 / 端口聚合）
--   · 策略路由（ip rule + 路由表）
--   · DHCP 服务 / DNS 服务（dnsmasq）
--   · 动态路由 BGP / OSPF（FRR）
--   · SNMP 服务（net-snmp）
--   · 插件市场源（在线索引）
-- 方言：SQLite
-- =============================================================================

-- ---------------------------- 网卡配置（netplan 片段） ----------------------------
CREATE TABLE IF NOT EXISTS sys_iface_config (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'ethernet',
  dhcp4       INTEGER NOT NULL DEFAULT 0,
  addresses   TEXT NOT NULL DEFAULT '',
  gateway     TEXT NOT NULL DEFAULT '',
  mtu         INTEGER NOT NULL DEFAULT 0,
  members     TEXT NOT NULL DEFAULT '',
  bond_mode   TEXT NOT NULL DEFAULT 'active-backup',
  stp         INTEGER NOT NULL DEFAULT 0,
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_iface_config_uk ON sys_iface_config (name, deleted);

-- ---------------------------- 策略路由 ----------------------------
CREATE TABLE IF NOT EXISTS sys_policy_route (
  id          INTEGER NOT NULL,
  name        TEXT NOT NULL,
  priority    INTEGER NOT NULL DEFAULT 10000,
  from_addr   TEXT NOT NULL DEFAULT '',
  to_addr     TEXT NOT NULL DEFAULT '',
  fwmark      INTEGER NOT NULL DEFAULT 0,
  iif         TEXT NOT NULL DEFAULT '',
  oif         TEXT NOT NULL DEFAULT '',
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_policy_route_uk ON sys_policy_route (priority, deleted);

-- ---------------------------- DHCP 服务（单行 id=1） ----------------------------
CREATE TABLE IF NOT EXISTS sys_dhcp_config (
  id              INTEGER NOT NULL,
  enabled         INTEGER NOT NULL DEFAULT 0,
  iface           TEXT NOT NULL DEFAULT '',
  range_start     TEXT NOT NULL DEFAULT '',
  range_end       TEXT NOT NULL DEFAULT '',
  netmask         TEXT NOT NULL DEFAULT '255.255.255.0',
  lease_hours     INTEGER NOT NULL DEFAULT 12,
  gateway         TEXT NOT NULL DEFAULT '',
  dns             TEXT NOT NULL DEFAULT '',
  domain          TEXT NOT NULL DEFAULT '',
  static_bindings TEXT NOT NULL DEFAULT '[]',
  applied         INTEGER NOT NULL DEFAULT 0,
  tenant_id       INTEGER NOT NULL DEFAULT 0,
  create_by       INTEGER NOT NULL DEFAULT 0,
  create_time     DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by       INTEGER NOT NULL DEFAULT 0,
  update_time     DATETIME NULL,
  deleted         INTEGER NOT NULL DEFAULT 0,
  remark          TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
INSERT OR IGNORE INTO sys_dhcp_config (id, enabled) VALUES (1, 0);

-- ---------------------------- DNS 服务（单行 id=1） ----------------------------
CREATE TABLE IF NOT EXISTS sys_dns_server_config (
  id          INTEGER NOT NULL,
  enabled     INTEGER NOT NULL DEFAULT 0,
  listen_addr TEXT NOT NULL DEFAULT '0.0.0.0',
  upstreams   TEXT NOT NULL DEFAULT '223.5.5.5',
  records     TEXT NOT NULL DEFAULT '[]',
  cache_size  INTEGER NOT NULL DEFAULT 1000,
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
INSERT OR IGNORE INTO sys_dns_server_config (id, enabled) VALUES (1, 0);

-- ---------------------------- 动态路由（BGP / OSPF） ----------------------------
CREATE TABLE IF NOT EXISTS sys_routing_config (
  id          INTEGER NOT NULL,
  protocol    TEXT NOT NULL DEFAULT 'bgp',
  enabled     INTEGER NOT NULL DEFAULT 0,
  asn         INTEGER NOT NULL DEFAULT 0,
  router_id   TEXT NOT NULL DEFAULT '',
  neighbors   TEXT NOT NULL DEFAULT '[]',
  networks    TEXT NOT NULL DEFAULT '',
  area        TEXT NOT NULL DEFAULT '0',
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
-- 说明：动态路由是「每协议一行」的配置表，从不软删除，因此唯一索引只按 protocol 建，
-- 这样 ON CONFLICT(protocol) 的 upsert 才能命中索引。
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_routing_config_uk ON sys_routing_config (protocol);

-- ---------------------------- SNMP 服务（单行 id=1） ----------------------------
CREATE TABLE IF NOT EXISTS sys_snmp_config (
  id           INTEGER NOT NULL,
  enabled      INTEGER NOT NULL DEFAULT 0,
  version      TEXT NOT NULL DEFAULT '2c',
  community    TEXT NOT NULL DEFAULT 'public',
  v3_user      TEXT NOT NULL DEFAULT '',
  v3_password  TEXT NOT NULL DEFAULT '',
  location     TEXT NOT NULL DEFAULT '',
  contact      TEXT NOT NULL DEFAULT '',
  listen_addr  TEXT NOT NULL DEFAULT '0.0.0.0',
  allowed_host TEXT NOT NULL DEFAULT '',
  applied      INTEGER NOT NULL DEFAULT 0,
  tenant_id    INTEGER NOT NULL DEFAULT 0,
  create_by    INTEGER NOT NULL DEFAULT 0,
  create_time  DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by    INTEGER NOT NULL DEFAULT 0,
  update_time  DATETIME NULL,
  deleted      INTEGER NOT NULL DEFAULT 0,
  remark       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
INSERT OR IGNORE INTO sys_snmp_config (id, enabled, community) VALUES (1, 0, 'public');

-- ---------------------------- 插件市场源 ----------------------------
CREATE TABLE IF NOT EXISTS sys_plugin_source (
  id           INTEGER NOT NULL,
  name         TEXT NOT NULL,
  url          TEXT NOT NULL,
  trust_key    TEXT NOT NULL DEFAULT '',
  enabled      INTEGER NOT NULL DEFAULT 1,
  last_sync_at TEXT NOT NULL DEFAULT '',
  last_error   TEXT NOT NULL DEFAULT '',
  item_count   INTEGER NOT NULL DEFAULT 0,
  tenant_id    INTEGER NOT NULL DEFAULT 0,
  create_by    INTEGER NOT NULL DEFAULT 0,
  create_time  DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
  update_by    INTEGER NOT NULL DEFAULT 0,
  update_time  DATETIME NULL,
  deleted      INTEGER NOT NULL DEFAULT 0,
  remark       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_plugin_source_uk ON sys_plugin_source (url);

-- 默认内置一个官方源占位（默认禁用，填好 URL 再启用）
INSERT OR IGNORE INTO sys_plugin_source (id, name, url, enabled, remark, create_time)
VALUES (1, '官方插件源', '', 0, '填写索引地址（如 https://example.com/plugin-index.json）后启用', datetime('now','localtime'));
