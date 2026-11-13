package service

import (
	"runtime"
	"time"

	"device-platform/pkg/version"
)

// 时间与平台信息的小工具，集中放在一处避免各模块重复实现。

// nowTime 当前时间（便于统一时区口径）。
func nowTime() time.Time { return time.Now() }

// minutesAgo 现在往前 n 分钟。
func minutesAgo(minutes int) time.Time { return time.Now().Add(-time.Duration(minutes) * time.Minute) }

// tickerOfSeconds 按秒创建 ticker。
func tickerOfSeconds(sec int) *time.Ticker {
	if sec <= 0 {
		sec = 10
	}
	return time.NewTicker(time.Duration(sec) * time.Second)
}

// BuildVersion 平台版本号（由 -ldflags 注入）。
func BuildVersion() string { return version.Version }

func goVersion() string { return runtime.Version() }
func goOS() string      { return runtime.GOOS }
func goArch() string    { return runtime.GOARCH }
