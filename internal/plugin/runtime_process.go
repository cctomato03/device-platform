package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/pkg/errs"
)

// maxPluginLogBytes 插件日志单文件上限，超过后滚动为 plugin.log.1（只保留一份，设备省空间）。
const maxPluginLogBytes = 20 << 20

// GOARCH 当前运行架构（插件模板变量与展示用）。
func GOARCH() string { return runtime.GOARCH }

// execCmd 原生进程运行句柄。
type execCmd struct {
	cmd     *exec.Cmd
	logFile *os.File
	stopped bool
}

// Terminate 终止进程（含子进程组）。
func (c *execCmd) Terminate() error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	c.stopped = true
	pid := c.cmd.Process.Pid
	if err := terminateProcess(pid); err != nil {
		// 进程可能已退出，这里不作为失败
		_ = c.cmd.Process.Kill()
	}
	// 最多等 5 秒确认退出
	for i := 0; i < 50; i++ {
		if c.cmd.ProcessState != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// startProcess 启动原生进程插件。
func (m *Manager) startProcess(ctx context.Context, p *entity.Plugin, man *Manifest, cfg map[string]any) error {
	m.mu.Lock()
	if _, ok := m.procs[p.Key]; ok {
		m.mu.Unlock()
		return errs.Invalid("插件已在运行中")
	}
	m.mu.Unlock()

	entry := filepath.Join(p.InstallPath, filepath.FromSlash(man.Entry))
	info, err := os.Stat(entry)
	if err != nil {
		return fmt.Errorf("插件可执行文件不存在：%s", man.Entry)
	}
	if info.IsDir() {
		return fmt.Errorf("entry 指向目录而不是可执行文件：%s", man.Entry)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(entry, info.Mode()|0o755); err != nil {
			return fmt.Errorf("设置可执行权限失败: %w", err)
		}
	}

	logPath := filepath.Join(p.DataPath, "logs", "plugin.log")
	if err := rotateLog(logPath); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	args := m.renderArgs(p, man, cfg)
	cmd := exec.Command(entry, args...)
	cmd.Dir = p.DataPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"DEVPLAT_PLUGIN_KEY="+p.Key,
		"DEVPLAT_PLUGIN_DATA="+p.DataPath,
		"DEVPLAT_HOME="+m.lay.Home,
	)
	applyProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("启动失败: %w", err)
	}

	handle := &execCmd{cmd: cmd, logFile: logFile}
	m.mu.Lock()
	m.procs[p.Key] = &procHandle{cmd: handle, pid: cmd.Process.Pid, logPath: logPath, startedAt: time.Now()}
	m.mu.Unlock()

	go m.waitProcess(p.Key, handle, man.RestartPolicy)
	return nil
}

// waitProcess 等待进程退出并处理状态与自动重启。
func (m *Manager) waitProcess(key string, h *execCmd, policy string) {
	err := h.cmd.Wait()
	_ = h.logFile.Close()

	m.mu.Lock()
	ph, ok := m.procs[key]
	manual := false
	if ok {
		manual = ph.manual
		delete(m.procs, key)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exitCode := 0
	if h.cmd.ProcessState != nil {
		exitCode = h.cmd.ProcessState.ExitCode()
	}
	if manual {
		_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
			Updates(map[string]any{"status": StatusStopped, "pid": 0}).Error
		return
	}

	msg := "进程已退出"
	if err != nil {
		msg = fmt.Sprintf("进程异常退出（exit=%d）：%v", exitCode, err)
	}

	// 自动重启策略：always 总是重启；on-failure 仅非 0 退出时重启
	shouldRestart := policy == "always" || (policy == "on-failure" && exitCode != 0)
	if shouldRestart {
		_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
			Updates(map[string]any{"status": StatusError, "pid": 0, "last_error": msg + "，3 秒后自动重启"}).Error
		time.Sleep(3 * time.Second)
		p, gerr := m.Get(ctx, key)
		if gerr == nil && p.Enabled {
			man, merr := m.manifestOf(p)
			if merr == nil {
				var cfg map[string]any
				_ = jsonUnmarshal(p.Config, &cfg)
				if serr := m.startProcess(ctx, p, man, cfg); serr == nil {
					now := time.Now()
					pid := 0
					m.mu.Lock()
					if h2, ok := m.procs[key]; ok {
						pid = h2.pid
					}
					m.mu.Unlock()
					_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
						Updates(map[string]any{"status": StatusRunning, "pid": pid, "started_at": now, "last_error": ""}).Error
					return
				}
			}
		}
	}

	_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
		Updates(map[string]any{"status": StatusError, "pid": 0, "last_error": msg}).Error
}

// rotateLog 日志超过上限时滚动（只保留一份历史，设备磁盘有限）。
func rotateLog(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < maxPluginLogBytes {
		return nil
	}
	_ = os.Remove(path + ".1")
	return os.Rename(path, path+".1")
}
