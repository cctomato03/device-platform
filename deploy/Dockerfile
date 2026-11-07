# syntax=docker/dockerfile:1

##############################
# 单可执行文件镜像（前端通过 go:embed 打包进二进制）
# 构建：docker build -f deploy/Dockerfile -t web-basic:latest .
#
# 国内 / 内网环境可替换基础镜像源与包源（都可覆盖）：
#   docker build -f deploy/Dockerfile \
#     --build-arg NODE_IMAGE=<镜像仓库>/node:22-alpine \
#     --build-arg GO_IMAGE=<镜像仓库>/golang:1.26-alpine \
#     --build-arg RUNTIME_IMAGE=<镜像仓库>/alpine:3.20 \
#     --build-arg NPM_REGISTRY=https://registry.npmmirror.com \
#     --build-arg GOPROXY=https://goproxy.cn,direct \
#     -t web-basic:latest .
##############################

# 全局 ARG：只有在第一个 FROM 之前声明，才能在 FROM 中引用
ARG NODE_IMAGE=node:22-alpine
ARG GO_VERSION=1.26
ARG GO_IMAGE=golang:${GO_VERSION}-alpine
ARG RUNTIME_IMAGE=alpine:3.20

# ---------- 1. 前端构建 ----------
FROM ${NODE_IMAGE} AS web
ARG NPM_REGISTRY=https://registry.npmjs.org
WORKDIR /src/web
# 依赖清单先拷贝，利用 layer 缓存（pnpm-workspace.yaml 里声明了允许执行安装脚本的依赖）
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN corepack enable && pnpm config set registry ${NPM_REGISTRY} && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---------- 2. 后端构建 ----------
FROM ${GO_IMAGE} AS backend
ARG GOPROXY=https://goproxy.cn,direct
# 版本信息注入（/version 与 -v 均可回读）；不传则保持 dev
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0 GOOS=linux
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 前端产物放到 embed 目录（webui 包的 //go:embed all:dist 依赖此路径）
COPY --from=web /src/web/dist ./internal/webui/dist
RUN go build -tags embed -trimpath \
      -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
      -o /out/web-basic ./cmd/server

# ---------- 3. 运行镜像 ----------
FROM ${RUNTIME_IMAGE}
RUN apk add --no-cache tzdata ca-certificates && \
    addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=backend /out/web-basic /app/web-basic
# 只需配置文件：建表与初始化数据全部由 migrations/*.up.sql 编译进二进制（go:embed），
# 启动时自动执行，镜像里不需要额外的 SQL 文件。
COPY configs /app/configs
# storage 为本地存储模式的落盘目录（对应 configs/config.yaml 的 storage.local.base_dir），logs 为日志目录
RUN mkdir -p /app/logs /app/storage && chown -R app:app /app
USER app
ENV TZ=Asia/Shanghai \
    APP__APP__ENV=prod \
    APP__APP__MODE=release \
    APP__APP__PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/web-basic"]
CMD ["-c", "/app/configs/config.yaml"]
