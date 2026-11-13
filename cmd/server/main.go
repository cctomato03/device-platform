// Command server 设备管理平台服务端（Go + Gin + GORM(SQLite) + Vue3 单二进制）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // 内嵌时区数据，设备无 tzdata 也能正常处理时区

	"device-platform/internal/bootstrap"
	"device-platform/internal/middleware"
)

var (
	// Version / Commit / BuildTime 由 -ldflags 注入
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("c", "", "配置文件路径（默认 <home>/conf/config.yaml）")
	home := flag.String("d", "", "数据根目录（数据库/日志/插件/上传文件统一存放）")
	showVersion := flag.Bool("v", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("device-platform %s (commit %s, built at %s)\n", Version, Commit, BuildTime)
		return
	}

	middleware.SetBuildInfo(Version, Commit, BuildTime)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx, *configPath, *home); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}
