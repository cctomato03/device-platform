package service

import (
	"context"
	"encoding/json"
	"io"

	"device-platform/internal/model/entity"
	"device-platform/internal/plugin"
	"device-platform/pkg/errs"
)

// PluginView 插件视图（含清单详情，前端据此渲染配置表单）。
type PluginView struct {
	entity.Plugin
	// Schema 配置字段定义
	Schema []plugin.FieldSchema `json:"schema"`
	// ConfigMap 已解析的配置对象
	ConfigMap map[string]any `json:"configMap"`
	// Runtime 运行态（PID/启动时间）
	Runtime *plugin.StatusInfo `json:"runtime,omitempty"`
	// ManifestDetail 清单详情（分类/端口/权限提示）
	ManifestDetail *plugin.Manifest `json:"manifestDetail,omitempty"`
}

// ListPlugins 插件列表。
func (s *Service) ListPlugins(ctx context.Context) ([]PluginView, error) {
	if s.Plugins == nil {
		return nil, errs.ErrInternal.WithErr(errs.Invalid("插件管理器未初始化"))
	}
	rows, err := s.Plugins.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PluginView, 0, len(rows))
	for i := range rows {
		out = append(out, s.toView(ctx, &rows[i], false))
	}
	return out, nil
}

// GetPlugin 插件详情。
func (s *Service) GetPlugin(ctx context.Context, key string) (*PluginView, error) {
	if s.Plugins == nil {
		return nil, errs.Invalid("插件管理器未初始化")
	}
	p, err := s.Plugins.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	v := s.toView(ctx, p, true)
	return &v, nil
}

func (s *Service) toView(ctx context.Context, p *entity.Plugin, withManifest bool) PluginView {
	v := PluginView{Plugin: *p}
	if man, err := plugin.ParseManifest([]byte(p.Manifest)); err == nil {
		v.Schema = man.Schema
		if withManifest {
			v.ManifestDetail = man
		}
	}
	v.ConfigMap = map[string]any{}
	if p.Config != "" {
		_ = json.Unmarshal([]byte(p.Config), &v.ConfigMap)
	}
	if st, err := s.Plugins.Status(ctx, p.Key); err == nil {
		v.Runtime = st
	}
	return v
}

// InstallPlugin 从上传包安装插件。
func (s *Service) InstallPlugin(ctx context.Context, filename string, r io.Reader) (*PluginView, error) {
	if s.Plugins == nil {
		return nil, errs.Invalid("插件管理器未初始化")
	}
	p, err := s.Plugins.InstallArchive(ctx, filename, r)
	if err != nil {
		return nil, err
	}
	v := s.toView(ctx, p, true)
	return &v, nil
}

// UninstallPlugin 卸载插件。
func (s *Service) UninstallPlugin(ctx context.Context, key string, purgeData bool) error {
	if s.Plugins == nil {
		return errs.Invalid("插件管理器未初始化")
	}
	return s.Plugins.Uninstall(ctx, key, purgeData)
}

// PluginAction 执行 start/stop/restart/enable/disable/autostart 动作。
func (s *Service) PluginAction(ctx context.Context, key, action string, value bool) error {
	if s.Plugins == nil {
		return errs.Invalid("插件管理器未初始化")
	}
	switch action {
	case "start":
		return s.Plugins.Start(ctx, key)
	case "stop":
		return s.Plugins.Stop(ctx, key)
	case "restart":
		return s.Plugins.Restart(ctx, key)
	case "enable":
		return s.Plugins.SetEnabled(ctx, key, true)
	case "disable":
		return s.Plugins.SetEnabled(ctx, key, false)
	case "autostart":
		return s.Plugins.SetAutoStart(ctx, key, value)
	}
	return errs.Invalid("不支持的动作：" + action)
}

// UpdatePluginConfig 保存插件配置。
func (s *Service) UpdatePluginConfig(ctx context.Context, key string, cfg map[string]any, restart bool) error {
	if s.Plugins == nil {
		return errs.Invalid("插件管理器未初始化")
	}
	return s.Plugins.UpdateConfig(ctx, key, cfg, restart)
}

// PluginLogs 读取插件日志。
func (s *Service) PluginLogs(ctx context.Context, key string, lines int) (string, error) {
	if s.Plugins == nil {
		return "", errs.Invalid("插件管理器未初始化")
	}
	return s.Plugins.Logs(ctx, key, lines)
}

// PluginStatus 插件运行态。
func (s *Service) PluginStatus(ctx context.Context, key string) (*plugin.StatusInfo, error) {
	if s.Plugins == nil {
		return nil, errs.Invalid("插件管理器未初始化")
	}
	return s.Plugins.Status(ctx, key)
}

// PluginRuntimeMeta 插件运行时能力（前端据置灰 docker 相关入口）。
type PluginRuntimeMeta struct {
	DockerAvailable bool   `json:"dockerAvailable"`
	DockerAllowed   bool   `json:"dockerAllowed"`
	Arch            string `json:"arch"`
	PluginDir       string `json:"pluginDir"`
	PluginDataDir   string `json:"pluginDataDir"`
}

// GetPluginRuntimeMeta 返回插件运行时能力。
func (s *Service) GetPluginRuntimeMeta(ctx context.Context) PluginRuntimeMeta {
	meta := PluginRuntimeMeta{
		DockerAllowed: s.Cfg.Plugin.AllowDocker,
		Arch:          goArch(),
		PluginDir:     s.Cfg.Plugin.Dir,
		PluginDataDir: s.Cfg.Plugin.DataDir,
	}
	if s.Plugins != nil {
		meta.DockerAvailable = s.Plugins.DockerAvailable(ctx)
	}
	return meta
}
