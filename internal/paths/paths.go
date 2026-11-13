// Package paths 统一管理平台的运行时目录布局。
//
// 设计目标：设备上「一个目录装下所有东西」，整机备份/迁移只需打包 home。
//
//	<home>/
//	├─ conf/           配置文件（config.yaml）
//	├─ bin/            平台二进制（安装脚本投放）
//	├─ data/           SQLite 数据库文件（device-platform.db + wal/shm）
//	├─ logs/           平台自身日志（按天滚动）
//	├─ plugins/        已安装插件包（plugins/<plugin-id>/<version>/...）
//	├─ plugin-data/    插件实例运行数据（配置、日志、持久化数据）
//	├─ storage/        平台上传的文件
//	├─ backup/         数据库与配置备份
//	├─ run/            pid 文件、临时锁
//	└─ tmp/            解压与临时文件
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout 运行时目录集合。
type Layout struct {
	Home       string
	Conf       string
	Bin        string
	Data       string
	DBFile     string
	Logs       string
	Plugins    string
	PluginData string
	Bundled    string
	Storage    string
	Backup     string
	Run        string
	Tmp        string
}

// Resolve 依照 home 生成目录布局并确保目录存在。
func Resolve(home string) (*Layout, error) {
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, fmt.Errorf("解析数据根目录失败: %w", err)
	}
	l := &Layout{
		Home:       abs,
		Conf:       filepath.Join(abs, "conf"),
		Bin:        filepath.Join(abs, "bin"),
		Data:       filepath.Join(abs, "data"),
		DBFile:     filepath.Join(abs, "data", "device-platform.db"),
		Logs:       filepath.Join(abs, "logs"),
		Plugins:    filepath.Join(abs, "plugins"),
		PluginData: filepath.Join(abs, "plugin-data"),
		Bundled:    filepath.Join(abs, "plugins-bundled"),
		Storage:    filepath.Join(abs, "storage"),
		Backup:     filepath.Join(abs, "backup"),
		Run:        filepath.Join(abs, "run"),
		Tmp:        filepath.Join(abs, "tmp"),
	}
	for _, d := range []string{l.Conf, l.Bin, l.Data, l.Logs, l.Plugins, l.PluginData, l.Bundled, l.Storage, l.Backup, l.Run, l.Tmp} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", d, err)
		}
	}
	return l, nil
}

// ConfigFile 默认配置文件路径。
func (l *Layout) ConfigFile() string { return filepath.Join(l.Conf, "config.yaml") }

// PluginDir 插件包目录（plugins/<id>）。
func (l *Layout) PluginDir(id string) string { return filepath.Join(l.Plugins, id) }

// PluginVersionDir 插件包某个版本的目录（plugins/<id>/<version>）。
func (l *Layout) PluginVersionDir(id, version string) string {
	return filepath.Join(l.Plugins, id, version)
}

// PluginWorkDir 插件实例数据目录（plugin-data/<id>）。
func (l *Layout) PluginWorkDir(id string) string { return filepath.Join(l.PluginData, id) }
