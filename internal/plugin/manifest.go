// Package plugin 插件框架：插件包规范、安装卸载、进程/docker 运行时、配置渲染与日志。
//
// 统一约定（对第三方开发者友好）：
//
//	插件包（.zip / .tar.gz / 目录）
//	├─ manifest.json       插件清单（本文件定义的标准）
//	├─ bin/…               可执行文件（runtime=process）
//	└─ templates/…         配置文件模板（平台按用户配置渲染后写入数据目录）
//
// 平台只做四件事：解压校验 → 渲染配置 → 拉起/停止进程（或容器）→ 收集日志。
// 因此同一个插件包在 arm64 / x86_64 设备上行为一致，docker 与原生进程也能用同一套界面管理。
package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// Runtime 运行时类型。
const (
	RuntimeProcess = "process"
	RuntimeDocker  = "docker"
)

// 状态常量。
const (
	StatusInstalled = "installed" // 已安装未启动
	StatusRunning   = "running"
	StatusStopped   = "stopped"
	StatusError     = "error"
)

// Manifest 插件清单（插件包内的 manifest.json）。
type Manifest struct {
	// ID 插件唯一标识，小写字母/数字/中划线，2-32 位；同时是目录名
	ID string `json:"id"`
	// Name 展示名称
	Name string `json:"name"`
	// Version 语义化版本，如 1.0.0
	Version string `json:"version"`
	// Vendor 厂商/作者
	Vendor string `json:"vendor"`
	// Category 分类：network/storage/ai/media/device/security/tool
	Category string `json:"category"`
	// Description 简介（列表页展示）
	Description string `json:"description"`
	// Icon 图标（Iconify 名称，如 ri:shut-down-line）
	Icon string `json:"icon"`
	// Homepage 主页
	Homepage string `json:"homepage"`
	// License 许可证
	License string `json:"license"`
	// Arch 支持的架构列表，空表示全部；例 ["amd64","arm64"]
	Arch []string `json:"arch,omitempty"`
	// Runtime 运行时：process（原生进程）| docker（容器）
	Runtime string `json:"runtime"`
	// Entry 可执行文件相对路径（runtime=process 必填），如 bin/frpc
	Entry string `json:"entry,omitempty"`
	// Args 启动参数模板，支持 {{ .配置项 }} 与 {{ .platform.* }} 变量
	Args []string `json:"args,omitempty"`
	// Docker 容器运行时参数
	Docker *DockerSpec `json:"docker,omitempty"`
	// RestartPolicy 重启策略：always | on-failure | never
	RestartPolicy string `json:"restartPolicy,omitempty"`
	// HealthCheck 健康检查
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
	// ConfigFiles 需要平台渲染的配置文件
	ConfigFiles []ConfigFile `json:"configFiles,omitempty"`
	// Schema 配置表单定义（平台据此生成配置界面）
	Schema []FieldSchema `json:"schema"`
	// Ports 插件占用的端口（仅展示与文档用途）
	Ports []int `json:"ports,omitempty"`
	// Privileged 是否需要 root/特权（前端会给出风险提示）
	Privileged bool `json:"privileged,omitempty"`
}

// DockerSpec docker 运行时参数。
type DockerSpec struct {
	Image   string            `json:"image"`
	Command []string          `json:"command,omitempty"`
	Ports   []string          `json:"ports,omitempty"`   // "8080:80"
	Volumes []string          `json:"volumes,omitempty"` // "{{ .platform.dataDir }}/conf:/etc/x"
	Env     map[string]string `json:"env,omitempty"`
	Network string            `json:"network,omitempty"`
	// Privileged 容器特权模式
	Privileged bool `json:"privileged,omitempty"`
	// Restart 容器重启策略（默认 unless-stopped）
	Restart string `json:"restart,omitempty"`
}

// HealthCheck 健康检查定义。
type HealthCheck struct {
	// Type none | tcp
	Type string `json:"type"`
	// Port tcp 探测端口（支持模板变量）
	Port string `json:"port,omitempty"`
	// IntervalSec 探测间隔
	IntervalSec int `json:"intervalSec,omitempty"`
}

// ConfigFile 需要渲染的配置文件。
type ConfigFile struct {
	// Path 目标路径，支持变量：{{ .platform.dataDir }}、{{ .platform.installDir }}
	Path string `json:"path"`
	// Template 模板相对路径（插件包内），为空时按 name 生成
	Template string `json:"template"`
	// Mode 文件权限，默认 0644
	Mode string `json:"mode,omitempty"`
	// Optional 模板缺失时不报错
	Optional bool `json:"optional,omitempty"`
}

