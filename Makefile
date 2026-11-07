# Web Basic 管理平台脚手架 —— 常用任务
# 说明：Windows 用户请使用 scripts/build.ps1，本 Makefile 面向 Linux / macOS / WSL。

SHELL := /bin/bash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILT)

.PHONY: help
help:
	@echo "make web-install   安装前端依赖"
	@echo "make web-dev       前端开发服务器 (5173)"
	@echo "make server-dev    后端开发服务器 (8090)"
	@echo "make build         构建单可执行文件 bin/web-basic-server"
	@echo "make build-dist    前后端分离构建（web/dist + bin/web-basic-server）"
	@echo "make test          运行后端测试与 vet"
	@echo "make docker        构建镜像 web-basic:latest"
	@echo "make up / down     docker compose 启停（MySQL8 + Redis）"
	@echo "make up57 / down57 docker compose 启停（MySQL5.7 变体）"

web-install:
	cd web && pnpm install

web-dev:
	cd web && pnpm dev

server-dev:
	go run ./cmd/server -c configs/config.yaml

build:
	bash scripts/build.sh bin

build-dist:
	cd web && pnpm build
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/web-basic-server ./cmd/server

test:
	go vet ./...
	go test ./... -count=1

docker:
	docker build -f deploy/Dockerfile -t web-basic:latest .

up:
	docker compose -f deploy/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose.yml down

up57:
	docker compose -f deploy/docker-compose.mysql57.yml up -d --build

down57:
	docker compose -f deploy/docker-compose.mysql57.yml down
