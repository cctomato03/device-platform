#!/usr/bin/env bash
# =============================================================================
# 设备管理平台 · 一键安装脚本（Ubuntu 20.04/22.04，x86_64 / arm64）
#
# 特点：
#   1) 默认把「二进制 + 配置 + 数据库 + 日志 + 插件」全部装进同一个目录（默认 /opt/device-platform），
#      整机备份 / 迁移只需打包这一个目录；
#   2) 以 systemd 服务方式常驻，支持 --user 指定运行用户；
#   3) 交互式或全参数化安装，适合出厂预装脚本化调用。
#
# 用法：
#   sudo bash install.sh                          # 交互式安装（推荐首次使用）
#   sudo bash install.sh -y \
#        --dir /opt/device-platform \
#        --port 8090 \
#        --admin-password 'Admin@123456' \
#        --user root \
#        --no-firewalld
#
# 参数：
#   --dir PATH            安装目录（默认 /opt/device-platform）
#   --port PORT           Web 端口（默认 8090）
#   --admin-password PWD  初始管理员密码（默认随机生成并打印）
#   --user USER           运行服务的系统用户（默认 root，需要 root 才能下发网络/防火墙配置）
#   --service-name NAME   systemd 服务名（默认 device-platform）
#   --binary PATH         指定二进制文件（默认在脚本同级目录或 dist/ 下自动查找）
#   --arch ARCH           指定架构（默认按 uname -m 自动判断：x86_64→amd64，aarch64→arm64）
#   --no-firewalld        不尝试放行防火墙端口
#   --no-start            只安装不启动
#   -y|--yes              非交互模式，全部使用默认值/命令行传入值
#   --uninstall           卸载（保留数据目录，加 --purge 一并删除数据）
#   --purge               卸载时删除整个安装目录（含数据库、日志、插件）
#   --upgrade             升级：只替换二进制，保留数据与配置
# =============================================================================
set -euo pipefail

INSTALL_DIR="/opt/device-platform"
PORT="8090"
ADMIN_PASSWORD=""
RUN_USER="root"
SERVICE_NAME="device-platform"
BINARY=""
ARCH=""
OPEN_FIREWALL=1
DO_START=1
ASSUME_YES=0
ACTION="install"
PURGE=0

log()  { printf '\033[36m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[warn]\033[0m %s\n' "$*"; }
die()  { printf '\033[31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

# ------------------------------ 参数解析 ------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --dir) INSTALL_DIR="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --admin-password) ADMIN_PASSWORD="$2"; shift 2 ;;
    --user) RUN_USER="$2"; shift 2 ;;
    --service-name) SERVICE_NAME="$2"; shift 2 ;;
    --binary) BINARY="$2"; shift 2 ;;
    --arch) ARCH="$2"; shift 2 ;;
    --no-firewalld) OPEN_FIREWALL=0; shift ;;
    --no-start) DO_START=0; shift ;;
    -y|--yes) ASSUME_YES=1; shift ;;
    --uninstall) ACTION="uninstall"; shift ;;
    --upgrade) ACTION="upgrade"; shift ;;
    --purge) PURGE=1; shift ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    *) die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[ "$(id -u)" -eq 0 ] || die "请用 root 运行（sudo bash $0）"

# ------------------------------ 系统识别 ------------------------------
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l|armv6l) echo "arm" ;;
    *) die "不支持的 CPU 架构：$(uname -m)" ;;
  esac
}
[ -n "$ARCH" ] || ARCH="$(detect_arch)"
BIN_NAME="device-platform-linux-${ARCH}"

if [ -f /etc/os-release ]; then
  . /etc/os-release
  log "系统：${PRETTY_NAME:-unknown}  架构：${ARCH}"
fi

# ------------------------------ 卸载 ------------------------------
if [ "$ACTION" = "uninstall" ]; then
  log "停止并禁用服务 $SERVICE_NAME"
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  systemctl disable "$SERVICE_NAME" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload || true
  if [ "$PURGE" -eq 1 ]; then
    warn "删除安装目录 $INSTALL_DIR（含数据库/日志/插件）"
    rm -rf "$INSTALL_DIR"
  else
    log "保留数据目录：$INSTALL_DIR（如需彻底删除，请加 --purge 重新执行卸载）"
  fi
  log "卸载完成"
  exit 0
