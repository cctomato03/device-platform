// Command server 通用后台管理系统脚手架（Go + Gin + GORM + Vue3）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // 内嵌时区数据，容器内无需额外安装 tzdata

	"web-basic/internal/bootstrap"
	"web-basic/internal/middleware"
)

var (
	// Version / Commit / BuildTime 由 -ldflags 注入
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("c", "configs/config.yaml", "配置文件路径")
	showVersion := flag.Bool("v", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("web-basic %s (commit %s, built at %s)\n", Version, Commit, BuildTime)
		return
	}

	// 暴露编译期版本信息（/version 接口与控制台日志）
	middleware.SetBuildInfo(Version, Commit, BuildTime)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}
