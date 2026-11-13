package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"device-platform/internal/model/entity"
)

// dockerBin docker 可执行文件路径（可在测试或特殊环境覆盖）。
var dockerBin = "docker"

var (
	dockerOnce sync.Once
	dockerOK   bool
)

// DockerAvailable 检测 docker 是否可用（首次探测后缓存，避免每次拉插件都起进程）。
func (m *Manager) DockerAvailable(ctx context.Context) bool {
	if !m.cfg.Plugin.AllowDocker {
		return false
	}
	dockerOnce.Do(func() {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, dockerBin, "version", "--format", "{{.Server.Version}}")
		dockerOK = cmd.Run() == nil
	})
	return dockerOK
}

// containerName 平台给插件容器起的统一名字，便于排查。
func containerName(key string) string { return "devplat-" + key }

// dockerStart 启动容器（已存在则 start）。
func (m *Manager) dockerStart(ctx context.Context, p *entity.Plugin, man *Manifest, cfg map[string]any) error {
	spec := man.Docker
	if spec == nil {
		return fmt.Errorf("清单缺少 docker 配置")
	}
	name := containerName(p.Key)
	if exists, running := m.containerState(ctx, name); exists {
		if running {
			return nil
		}
		return m.runDocker(ctx, "start", name)
	}

	data := map[string]any{"platform": map[string]string{
		"dataDir":    p.DataPath,
		"installDir": p.InstallPath,
		"home":       m.lay.Home,
		"confDir":    p.DataPath + "/conf",
		"logDir":     p.DataPath + "/logs",
	}}
	for k, v := range cfg {
		data[k] = v
	}

	args := []string{"run", "-d", "--name", name}
	restart := spec.Restart
	if restart == "" {
		restart = "unless-stopped"
	}
	args = append(args, "--restart", restart)
	if spec.Privileged || man.Privileged {
		args = append(args, "--privileged")
	}
	if spec.Network != "" {
		args = append(args, "--network", spec.Network)
	}
	for _, port := range spec.Ports {
		rendered, _ := renderString(port, data)
		args = append(args, "-p", rendered)
	}
	for _, vol := range spec.Volumes {
		rendered, _ := renderString(vol, data)
		args = append(args, "-v", rendered)
	}
	for k, v := range spec.Env {
		rendered, _ := renderString(v, data)
		args = append(args, "-e", k+"="+rendered)
	}
	args = append(args, spec.Image)
	for _, c := range spec.Command {
		rendered, _ := renderString(c, data)
		args = append(args, rendered)
	}
	if len(man.Args) > 0 {
		args = append(args, m.renderArgs(p, man, cfg)...)
	}
	out, err := m.runDockerOut(ctx, args...)
	if err != nil {
		return fmt.Errorf("docker run 失败: %v (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

// dockerStop 停止容器。
func (m *Manager) dockerStop(ctx context.Context, key string) error {
	name := containerName(key)
	if exists, running := m.containerState(ctx, name); !exists || !running {
		return nil
	}
	_, err := m.runDockerOut(ctx, "stop", "-t", "10", name)
	return err
}

// dockerRemove 删除容器。
func (m *Manager) dockerRemove(ctx context.Context, key string) {
	_, _ = m.runDockerOut(ctx, "rm", "-f", containerName(key))
}

// dockerLogs 读取容器日志。
func (m *Manager) dockerLogs(ctx context.Context, key string, lines int) (string, error) {
	out, err := m.runDockerOut(ctx, "logs", "--tail", fmt.Sprint(lines), containerName(key))
	if err != nil {
		return out, nil // docker 把容器 stderr 也写到 stderr，这里仍返回内容
	}
	return out, nil
}

// containerState 查询容器是否存在与是否运行。
func (m *Manager) containerState(ctx context.Context, name string) (exists, running bool) {
	out, err := m.runDockerOut(ctx, "inspect", "-f", "{{.State.Running}}", name)
	if err != nil {
		return false, false
	}
	return true, strings.TrimSpace(out) == "true"
}

func (m *Manager) runDocker(ctx context.Context, args ...string) error {
	_, err := m.runDockerOut(ctx, args...)
	return err
}

// runDockerOut 执行 docker 命令并返回输出（stderr 合并）。
func (m *Manager) runDockerOut(ctx context.Context, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, dockerBin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// jsonUnmarshal 简化 JSON 反序列化（统一处理错误）。
func jsonUnmarshal(s string, out any) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), out)
}
