# 设备管理平台（device-platform）

在 Linux 设备（Ubuntu 20.04/22.04，**x86_64 / arm64**）上运行的一体化设备管理平台：

- **单二进制**：后端 Go（Gin + GORM）+ 前端 Vue3 全部内嵌，装完只有一个可执行文件；
- **零外部依赖**：数据库是嵌入式 **SQLite**，缓存是**进程内**实现，不需要 MySQL / Redis；
- **一个目录装下所有东西**：配置、数据库、日志、插件、上传文件都在 `app.home` 内；
- **一键安装**：`install.sh` 生成配置、写入 systemd、启动服务，支持自定义目录/端口/用户；
- **插件市场**：统一的插件规范 + 生命周期管理（安装/配置/启停/日志/卸载），支持**原生进程**与 **docker** 两种运行时。

---

## 1. 能力总览

| 域 | 能力 |
| --- | --- |
| 主机管理 | 系统概览、硬件状态（CPU/内存/磁盘/温度/负载）、系统监控（指标趋势）、网络流量 |
| 网络管理 | 网络接口、网卡地址（netplan）、静态路由（`ip route`）、策略路由（`ip rule`）、网桥与端口聚合、DHCP 服务（dnsmasq）、DNS 服务（dnsmasq）、BGP/OSPF（FRR）、DNS 配置、网络状态 |
| 设备管理 | 本地设置（设备名/时区/NTP/DNS/嗅探开关）、运行告警（阈值规则+记录）、排查工具（ping/traceroute/tcpdump/ss）、网络状态、SNMP 服务、关于系统 |
| 安全策略 | 防火墙状态与 firewalld 安装、一体化访问控制策略（含**冲突检测**）、NAT（SNAT/DNAT）、带宽管理（QoS） |
| 日志审计 | 访问日志、操作日志（字段级 diff）、登录日志、系统日志、流量日志 |
| 插件市场 | 插件安装/卸载/启停/重启/禁用/自启、schema 驱动的配置界面、配置渲染、日志查看；**在线市场**（远程索引 + HMAC 签名 + sha256 校验 + 一键升级） |
| 基础平台 | 用户/角色/菜单（按钮与接口级权限）、部门/岗位/字典/参数、定时任务、文件管理、站点设置、通知公告、待办与站内信 |

内置样板插件：**FRP 内网穿透（frpc）** —— 把设备本地端口映射到公网 frps。

---

## 2. 快速开始

### 2.1 开发

```bash
# 后端（首次启动会在 runtime/ 下自动建库、建表、初始化超管）
APP__APP__ADMIN_PASSWORD=Admin@123456 APP__JWT__SECRET=please-change-this-secret-32chars go run ./cmd/server -d runtime

# 前端（:5173，代理到 :8090）
cd web && pnpm install && pnpm dev
```

默认账号 `admin`，密码取 `APP__APP__ADMIN_PASSWORD`（未设置时按配置生成，见启动日志）。

### 2.2 构建交付物

```bash
bash scripts/build-linux.sh          # 前端 + linux/amd64 + linux/arm64 + 插件包 → dist/
```

### 2.3 设备上安装

```bash
sudo bash install.sh -y --dir /opt/device-platform --port 8090 --admin-password 'Admin@123456'
# 浏览器打开 http://<设备IP>:8090
```

---

## 3. 目录结构（源码）

```
device-platform/
├─ cmd/server/               入口（-d 指定数据根目录）
├─ configs/config.yaml       默认配置（环境变量 APP__XXX__YYY 覆盖）
├─ internal/
│  ├─ api/                   HTTP 处理层
│  ├─ service/               业务逻辑（host/network/device/security/plugin/alerts…）
│  ├─ plugin/                插件框架：manifest、安装、配置渲染、进程/容器运行时
│  ├─ sysinfo/               硬件与系统指标采集（gopsutil）
│  ├─ syscmd/                受控系统命令执行（超时、白名单、错误可读化）
│  ├─ paths/                 统一数据目录布局
│  ├─ database/              SQLite 连接（WAL / busy_timeout / 时区）
│  ├─ migrate/               版本化迁移执行器（migrations/*.up.sql 内嵌）
│  ├─ model/entity/          表结构映射
│  ├─ repository/            GORM 数据访问
│  ├─ middleware/            鉴权、权限、访问日志、操作日志、指标、限流
│  ├─ router/                路由与权限码
│  └─ webui/                 内嵌前端产物
├─ migrations/               SQLite 迁移（唯一建表入口）
├─ pkg/                      通用库：cache / captcha / jwt / crypto / errs / response / storage / version
├─ plugins-src/frp/          内置插件源码（frp 样板）
├─ deploy/                   install.sh、systemd 单元、Dockerfile、compose
├─ scripts/                  构建、交叉编译、插件打包、E2E、图标子集
├─ web/                      前端（Vue3 + Element Plus + Tailwind）
└─ docs/                     设计、插件规范、部署运维、开发/操作手册
```

运行时目录（设备上，默认 `/opt/device-platform`）：

```
bin/ conf/ data/ logs/ plugins/ plugins-bundled/ plugin-data/ storage/ backup/ run/ tmp/
```

---

## 4. 文档

| 文档 | 内容 |
| --- | --- |
| [平台设计规划](docs/平台设计规划.md) | 架构分层、技术选型、功能实现状态、迭代路线、行业扩展 |
| [插件开发规范](docs/插件开发规范.md) | manifest 字段、schema 表单、模板变量、生命周期、验收清单、示例 |
| [部署运维手册](docs/部署运维手册.md) | 安装/升级/备份/迁移/反代/排障/安全加固 |
| [开发手册](docs/开发手册.md) | 脚手架分层约定、新增功能路径、前端约定、测试 |
| [操作手册](docs/操作手册.md) | 面向使用者的功能说明 |

---

## 5. 与上游脚手架（web-basic）的主要差异

| 项 | 上游 | 本平台 |
| --- | --- | --- |
| 数据库 | MySQL 8 | **SQLite**（WAL，纯 Go 驱动，可交叉编译） |
| 缓存 | Redis | **进程内缓存**（`pkg/cache`，含集合与租约） |
| 配置 | `mysql.*` / `redis.*` | `database.*` + `app.home` + `plugin.*` + `system.*` |
| 启动 | `-c configs/config.yaml` | `-d <home>`（配置缺失自动生成） |
| 部署 | Docker Compose + MySQL/Redis | **单二进制 + systemd 一键安装**（容器方式保留） |
| 新增 | — | 插件框架与市场、主机/网络/设备/安全/日志模块、访问日志、告警与指标采样 |

---

## 6. 测试

```bash
# 后端
go vet ./... && go test ./...
# 前端类型检查
cd web && npx vue-tsc --noEmit
# 接口级 E2E（需先启动服务）
node scripts/e2e-api.mjs http://127.0.0.1:8090 admin
node scripts/e2e-smoke.mjs http://127.0.0.1:8090 admin
```

---

## 7. 许可证

沿用脚手架的上游许可（见 `LICENSE` 与各插件目录内的 `license` 字段）。