fi

# ------------------------------ 定位二进制 ------------------------------
if [ -z "$BINARY" ]; then
  for cand in \
    "$(dirname "$0")/$BIN_NAME" \
    "$(dirname "$0")/device-platform-server" \
    "$(dirname "$0")/../dist/$BIN_NAME" \
    "$(dirname "$0")/device-platform-linux-$ARCH"; do
    if [ -f "$cand" ]; then BINARY="$cand"; break; fi
  done
fi
[ -n "$BINARY" ] && [ -f "$BINARY" ] || die "未找到二进制文件，请用 --binary 指定，或把 $BIN_NAME 放在脚本同级目录"
log "使用二进制：$BINARY"

# ------------------------------ 交互式补充 ------------------------------
if [ "$ASSUME_YES" -eq 0 ] && [ "$ACTION" = "install" ]; then
  read -r -p "安装目录 [$INSTALL_DIR]: " ans; [ -n "${ans:-}" ] && INSTALL_DIR="$ans"
  read -r -p "Web 端口 [$PORT]: " ans; [ -n "${ans:-}" ] && PORT="$ans"
  read -r -p "运行用户（root 才能下发网络/防火墙配置）[$RUN_USER]: " ans; [ -n "${ans:-}" ] && RUN_USER="$ans"
  read -r -s -p "管理员初始密码（留空自动生成）: " ans; echo
  [ -n "${ans:-}" ] && ADMIN_PASSWORD="$ans"
fi

# ------------------------------ 目录准备 ------------------------------
if [ "$ACTION" = "upgrade" ]; then
  [ -d "$INSTALL_DIR" ] || die "升级失败：$INSTALL_DIR 不存在，请先安装"
fi

log "创建目录结构：$INSTALL_DIR"
mkdir -p "$INSTALL_DIR"/{bin,conf,data,logs,plugins,plugins-bundled,plugin-data,storage,backup,run,tmp}
mkdir -p "/etc/systemd/system"

# 停止旧服务（升级时避免文件占用）
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
  log "停止已有服务 $SERVICE_NAME"
  systemctl stop "$SERVICE_NAME"
fi

if [ "$ACTION" = "upgrade" ]; then
  cp -f "$INSTALL_DIR/bin/device-platform" "$INSTALL_DIR/backup/device-platform.$(date +%Y%m%d%H%M%S)" 2>/dev/null || true
fi

install -m 0755 "$BINARY" "$INSTALL_DIR/bin/device-platform"

# ------------------------------ 配置生成 ------------------------------
CONF="$INSTALL_DIR/conf/config.yaml"
if [ -f "$CONF" ] && [ "$ACTION" = "install" ]; then
  warn "已存在配置 $CONF，保留原配置（如需重置请先备份删除）"
fi

if [ ! -f "$CONF" ]; then
  if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 16)"
    log "已生成管理员初始密码：$ADMIN_PASSWORD"
  fi
  JWT_SECRET="$(head -c 48 /dev/urandom | base64 | tr -dc 'A-Za-z0-9')"
  cat > "$CONF" <<YAML
# 设备管理平台配置（由 install.sh 生成，可直接编辑后重启服务生效）
app:
  name: device-platform
  env: prod
  home: $INSTALL_DIR          # 数据根目录：数据库/日志/插件/上传文件都在这里
  port: $PORT
  mode: release
  timezone: Asia/Shanghai
  admin_password: "$ADMIN_PASSWORD"

database:
  driver: sqlite
  path: $INSTALL_DIR/data/device-platform.db
  busy_timeout_ms: 10000
  wal: true

jwt:
  secret: "$JWT_SECRET"
  access_ttl: 2h
  refresh_ttl: 168h

auth:
  captcha_enabled: true
  max_fail_count: 5
  lock_minutes: 10

storage:
  type: local
  local:
    base_dir: $INSTALL_DIR/storage
    url_prefix: /static

