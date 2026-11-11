# Web Basic Admin

一个开箱即用、可直接二次开发的后台管理脚手架：**Go + Gin + GORM** 后端，**Vue 3 + Web Admin Pro** 前端，
内嵌迁移、单可执行文件部署。

> 📘 二次开发看 [docs/开发手册.md](./docs/开发手册.md) ｜ 🖱 功能使用看 [docs/操作手册.md](./docs/操作手册.md)

## 特性

- **权限体系**：用户 / 角色 / 菜单（目录·页面·按钮·接口）/ 部门 / 岗位 / 数据权限 / 多租户
- **工作台**：4 张统计卡片 + 「我的待办 / 我的消息 / 通知公告」页签，顶栏铃铛显示未读
- **系统设置**：站点个性化（名称/logo/登录页文案/注册开关/验证码/滑块）、字典、参数、租户、文件、定时任务、代码生成
- **通知与协作**：通知公告（发布扇出站内信）、站内信、待办；业务系统可通过开放接口推送（`X-API-KEY` + 幂等键）
- **安全**：双 token 静默刷新、图形验证码、滑块校验、登录失败锁定、强口令策略、字段级操作日志
- **运维**：健康检查、Prometheus 指标、结构化日志、内嵌数据库迁移、Docker Compose

## 快速开始

```bash
# 基础设施（MySQL 3307 / Redis 6380）
docker compose -f deploy/docker-compose.yml up -d mysql redis

# 后端（:8090，首次启动自动建表 + 初始化 + 建超管）
APP__APP__ADMIN_PASSWORD=Admin@123456 go run ./cmd/server -c configs/config.yaml

# 前端（:5173）
cd web && pnpm install && pnpm dev
```

默认账号 `admin`，密码取 `APP__APP__ADMIN_PASSWORD`。

生产部署（单二进制或 Docker）：

```bash
cd web && pnpm build && cd ..
powershell -ExecutionPolicy Bypass -File scripts/build.ps1   # 或 make build
docker compose -f deploy/docker-compose.yml up -d --build     # 前后端一体镜像 + MySQL + Redis
```

## 测试

```bash
node scripts/e2e-api.mjs   http://127.0.0.1:8090 admin  # 接口 76 条断言
node scripts/e2e-smoke.mjs http://127.0.0.1:8090 admin  # 浏览器 61 条断言
```

环境变量：`E2E_ADMIN_PASSWORD`（管理员密码）、`E2E_REDIS_CONTAINER`（读验证码用，默认自动探测）。

## 目录速览

```
cmd/server      入口            internal/{api,service,repository,model,router,middleware,migrate}
migrations      数据库迁移       pkg/{crypto,jwt,errs,response,captcha,id,storage}
scripts         构建 / 图标子集 / E2E          web/  前端（Vue3 + Web Admin Pro）
configs         默认配置（环境变量 APP__XXX__YYY 覆盖）      deploy/  Dockerfile + compose
```
