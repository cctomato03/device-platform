// Package logger 结构化日志（zap + lumberjack 切割），同时输出到控制台与文件。
package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"device-platform/internal/config"
)

var global *zap.Logger

// Init 初始化全局 logger。
func Init(cfg config.Log) error {
	if cfg.Dir != "" {
		if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
			return err
		}
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var cores []zapcore.Core
	if cfg.Dir != "" {
		writer := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Dir, "app.log"),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   true,
		}
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(writer),
			levelOf(cfg.Level),
		))
	}
	if cfg.Console {
		consoleCfg := encoderCfg
		consoleCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleCfg),
			zapcore.AddSync(os.Stdout),
			levelOf(cfg.Level),
		))
	}
	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(os.Stdout), zapcore.InfoLevel))
	}

	global = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	zap.ReplaceGlobals(global)
	return nil
}

func levelOf(s string) zapcore.Level {
	switch s {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// L 返回全局 logger。
func L() *zap.Logger {
	if global == nil {
		global, _ = zap.NewProduction()
	}
	return global
}

// S 返回 SugaredLogger。
func S() *zap.SugaredLogger { return L().Sugar() }

// Sync 刷盘（进程退出前调用）。
func Sync() {
	if global != nil {
		_ = global.Sync()
	}
}
