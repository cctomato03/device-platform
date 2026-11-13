# 设备管理平台 · 容器镜像
#
# 说明：平台自身零外部依赖（SQLite + 进程内缓存），镜像里只有二进制与配置目录；
# 容器里管理网络/防火墙/插件需要 --privileged 或额外的 capability（见 docker-compose.yml）。
#
# 构建：docker build -f deploy/Dockerfile -t device-platform:latest .
# 运行：docker run -d --name device-platform -p 8090:8090 \
#         -v /opt/device-platform:/data --privileged device-platform:latest

# ---------- 构建阶段 ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src

# 先拷依赖清单，利用层缓存
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

COPY . .
# 前端产物已在构建前通过 `cd web && pnpm build` 生成（见 Makefile / scripts/build-linux.sh）
RUN test -f web/dist/index.html || (echo "缺少 web/dist：请先执行 cd web && pnpm build" && exit 1)
RUN rm -rf internal/webui/dist && mkdir -p internal/webui/dist && cp -r web/dist/. internal/webui/dist/

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -tags embed -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/device-platform ./cmd/server

# ---------- 运行阶段 ----------
FROM alpine:3.20

# 设备管理常用工具：网络排查（ping/traceroute/tcpdump/ss）、防火墙与限速（iptables/tc）
RUN apk add --no-cache \
      ca-certificates tzdata bash \
      iproute2 iputils traceroute tcpdump \
      iptables nftables

WORKDIR /app
COPY --from=builder /out/device-platform /app/device-platform
COPY configs/config.yaml /app/conf/config.yaml.example

# 数据根目录（数据库/日志/插件/上传文件都在这里，挂载出来即可持久化）
ENV DEVPLAT_HOME=/data
RUN mkdir -p /data/{conf,data,logs,plugins,plugins-bundled,plugin-data,storage,backup,run,tmp}

EXPOSE 8090
VOLUME ["/data"]

ENTRYPOINT ["/app/device-platform"]
CMD ["-d", "/data"]
