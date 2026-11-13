package bootstrap

import "runtime"

// archName 返回平台/架构标识，用于启动日志与插件兼容性判断展示。
func archName() string { return runtime.GOOS + "/" + runtime.GOARCH }