// FieldSchema 配置项定义（平台据此渲染表单）。
type FieldSchema struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        string        `json:"type"` // string | password | int | bool | select | textarea | list
	Default     any           `json:"default,omitempty"`
	Required    bool          `json:"required,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Help        string        `json:"help,omitempty"`
	Min         *float64      `json:"min,omitempty"`
	Max         *float64      `json:"max,omitempty"`
	Options     []FieldOption `json:"options,omitempty"` // select 选项
	// Fields list 类型时，列表项的子字段定义
	Fields []FieldSchema `json:"fields,omitempty"`
	// Columns list 类型时，列表项各字段列名
	Group string `json:"group,omitempty"`
}

// FieldOption 下拉选项。
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)

// ParseManifest 解析清单 JSON。
//
// 容错处理：Windows 下编辑器常写入 UTF-8 BOM，这里直接剥掉，
// 避免插件作者被「invalid character 'ï'」这种报错卡住。
func ParseManifest(b []byte) (*Manifest, error) {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest.json 解析失败: %w", err)
	}
	if err := m.normalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

// normalize 补默认值并校验。
func (m *Manifest) normalize() error {
	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		m.Name = m.ID
	}
	if m.Runtime == "" {
		m.Runtime = RuntimeProcess
	}
	if m.Runtime != RuntimeProcess && m.Runtime != RuntimeDocker {
		return fmt.Errorf("runtime 仅支持 process 或 docker（当前 %s）", m.Runtime)
	}
	if m.RestartPolicy == "" {
		m.RestartPolicy = "on-failure"
	}
	if m.Docker != nil && m.Docker.Restart == "" {
		m.Docker.Restart = "unless-stopped"
	}
	for i := range m.Schema {
		m.Schema[i].normalize()
	}
	if m.HealthCheck != nil && m.HealthCheck.IntervalSec <= 0 {
		m.HealthCheck.IntervalSec = 10
	}
	if m.HealthCheck != nil && m.HealthCheck.Type == "" {
		m.HealthCheck.Type = "none"
	}
	return m.Validate()
}

func (f *FieldSchema) normalize() {
	if f.Type == "" {
		f.Type = "string"
	}
	for i := range f.Fields {
		f.Fields[i].normalize()
	}
}

// Validate 校验清单合法性（安装前必须通过）。
func (m *Manifest) Validate() error {
	if !idRe.MatchString(m.ID) {
		return fmt.Errorf("id 非法：需为 2-32 位小写字母/数字/中划线，当前 %q", m.ID)
	}
	if m.Version == "" {
		return fmt.Errorf("version 不能为空")
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`).MatchString(m.Version) {
		return fmt.Errorf("version 需为语义化版本，如 1.0.0（当前 %q）", m.Version)
	}
	if len(m.Arch) > 0 && !m.archSupported(runtime.GOARCH) {
		return fmt.Errorf("插件不支持当前架构 %s（支持：%s）", runtime.GOARCH, strings.Join(m.Arch, ", "))
	}
	switch m.Runtime {
	case RuntimeProcess:
		if strings.TrimSpace(m.Entry) == "" {
			return fmt.Errorf("runtime=process 时 entry 不能为空（例：bin/frpc）")
		}
		if strings.Contains(m.Entry, "..") {
			return fmt.Errorf("entry 不允许包含 ..")
		}
	case RuntimeDocker:
		if m.Docker == nil || strings.TrimSpace(m.Docker.Image) == "" {
			return fmt.Errorf("runtime=docker 时必须提供 docker.image")
		}
	}
	for i, f := range m.Schema {
		if strings.TrimSpace(f.Key) == "" {
			return fmt.Errorf("schema[%d].key 不能为空", i)
		}
		switch f.Type {
		case "string", "password", "int", "bool", "select", "textarea", "list":
		default:
			return fmt.Errorf("schema[%d].type 不支持：%s", i, f.Type)
		}
	}
	for i, cf := range m.ConfigFiles {
		if strings.TrimSpace(cf.Path) == "" {
			return fmt.Errorf("configFiles[%d].path 不能为空", i)
		}
		if strings.Contains(cf.Path, "..") {
			return fmt.Errorf("configFiles[%d].path 不允许包含 ..", i)
		}
	}
	return nil
}

func (m *Manifest) archSupported(arch string) bool {
	for _, a := range m.Arch {
		if strings.EqualFold(a, arch) {
			return true
		}
	}
	return false
}

// DefaultConfig 依据 schema 生成默认配置。
func (m *Manifest) DefaultConfig() map[string]any {
	out := make(map[string]any, len(m.Schema))
	for _, f := range m.Schema {
		switch f.Type {
		case "bool":
			if v, ok := f.Default.(bool); ok {
				out[f.Key] = v
			} else {
				out[f.Key] = false
			}
		case "int":
			out[f.Key] = toNumber(f.Default)
		case "list":
			if v, ok := f.Default.([]any); ok {
				out[f.Key] = v
			} else {
				out[f.Key] = []any{}
			}
		default:
			if f.Default != nil {
				out[f.Key] = f.Default
			} else {
				out[f.Key] = ""
			}
		}
	}
	return out
}

// ValidateConfig 校验用户提交的配置（必填、数值范围）。
func (m *Manifest) ValidateConfig(cfg map[string]any) error {
	for _, f := range m.Schema {
		v, exists := cfg[f.Key]
		if f.Required {
			if !exists || isEmpty(v) {
				return fmt.Errorf("配置项「%s」不能为空", f.Label)
			}
		}
		if f.Type == "int" && exists && v != nil {
			n, ok := toFloat(v)
			if !ok {
				return fmt.Errorf("配置项「%s」需为数字", f.Label)
			}
			if f.Min != nil && n < *f.Min {
				return fmt.Errorf("配置项「%s」不能小于 %v", f.Label, *f.Min)
			}
			if f.Max != nil && n > *f.Max {
				return fmt.Errorf("配置项「%s」不能大于 %v", f.Label, *f.Max)
			}
		}
		if f.Type == "select" && exists && len(f.Options) > 0 {
			s := fmt.Sprint(v)
			ok := false
			for _, o := range f.Options {
				if o.Value == s {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("配置项「%s」取值非法：%s", f.Label, s)
			}
		}
	}
	return nil
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	}
	return false
}

func toNumber(v any) int {
	f, _ := toFloat(v)
	return int(f)
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		var f float64
		if _, err := fmt.Sscanf(t, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}
