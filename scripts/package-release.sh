#!/usr/bin/env bash
# 把 dist/ 下的二进制打成「可直接拷到设备上用」的发布包。
#
# 用法：
#   bash scripts/package-release.sh [version] [dist目录]
#
# 产物（<dist>/release/）：
#   device-platform-<version>-linux-amd64.tar.gz
#   device-platform-<version>-linux-arm64.tar.gz
#   frp-<ver>.zip                    # 内置插件包（可选）
#   SHA256SUMS
#
# 每个 tar.gz 解开即是一个完整安装包：
#   device-platform-<version>-linux-<arch>/
#   ├─ bin/device-platform        （curl -fL 下载后 chmod +x 即用）
#   ├─ install.sh                 （一键安装成 systemd 服务）
#   ├─ device-platform.service    （systemd 单元模板）
#   ├─ conf/config.example.yaml   （配置样例）
#   ├─ plugins-bundled/frp/       （内置插件，装完在「插件市场→可安装」里可见）
#   ├─ docs/  README.md  VERSION
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
DIST="${2:-dist}"
REL="$DIST/release"

[ -f "$DIST/device-platform-linux-amd64" ] || [ -f "$DIST/device-platform-linux-arm64" ] || {
  echo "错误：$DIST 下没有找到 device-platform-linux-* 二进制，请先跑 scripts/build-linux.sh"
  exit 1
}

mkdir -p "$REL"
rm -f "$REL"/*.tar.gz "$REL"/SHA256SUMS

for arch in amd64 arm64; do
  bin="$DIST/device-platform-linux-$arch"
  [ -f "$bin" ] || { echo "跳过 $arch（无产物）"; continue; }

  name="device-platform-${VERSION}-linux-${arch}"
  stage="$DIST/stage/$name"
  rm -rf "$stage"
  mkdir -p "$stage"/{bin,conf,docs,plugins-bundled}

  cp -f "$bin" "$stage/bin/device-platform"
  chmod 0755 "$stage/bin/device-platform"

  [ -f deploy/install.sh ] && install -m 0755 deploy/install.sh "$stage/install.sh"
  [ -f deploy/device-platform.service ] && cp -f deploy/device-platform.service "$stage/"
  [ -f configs/config.yaml ] && cp -f configs/config.yaml "$stage/conf/config.example.yaml"
  [ -f README.md ] && cp -f README.md "$stage/"

  # 文档随包分发，现场排障不用回仓库找
  if [ -d docs ]; then
    for f in docs/*.md; do [ -f "$f" ] && cp -f "$f" "$stage/docs/"; done
  fi

  # 内置插件（插件包本身不带二进制的话，页面会提示「可执行文件不存在」，属预期）
  if [ -d plugins-src ]; then
    for d in plugins-src/*/; do
      [ -f "$d/manifest.json" ] || continue
      id="$(basename "$d")"
      mkdir -p "$stage/plugins-bundled/$id"
      cp -r "$d". "$stage/plugins-bundled/$id/"
    done
  fi

  cat > "$stage/VERSION" <<EOF
version: ${VERSION}
commit: $(git rev-parse --short HEAD 2>/dev/null || echo none)
built: $(date -u +%Y-%m-%dT%H:%M:%SZ)
arch: ${arch}
EOF

  tar -C "$DIST/stage" -czf "$REL/$name.tar.gz" "$name"
  echo "已生成：$REL/$name.tar.gz ($(du -h "$REL/$name.tar.gz" | cut -f1))"
done

# 内置插件包（方便从「上传插件包」直接安装）
for zip in "$DIST"/plugins/*.zip; do
  [ -f "$zip" ] && cp -f "$zip" "$REL/"
done

( cd "$REL" && sha256sum ./*.tar.gz ./*.zip > SHA256SUMS 2>/dev/null || sha256sum ./*.tar.gz > SHA256SUMS )
echo "已生成：$REL/SHA256SUMS"
ls -1 "$REL"
