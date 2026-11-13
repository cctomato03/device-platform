# FRP 内网穿透插件（frp）

把设备上的本地端口映射到公网 **frps** 服务器，用 frp 官方 `frpc` 客户端实现。

## 目录结构

```
frp/
├─ manifest.json              # 插件清单（平台识别入口，规范见 docs/插件开发规范.md）
├─ templates/frpc.toml.tmpl   # 配置模板，平台按界面配置渲染为 frpc.toml
└─ bin/frpc(.exe)             # 客户端二进制（需自行放入，见下）
```

## 准备二进制（必须）

插件包**不内置** frpc 二进制（体积大、且与架构相关），安装前把对应架构的 `frpc`
放到 `bin/` 下：

```bash
# x86_64 设备
curl -L https://github.com/fatedier/frp/releases/download/v0.61.1/frp_0.61.1_linux_amd64.tar.gz \
  | tar -xz --strip-components=1 -C plugins-src/frp/bin frp_0.61.1_linux_amd64/frpc

# arm64 设备
curl -L https://github.com/fatedier/frp/releases/download/v0.61.1/frp_0.61.1_linux_arm64.tar.gz \
  | tar -xz --strip-components=1 -C plugins-src/frp/bin frp_0.61.1_linux_arm64/frpc
```

也可以直接把平台装的 `frpc` 复制进来。打包时用：

```bash
bash scripts/pack-plugin.sh plugins-src/frp      # 生成 dist/plugins/frp-1.0.0.zip
```

## 平台侧行为

1. 安装：解压到 `<home>/plugins/frp/1.0.0/`，数据目录 `<home>/plugin-data/frp/`。
2. 保存配置：按 `manifest.json` 的 `schema` 生成表单，渲染 `conf/frpc.toml`（权限 0600）。
3. 启动：执行 `bin/frpc -c <dataDir>/conf/frpc.toml`，stdout/stderr 落到
   `<home>/plugin-data/frp/logs/plugin.log`，可在「插件市场 → 日志」实时查看。
4. 守护：`restartPolicy=always`，进程异常退出平台会自动拉起。
5. 停止/卸载：停止会向进程组发 SIGTERM；卸载可勾选是否删除数据目录。

## 服务器侧（frps）最小配置

```toml
# /etc/frp/frps.toml
bindPort = 7000
auth.method = "token"
auth.token = "请改成强随机串"
```

`serverAddr` 填公网 IP/域名，`serverPort` 填 7000，`authToken` 与上面一致即可。

## 常见问题

| 现象 | 原因与处理 |
| --- | --- |
| 启动即退出，日志 `connect to server error` | 服务器地址/端口不通，或 frps 未启动；先在「排查工具」里 ping/telnet 验证 |
| 日志 `authorization failed` | `authToken` 与 frps 不一致 |
| 端口被占用 | 服务器端 `remotePort` 已被其他代理使用，换一个端口 |
| arm 设备 exec format error | 放错架构的 frpc 二进制，按设备 `uname -m` 重新下载 |
