#!/usr/bin/env python3
"""生成插件市场索引（plugin-index.json）并打印签名。

用途：给「插件源」提供内容。索引描述每个插件的下载地址、sha256、支持架构等，
平台侧会校验：
  1) 响应头 X-Index-Signature = hex(HMAC-SHA256(索引原始字节, 信任密钥))
  2) 插件包 sha256 与索引一致

用法：
  # 1) 先打包插件（每个插件目录产出 zip）
  bash scripts/pack-plugin.sh plugins-src/frp

  # 2) 生成索引（下载地址按实际部署地址填）
  python3 scripts/gen-plugin-index.py \
      --dir dist/plugins \
      --base-url https://plugins.example.com \
      --out dist/plugins/plugin-index.json \
      --key 'your-trust-key'

  # 3) 本地联调（可用自带的小服务器，自动带签名头）
  MARKET_KEY='your-trust-key' python3 scripts/market-index-server.py dist/plugins 18080
  #    平台里配置插件源地址：http://127.0.0.1:18080/plugin-index.json ，密钥同上
"""
import argparse
import hashlib
import hmac
import json
import re
import sys
import zipfile
from pathlib import Path


def read_manifest(zip_path: Path) -> dict | None:
    """从插件包中读取 manifest.json（包内根目录或单层目录下）。"""
    try:
        with zipfile.ZipFile(zip_path) as z:
            names = [n for n in z.namelist() if n.endswith("manifest.json")]
            if not names:
                return None
            # 优先根目录
            names.sort(key=len)
            with z.open(names[0]) as f:
                return json.loads(f.read().decode("utf-8-sig"))
    except Exception as exc:  # noqa: BLE001
        print(f"  跳过 {zip_path.name}：{exc}", file=sys.stderr)
        return None


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main() -> int:
    ap = argparse.ArgumentParser(description="生成插件市场索引 JSON")
    ap.add_argument("--dir", default="dist/plugins", help="插件包目录（*.zip）")
    ap.add_argument("--base-url", default="", help="插件包对外下载前缀，如 https://plugins.example.com")
    ap.add_argument("--out", default="dist/plugins/plugin-index.json", help="索引输出路径")
    ap.add_argument("--key", default="", help="可选：打印 HMAC 签名（供响应头使用）")
    args = ap.parse_args()

    src = Path(args.dir)
    if not src.is_dir():
        print(f"目录不存在：{src}", file=sys.stderr)
        return 1

    plugins = []
    for zip_path in sorted(src.glob("*.zip")):
        man = read_manifest(zip_path)
        if not man:
            continue
        pid = man.get("id") or zip_path.stem
        version = man.get("version") or "0.0.0"
        download = f"{args.base_url.rstrip('/')}/{zip_path.name}" if args.base_url else zip_path.name
        plugins.append(
            {
                "id": pid,
                "name": man.get("name") or pid,
                "version": version,
                "vendor": man.get("vendor", ""),
                "category": man.get("category", ""),
                "description": man.get("description", ""),
                "icon": man.get("icon", ""),
                "runtime": man.get("runtime", "process"),
                "arch": man.get("arch", []),
                "homepage": man.get("homepage", ""),
                "downloadUrl": download,
                "sha256": sha256_file(zip_path),
                "sizeBytes": zip_path.stat().st_size,
            }
        )
        print(f"  + {pid} {version} ({zip_path.name})")

    index = {
        "version": "1",
        "updatedAt": __import__("datetime").datetime.now().astimezone().isoformat(timespec="seconds"),
        "plugins": plugins,
    }
    body = json.dumps(index, ensure_ascii=False, indent=2).encode("utf-8")
    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_bytes(body)
    print(f"索引已生成：{out}（{len(plugins)} 个插件）")

    if args.key:
        sig = hmac.new(args.key.encode(), body, hashlib.sha256).hexdigest()
        print(f"\n响应头（由服务器返回）：\n  X-Index-Signature: {sig}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
