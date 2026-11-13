#!/usr/bin/env bash
# 设备端交叉编译脚本：一次产出 linux/amd64 与 linux/arm64 两个可直接交付的二进制。
#
# 用法：
#   bash scripts/build-linux.sh                 # 构建前端 + 双架构二进制 + 插件包
#   bash scripts/build-linux.sh --skip-web      # 复用已有 web/dist，只重编后端
#   bash scripts/build-linux.sh --arch arm64    # 只编某一架构（amd64 / arm64 / arm）
#
# 产物（dist/ 目录）：
#   device-platform-linux-amd64
#   device-platform-linux-arm64
#   plugins/frp-<version>.zip
#   install.sh / device-platform.service / config.yaml（安装包内容）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_WEB=0
ARCHES="amd64 arm64"
for arg in "$@"; do
  case "$arg" in
    --skip-web) SKIP_WEB=1 ;;
    --arch) shift ;;
    amd64|arm64|arm) ARCHES="$arg" ;;
  esac
done

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}"

DIST="$ROOT/dist"
mkdir -p "$DIST/plugins"

if [ "$SKIP_WEB" -eq 0 ]; then
  echo "==> [1/4] 构建前端"
  ( cd web && { [ -d node_modules ] || pnpm install; } && pnpm build )
else
  echo "==> [1/4] 跳过前端构建（--skip-web）"
fi

[ -f web/dist/index.html ] || { echo "web/dist/index.html 不存在，请先构建前端"; exit 1; }
echo "==> [2/4] 同步前端产物到 embed 目录"
rm -rf internal/webui/dist
mkdir -p internal/webui/dist
cp -r web/dist/. internal/webui/dist/

echo "==> [3/4] 交叉编译后端：$ARCHES"
for arch in $ARCHES; do
  out="$DIST/device-platform-linux-$arch"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
    go build -tags embed -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/server
  chmod +x "$out"
  echo "    $out ($(du -h "$out" | cut -f1))"
done

echo "==> [4/4] 打包内置插件"
for dir in plugins-src/*/; do
  [ -f "$dir/manifest.json" ] || continue
  bash scripts/pack-plugin.sh "$dir" || true
done

cp -f deploy/install.sh "$DIST/install.sh" 2>/dev/null || true
cp -f deploy/device-platform.service "$DIST/device-platform.service" 2>/dev/null || true
chmod +x "$DIST/install.sh" 2>/dev/null || true

echo "==> 完成：$DIST  (version=$VERSION commit=$COMMIT)"
ls -1 "$DIST"
