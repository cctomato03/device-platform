#!/usr/bin/env bash
# 单可执行文件构建脚本（Linux / macOS）
# 用法：bash scripts/build.sh [--skip-web] [out-dir]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_WEB=0
OUT_DIR="bin"
for arg in "$@"; do
  case "$arg" in
    --skip-web) SKIP_WEB=1 ;;
    *) OUT_DIR="$arg" ;;
  esac
done

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}"

if [ "$SKIP_WEB" -eq 0 ]; then
  echo "==> 构建前端"
  ( cd web && { [ -d node_modules ] || pnpm install; } && pnpm build )
else
  echo "==> 跳过前端构建（--skip-web）"
fi

# 始终重新同步 embed 目录，避免把上一次的旧产物打进二进制
[ -f web/dist/index.html ] || { echo "web/dist/index.html 不存在：请先构建前端"; exit 1; }
echo "==> 同步前端产物到 embed 目录"
rm -rf internal/webui/dist
mkdir -p internal/webui/dist
cp -r web/dist/. internal/webui/dist/

echo "==> 编译后端（内嵌前端，-tags embed）"
mkdir -p "$OUT_DIR"
CGO_ENABLED=0 go build -tags embed -trimpath -ldflags "$LDFLAGS" -o "$OUT_DIR/web-basic-server" ./cmd/server

echo "==> 完成：$OUT_DIR/web-basic-server  version=$VERSION commit=$COMMIT"
