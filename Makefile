# 设备管理平台 Makefile —— 常用任务
# Windows 用户也可用 scripts/build.ps1；本文件面向 Linux / macOS / WSL。
SHELL := /bin/bash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILT)
DATA_DIR ?= runtime

.PHONY: help
help:
	@echo "make web-install   安装前端依赖"
	@echo "make web-dev       前端开发服务器 (5173)"
	@echo "make server-dev    后端开发服务器 (8090，数据目录 $(DATA_DIR))"
	@echo "make build         构建单可执行文件（前端内嵌）bin/device-platform-server"
	@echo "make build-linux   交叉编译 linux/amd64 + linux/arm64 交付包到 dist/"
	@echo "make plugin        打包 plugins-src 下的插件包到 dist/plugins"
	@echo "make market-index  生成插件市场索引（需 MARKET_KEY/BASE_URL 变量）"
	@echo "make test          后端 vet + 单测 + 前端类型检查"
	@echo "make clean         清理构建产物与本地运行数据"

web-install:
	cd web && pnpm install

web-dev:
	cd web && pnpm dev

server-dev:
	go run ./cmd/server -d $(DATA_DIR)

build:
	bash scripts/build.sh bin

build-linux:
	bash scripts/build-linux.sh

plugin:
	@for d in plugins-src/*/; do [ -f $$d/manifest.json ] && bash scripts/pack-plugin.sh $$d; done

# 生成在线市场索引：make market-index BASE_URL=https://plugins.example.com MARKET_KEY=xxx
BASE_URL ?= http://127.0.0.1:18080
MARKET_KEY ?=
market-index: plugin
	python3 scripts/gen-plugin-index.py --dir dist/plugins --base-url $(BASE_URL) \
		--out dist/plugins/plugin-index.json $(if $(MARKET_KEY),--key $(MARKET_KEY),)

test:
	go vet ./...
	go test ./... -count=1
	cd web && npx vue-tsc --noEmit

clean:
	rm -rf bin dist internal/webui/dist web/dist
	@echo "本地运行数据（$(DATA_DIR)）请手动删除：rm -rf $(DATA_DIR)"