log:
  level: info
  dir: $INSTALL_DIR/logs
  max_size_mb: 50
  max_backups: 10
  max_age_days: 30

plugin:
  dir: $INSTALL_DIR/plugins
  data_dir: $INSTALL_DIR/plugin-data
  allow_docker: true
  auto_start: true

system:
  monitor_interval_sec: 10
  metric_retain_hours: 168
  tools_enabled: true
  log_retain_days: 30

metrics:
  enabled: true
YAML
  chmod 600 "$CONF"
  log "已生成配置：$CONF"
fi

# 内置插件：把随包附带的插件目录放进 plugins-bundled（市场里就会显示为「可安装」）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
for src in "$SCRIPT_DIR/plugins-bundled" "$SCRIPT_DIR/../plugins-src"; do
  if [ -d "$src" ]; then
    cp -r "$src"/* "$INSTALL_DIR/plugins-bundled/" 2>/dev/null || true
  fi
done

# ------------------------------ 运行用户 ------------------------------
if [ "$RUN_USER" != "root" ]; then
  if ! id -u "$RUN_USER" >/dev/null 2>&1; then
    warn "用户 $RUN_USER 不存在，自动创建系统用户"
    useradd --system --no-create-home --shell /usr/sbin/nologin "$RUN_USER"
  fi
  log "调整 $INSTALL_DIR 归属为 $RUN_USER"
  chown -R "$RUN_USER":"$RUN_USER" "$INSTALL_DIR"
fi

# ------------------------------ systemd ------------------------------
UNIT="/etc/systemd/system/${SERVICE_NAME}.service"
log "写入 systemd 单元：$UNIT"
cat > "$UNIT" <<UNITEOF
[Unit]
Description=Device Platform (设备管理平台)
Documentation=file://$INSTALL_DIR
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/device-platform -d $INSTALL_DIR
# 依赖数据目录内的配置；首次启动会自动生成
Environment=DEVPLAT_HOME=$INSTALL_DIR
Restart=on-failure
RestartSec=5s
# 允许管理网络/防火墙所需的 capability（不提升到完全特权）
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
LimitNOFILE=65535
# 日志走平台自身文件日志（$INSTALL_DIR/logs），systemd 只保留少量 stderr
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNITEOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true

if [ "$DO_START" -eq 1 ]; then
  log "启动服务 $SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
  sleep 3
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "服务已启动：http://<设备IP>:$PORT"
  else
    warn "服务未能启动，请查看：journalctl -u $SERVICE_NAME -n 100 --no-pager"
  fi
fi

# ------------------------------ 防火墙放行 ------------------------------
if [ "$OPEN_FIREWALL" -eq 1 ]; then
  if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
    firewall-cmd --permanent --add-port="$PORT/tcp" >/dev/null 2>&1 && firewall-cmd --reload >/dev/null 2>&1 && \
      log "已在 firewalld 放行 $PORT/tcp"
  elif command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    ufw allow "$PORT"/tcp >/dev/null 2>&1 && log "已在 ufw 放行 $PORT/tcp"
  else
    log "未检测到启用的防火墙（firewalld/ufw），跳过端口放行"
  fi
fi

cat <<SUMMARY

============================================================
 设备管理平台安装完成
------------------------------------------------------------
 安装目录 : $INSTALL_DIR
 配置文件 : $CONF
 数据库   : $INSTALL_DIR/data/device-platform.db
 日志目录 : $INSTALL_DIR/logs
 插件目录 : $INSTALL_DIR/plugins  （数据：$INSTALL_DIR/plugin-data）
 访问地址 : http://<设备IP>:$PORT
 服务管理 : systemctl {status|restart|stop} $SERVICE_NAME
 查看日志 : journalctl -u $SERVICE_NAME -f   或  tail -f $INSTALL_DIR/logs/*.log

 常用操作
   升级   : sudo bash install.sh --upgrade --binary <新二进制>
   卸载   : sudo bash install.sh --uninstall          （保留数据）
   彻底删 : sudo bash install.sh --uninstall --purge  （含数据库）
============================================================
SUMMARY
