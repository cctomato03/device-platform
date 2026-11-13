#!/usr/bin/env bash
# 打包插件目录为可上传的插件包（zip）。
#
# 用法：bash scripts/pack-plugin.sh plugins-src/frp [输出目录，默认 dist/plugins]
#
# 约定：插件目录必须包含 manifest.json；包内保持目录结构（manifest.json 在根）。
# 提示：process 类型插件需要把对应架构的可执行文件放到 bin/ 下再打包。
set -euo pipefail

SRC="${1:-}"
OUT_DIR="${2:-dist/plugins}"
if [ -z "$SRC" ]; then
  echo "用法：bash scripts/pack-plugin.sh <插件目录> [输出目录]"
  exit 1
fi
[ -f "$SRC/manifest.json" ] || { echo "错误：$SRC 下没有 manifest.json"; exit 1; }

ID="$(sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$SRC/manifest.json" | head -n 1)"
VER="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$SRC/manifest.json" | head -n 1)"
ID="${ID:-plugin}"
VER="${VER:-0.0.0}"

mkdir -p "$OUT_DIR"
TARGET="$OUT_DIR/${ID}-${VER}.zip"
rm -f "$TARGET"

# 排除开发期噪音文件
( cd "$SRC" && zip -qr "$TARGET" . -x "*.DS_Store" -x "__MACOSX/*" -x "*.git*" ) 2>/dev/null || {
  # 没有 zip 命令时用 python 兜底
  python3 - "$SRC" "$TARGET" <<'PY'
import os, sys, zipfile
src, target = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED) as z:
    for root, _, files in os.walk(src):
        for f in files:
            if f.startswith("."):
                continue
            full = os.path.join(root, f)
            z.write(full, os.path.relpath(full, src))
PY
}

echo "已打包：$TARGET"
if [ -d "$SRC/bin" ] && [ -z "$(ls -A "$SRC/bin" 2>/dev/null)" ]; then
  echo "⚠ 提示：$SRC/bin 为空，process 类型插件启动会提示「可执行文件不存在」"
fi
