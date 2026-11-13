#!/usr/bin/env python3
"""插件市场索引的本地联调服务器（仅开发/演示用）。

它做两件事：
  1) 静态托管插件包与索引文件；
  2) 给索引响应自动加上 `X-Index-Signature` 头（HMAC-SHA256，密钥取环境变量 MARKET_KEY），
     让平台的签名校验链路能完整跑通。

用法：
  MARKET_KEY='your-trust-key' python3 scripts/market-index-server.py dist/plugins 18080
  # 平台「插件源」填：http://127.0.0.1:18080/plugin-index.json ，签名密钥填同一个 key

生产环境请不要用它：用 Nginx + 对象存储/静态托管，并在反代里加签名头（或改为签发端内置签名）。
"""
import hashlib
import hmac
import os
import sys
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

KEY = os.environ.get("MARKET_KEY", "")


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):  # noqa: N802
        # 索引文件加签名头
        if self.path.split("?")[0].endswith(".json") and KEY:
            file_path = Path(self.directory) / self.path.split("?")[0].lstrip("/")
            if file_path.is_file():
                body = file_path.read_bytes()
                sig = hmac.new(KEY.encode(), body, hashlib.sha256).hexdigest()
                self.send_response(200)
                self.send_header("Content-Type", "application/json; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.send_header("X-Index-Signature", sig)
                self.end_headers()
                self.wfile.write(body)
                return
        return super().do_GET()

    def log_message(self, fmt, *args):  # noqa: A003
        sys.stderr.write("[market] " + (fmt % args) + "\n")


def main() -> int:
    root = sys.argv[1] if len(sys.argv) > 1 else "dist/plugins"
    port = int(sys.argv[2]) if len(sys.argv) > 2 else 18080
    if not Path(root).is_dir():
        print(f"目录不存在：{root}", file=sys.stderr)
        return 1
    if not KEY:
        print("警告：未设置 MARKET_KEY，索引将不带签名（平台侧把签名密钥留空即可）", file=sys.stderr)
    handler = partial(Handler, directory=root)
    httpd = ThreadingHTTPServer(("0.0.0.0", port), handler)
    print(f"插件市场联调服务已启动：http://127.0.0.1:{port}/plugin-index.json （目录 {root}）")
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n已停止")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
