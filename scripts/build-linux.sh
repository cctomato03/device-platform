#!/usr/bin/env bash
# 设备端交叉编译脚本：一次产出 linux/amd64 与 linux/arm64 两个可直接交付的二进制。
#
# 用法：
#   bash scripts/build-linux.sh                     # 构建前端 + 双架构二进制 + 插件包
#   bash scripts/build-linux.sh --skip-web          # 复用已有 web/dist，只重编后端
#   bash scripts/build-linux.sh --arch amd64        # 只编某一架构（amd64 / arm64 / arm）
#   bash scripts/build-linux.sh --skip-typecheck    # 跳过前端类型检查（本地快速迭代）
#
# 产物（dist/ 目录）：
#   device-platform-linux-amd64
#   device-platform-linux-arm64
#   plugins/<id>-<version>.zip
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_WEB=0
SKIP_TYPECHECK=0
ARCHES="amd64 arm64"

usage() { sed -n '2,16p' "$0"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --skip-web) SKIP_WEB=1; shift ;;
    --skip-typecheck) SKIP_TYPECHECK=1; shift ;;
    --arch)
      [ $# -ge 2 ] || { echo "错误：--arch 需要一个取值（amd64/arm64/arm）"; exit 1; }
      ARCHES="$2"; shift 2 ;;
    --arch=*) ARCHES="${1#*=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$1（-h 查看用法）"; exit 1 ;;
  esac
done

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}"

DIST="$ROOT/dist"
mkdir -p "$DIST/plugins"

if [ "$SKIP_WEB" -eq 0 ]; then
  echo "==> [1/4] 构建前端（vite build → 类型检查）"
  (
    cd web
    if [ ! -d node_modules ]; then
      # 有 lockfile 就用锁定版本安装，保证 CI 与本地一致
      if [ -f pnpm-lock.yaml ]; then pnpm install --frozen-lockfile; else pnpm install; fi
    fi
    # 注意：auto-imports.d.ts / components.d.ts 是 vite 插件生成的（默认不入库），
    # 所以必须先跑一次 vite build，才能让随后的 vue-tsc 类型检查通过。
    pnpm exec vite build
    if [ "$SKIP_TYPECHECK" -eq 0 ]; then
      pnpm exec vue-tsc --noEmit
    else
      echo "    已跳过前端类型检查（--skip-typecheck）"
    fi
  )
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
  [ -d "$dir" ] || continue
  [ -f "$dir/manifest.json" ] || continue
  bash scripts/pack-plugin.sh "$dir" || true
done

echo "==> 完成：$DIST  (version=$VERSION commit=$COMMIT)"
ls -1 "$DIST"
