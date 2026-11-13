package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"gorm.io/gorm"

	"device-platform/internal/config"
	"device-platform/internal/model/entity"
	"device-platform/internal/paths"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// Manager 插件管理器：单机管理所有插件的生命周期。
//
// 并发约定：所有会改变状态的公开方法都在 mu 保护下执行，
// 进程等待（cmd.Wait）在独立 goroutine 中回调 updateStatus。
type Manager struct {
	cfg *config.Config
	lay *paths.Layout
	db  *gorm.DB

	mu    sync.Mutex
	procs map[string]*procHandle
}

// procHandle 运行中的原生进程句柄。
type procHandle struct {
	cmd       *execCmd
	pid       int
	logPath   string
	startedAt time.Time
	// manual 标记是否由用户主动停止（主动停止不触发自动重启）
	manual bool
}

// NewManager 创建插件管理器。
func NewManager(cfg *config.Config, lay *paths.Layout, db *gorm.DB) *Manager {
	return &Manager{
		cfg:   cfg,
		lay:   lay,
		db:    db,
		procs: make(map[string]*procHandle, 4),
	}
}

// ---------- 查询 ----------

// List 列出全部已安装插件。
func (m *Manager) List(ctx context.Context) ([]entity.Plugin, error) {
	var rows []entity.Plugin
	if err := m.db.WithContext(ctx).Order("installed_at asc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rows, nil
}

// Get 读取单个插件。
func (m *Manager) Get(ctx context.Context, key string) (*entity.Plugin, error) {
	var p entity.Plugin
	if err := m.db.WithContext(ctx).Where("key = ?", key).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.Invalid("插件不存在：" + key)
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &p, nil
}

// StatusInfo 运行态详情（供前端轮询）。
type StatusInfo struct {
	Key       string     `json:"key"`
	Status    string     `json:"status"`
	PID       int        `json:"pid"`
	StartedAt *time.Time `json:"startedAt"`
	LastError string     `json:"lastError"`
	UptimeSec int64      `json:"uptimeSec"`
}

// Status 读取运行态。
func (m *Manager) Status(ctx context.Context, key string) (*StatusInfo, error) {
	p, err := m.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	info := &StatusInfo{Key: p.Key, Status: p.Status, PID: p.PID, StartedAt: p.StartedAt, LastError: p.LastError}
	if p.StartedAt != nil && p.Status == StatusRunning {
		info.UptimeSec = int64(time.Since(*p.StartedAt).Seconds())
	}
	// 原生进程：句柄里记录的 pid 才是权威值
	m.mu.Lock()
	h, ok := m.procs[key]
	m.mu.Unlock()
	if ok && h.pid > 0 {
		info.PID = h.pid
		info.Status = StatusRunning
	}
	return info, nil
}

// ---------- 安装 / 卸载 ----------

// InstallArchive 从上传的插件包安装（zip/tar.gz）。
func (m *Manager) InstallArchive(ctx context.Context, filename string, r io.Reader) (*entity.Plugin, error) {
	if err := os.MkdirAll(m.lay.Tmp, 0o755); err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	tmp, err := os.CreateTemp(m.lay.Tmp, "plugin-*"+archiveExt(filename))
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return nil, errs.ErrUploadFailed.WithErr(err)
	}
	tmp.Close()

	// 解压到临时目录，找出 manifest.json（允许包内多一层目录）
	stage := filepath.Join(m.lay.Tmp, "stage-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	defer os.RemoveAll(stage)
	if err := ExtractArchive(tmpPath, stage); err != nil {
		return nil, errs.InvalidErr(err)
	}
	root, err := findManifestRoot(stage)
	if err != nil {
		return nil, errs.InvalidErr(err)
	}
	return m.installFromDir(ctx, root)
}

// InstallBundled 安装平台自带插件目录（<home>/plugins/<id>/ 下已有 manifest.json 的解压目录）。
func (m *Manager) InstallBundled(ctx context.Context, dir string) (*entity.Plugin, error) {
	return m.installFromDir(ctx, dir)
}

// installFromDir 从已解压目录安装/升级。
func (m *Manager) installFromDir(ctx context.Context, srcDir string) (*entity.Plugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	raw, err := os.ReadFile(filepath.Join(srcDir, "manifest.json"))
	if err != nil {
		return nil, errs.Invalid("插件包缺少 manifest.json")
	}
	man, err := ParseManifest(raw)
	if err != nil {
		return nil, errs.InvalidErr(err)
	}
	if man.Runtime == RuntimeDocker && !m.DockerAvailable(ctx) {
		if !m.cfg.Plugin.AllowDocker {
			return nil, errs.Invalid("当前设备未启用 docker 插件能力（plugin.allow_docker=false）")
		}
		return nil, errs.Invalid("当前设备未安装 docker 或 docker 服务不可用，无法安装 docker 类型插件")
	}

	dstDir := m.lay.PluginVersionDir(man.ID, man.Version)
	if filepath.Clean(srcDir) != filepath.Clean(dstDir) {
		if err := os.RemoveAll(dstDir); err != nil {
			return nil, errs.ErrInternal.WithErr(err)
		}
		if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
			return nil, errs.ErrInternal.WithErr(err)
		}
		if err := copyTree(srcDir, dstDir); err != nil {
			return nil, errs.ErrInternal.WithErr(fmt.Errorf("复制插件文件失败: %w", err))
		}
	}
	size := dirSize(dstDir)

	dataDir := m.lay.PluginWorkDir(man.ID)
	for _, d := range []string{dataDir, filepath.Join(dataDir, "logs"), filepath.Join(dataDir, "conf")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, errs.ErrInternal.WithErr(err)
		}
	}

	// 已有安装：按升级处理，保留用户配置（新 schema 的字段补默认值）
	var existing entity.Plugin
	err = m.db.WithContext(ctx).Where("key = ?", man.ID).First(&existing).Error
	cfg := man.DefaultConfig()
	autoStart := true
	enabled := true
	if err == nil {
		var old map[string]any
		if json.Unmarshal([]byte(existing.Config), &old) == nil {
			for k := range cfg {
				if v, ok := old[k]; ok {
					cfg[k] = v
				}
			}
		}
		autoStart = existing.AutoStart
		enabled = existing.Enabled
	} else if err != gorm.ErrRecordNotFound {
		return nil, errs.ErrDB.WithErr(err)
	}

	manJSON, _ := json.Marshal(man)
	cfgJSON, _ := json.Marshal(cfg)
	row := entity.Plugin{
		Base:        entity.Base{ID: id.Next()},
		Key:         man.ID,
		Name:        man.Name,
		Version:     man.Version,
		Vendor:      man.Vendor,
		Category:    man.Category,
		Description: man.Description,
		Icon:        man.Icon,
		Runtime:     man.Runtime,
		Status:      StatusStopped,
		Enabled:     enabled,
		AutoStart:   autoStart,
		Config:      string(cfgJSON),
		Manifest:    string(manJSON),
		InstallPath: dstDir,
		DataPath:    dataDir,
		InstalledAt: time.Now(),
		SizeBytes:   size,
		TenantID:    m.cfg.Tenant.DefaultID,
	}
	if err == nil {
		row.ID = existing.ID
		row.Base = existing.Base
		row.InstalledAt = existing.InstalledAt
		err = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", man.ID).Updates(map[string]any{
			"name":         row.Name,
			"version":      row.Version,
			"vendor":       row.Vendor,
			"category":     row.Category,
			"description":  row.Description,
			"icon":         row.Icon,
			"runtime":      row.Runtime,
			"status":       StatusStopped,
			"pid":          0,
			"config":       row.Config,
			"manifest":     row.Manifest,
			"install_path": row.InstallPath,
			"data_path":    row.DataPath,
			"size_bytes":   row.SizeBytes,
			"last_error":   "",
			"update_time":  time.Now(),
		}).Error
	} else {
		err = m.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}

	// 首次安装立即渲染配置文件，便于用户先看配置再启动
	if err := m.renderConfigFiles(&row, man, cfg); err != nil {
		_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", man.ID).
			Updates(map[string]any{"last_error": err.Error()}).Error
		return nil, errs.InvalidErr(fmt.Errorf("渲染配置文件失败: %w", err))
	}
	return m.Get(ctx, man.ID)
}

// Uninstall 卸载插件；purgeData=true 时连同数据目录一起删除。
func (m *Manager) Uninstall(ctx context.Context, key string, purgeData bool) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := m.stopLocked(ctx, p, true); err != nil {
		// 停止失败不阻断卸载，但记录原因
		m.setError(ctx, key, err)
	}
	if p.Runtime == RuntimeDocker {
		m.dockerRemove(ctx, key)
	}
	if err := os.RemoveAll(m.lay.PluginDir(key)); err != nil {
		return errs.ErrInternal.WithErr(fmt.Errorf("删除插件文件失败: %w", err))
	}
	if purgeData {
		if err := os.RemoveAll(m.lay.PluginWorkDir(key)); err != nil {
			return errs.ErrInternal.WithErr(fmt.Errorf("删除插件数据失败: %w", err))
		}
	}
	if err := m.db.WithContext(ctx).Where("key = ?", key).Delete(&entity.Plugin{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ---------- 生命周期 ----------

// Start 启动插件。
func (m *Manager) Start(ctx context.Context, key string) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	if !p.Enabled {
		return errs.Invalid("插件已禁用，请先启用再启动")
	}
	man, err := m.manifestOf(p)
	if err != nil {
		return err
	}
	if p.Runtime == RuntimeDocker && !m.DockerAvailable(ctx) {
		return errs.Invalid("docker 服务不可用，无法启动容器型插件")
	}
	// 启动前重新渲染配置：保证配置文件与界面配置一致
	var cfg map[string]any
	if err := json.Unmarshal([]byte(p.Config), &cfg); err != nil {
		return errs.Invalid("插件配置损坏，请重新保存配置")
	}
	if err := m.renderConfigFiles(p, man, cfg); err != nil {
		return errs.InvalidErr(fmt.Errorf("渲染配置文件失败: %w", err))
	}

	switch p.Runtime {
	case RuntimeDocker:
		if err := m.dockerStart(ctx, p, man, cfg); err != nil {
			m.setError(ctx, key, err)
			return errs.InvalidErr(err)
		}
	default:
		if err := m.startProcess(ctx, p, man, cfg); err != nil {
			m.setError(ctx, key, err)
			return errs.InvalidErr(err)
		}
	}

	now := time.Now()
	pid := 0
	m.mu.Lock()
	if h, ok := m.procs[key]; ok {
		pid = h.pid
	}
	m.mu.Unlock()
	if err := m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).Updates(map[string]any{
		"status":     StatusRunning,
		"pid":        pid,
		"started_at": now,
		"last_error": "",
	}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// Stop 停止插件。
func (m *Manager) Stop(ctx context.Context, key string) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	return m.stopLocked(ctx, p, true)
}

// stopLocked 停止并更新状态。
func (m *Manager) stopLocked(ctx context.Context, p *entity.Plugin, manual bool) error {
	var stopErr error
	if p.Runtime == RuntimeDocker {
		stopErr = m.dockerStop(ctx, p.Key)
	} else {
		m.mu.Lock()
		h, ok := m.procs[p.Key]
		if ok {
			h.manual = manual
		}
		m.mu.Unlock()
		if ok {
			stopErr = h.cmd.Terminate()
		}
	}
	m.mu.Lock()
	delete(m.procs, p.Key)
	m.mu.Unlock()

	updates := map[string]any{"status": StatusStopped, "pid": 0, "started_at": nil}
	if stopErr != nil {
		updates["last_error"] = stopErr.Error()
	}
	if err := m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", p.Key).Updates(updates).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return stopErr
}

// Restart 重启插件。
func (m *Manager) Restart(ctx context.Context, key string) error {
	if err := m.Stop(ctx, key); err != nil {
		return err
	}
	// 给进程/容器一点退出时间
	time.Sleep(500 * time.Millisecond)
	return m.Start(ctx, key)
}

// SetEnabled 启用/禁用（禁用会先停止）。
func (m *Manager) SetEnabled(ctx context.Context, key string, enabled bool) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	if !enabled {
		if err := m.stopLocked(ctx, p, true); err != nil {
			_ = err // 禁用优先成功
		}
	}
	if err := m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
		Updates(map[string]any{"enabled": enabled, "update_time": time.Now()}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// SetAutoStart 设置是否随平台自启。
func (m *Manager) SetAutoStart(ctx context.Context, key string, auto bool) error {
	return m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
		Updates(map[string]any{"auto_start": auto, "update_time": time.Now()}).Error
}

// UpdateConfig 保存配置（可选立即重启生效）。
func (m *Manager) UpdateConfig(ctx context.Context, key string, cfg map[string]any, restart bool) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	man, err := m.manifestOf(p)
	if err != nil {
		return err
	}
	merged := man.DefaultConfig()
	for k, v := range cfg {
		merged[k] = v
	}
	if err := man.ValidateConfig(merged); err != nil {
		return errs.InvalidErr(err)
	}
	raw, _ := json.Marshal(merged)
	if err := m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).
		Updates(map[string]any{"config": string(raw), "update_time": time.Now()}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	p.Config = string(raw)
	if err := m.renderConfigFiles(p, man, merged); err != nil {
		return errs.InvalidErr(fmt.Errorf("渲染配置文件失败: %w", err))
	}
	if restart && p.Status == StatusRunning {
		return m.Restart(ctx, key)
	}
	return nil
}

// ---------- 配置渲染 ----------

// RenderVars 模板可用的变量集合。
type RenderVars struct {
	Platform map[string]string
	Config   map[string]any
}

// templateFuncs 插件模板可用的函数（只提供无副作用的字符串处理，避免模板里做危险操作）。
var templateFuncs = template.FuncMap{
	"split":  strings.Split,
	"join":   strings.Join,
	"trim":   strings.TrimSpace,
	"upper":  strings.ToUpper,
	"lower":  strings.ToLower,
	"default": func(def, v any) any {
		if v == nil || fmt.Sprint(v) == "" {
			return def
		}
		return v
	},
	"hasPrefix": strings.HasPrefix,
	"replace":   strings.ReplaceAll,
	"json": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(b)
	},
}

func (m *Manager) renderConfigFiles(p *entity.Plugin, man *Manifest, cfg map[string]any) error {
	vars := RenderVars{
		Platform: map[string]string{
			"dataDir":     p.DataPath,
			"installDir":  p.InstallPath,
			"home":        m.lay.Home,
			"pluginKey":   p.Key,
			"version":     p.Version,
			"arch":        GOARCH(),
			"logDir":      filepath.Join(p.DataPath, "logs"),
			"confDir":     filepath.Join(p.DataPath, "conf"),
			"runtime":     p.Runtime,
			"platformURL": fmt.Sprintf("http://127.0.0.1:%d", m.cfg.App.Port),
		},
		Config: cfg,
	}
	data := map[string]any{"platform": vars.Platform}
	for k, v := range cfg {
		data[k] = v
	}

	for _, cf := range man.ConfigFiles {
		target := expandVars(cf.Path, vars.Platform)
		tmplRel := cf.Template
		if tmplRel == "" {
			tmplRel = filepath.Base(target) + ".tmpl"
		}
		tmplPath := filepath.Join(p.InstallPath, "templates", tmplRel)
		body, err := os.ReadFile(tmplPath)
		if err != nil {
			if cf.Optional {
				continue
			}
			return fmt.Errorf("模板不存在: templates/%s", tmplRel)
		}
		tpl, err := template.New(tmplRel).Funcs(templateFuncs).Option("missingkey=zero").Parse(string(body))
		if err != nil {
			return fmt.Errorf("模板 %s 语法错误: %w", tmplRel, err)
		}
		var sb strings.Builder
		if err := tpl.Execute(&sb, data); err != nil {
			return fmt.Errorf("渲染模板 %s 失败: %w", tmplRel, err)
		}
		mode := os.FileMode(0o644)
		if cf.Mode != "" {
			if v, err := strconv.ParseUint(cf.Mode, 8, 32); err == nil {
				mode = os.FileMode(v)
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(sb.String()), mode); err != nil {
			return err
		}
	}
	return nil
}

// renderArgs 渲染启动参数。
func (m *Manager) renderArgs(p *entity.Plugin, man *Manifest, cfg map[string]any) []string {
	vars := map[string]string{
		"dataDir":    p.DataPath,
		"installDir": p.InstallPath,
		"home":       m.lay.Home,
		"pluginKey":  p.Key,
		"version":    p.Version,
		"logDir":     filepath.Join(p.DataPath, "logs"),
		"confDir":    filepath.Join(p.DataPath, "conf"),
	}
	data := map[string]any{"platform": vars}
	for k, v := range cfg {
		data[k] = v
	}
	out := make([]string, 0, len(man.Args))
	for _, a := range man.Args {
		rendered, err := renderString(a, data)
		if err != nil {
			out = append(out, a)
			continue
		}
		out = append(out, rendered)
	}
	return out
}

func renderString(s string, data map[string]any) (string, error) {
	if !strings.Contains(s, "{{") {
		return expandVars(s, nil), nil
	}
	tpl, err := template.New("arg").Funcs(templateFuncs).Option("missingkey=zero").Parse(s)
	if err != nil {
		return s, err
	}
	var sb strings.Builder
	if err := tpl.Execute(&sb, data); err != nil {
		return s, err
	}
	return sb.String(), nil
}

// expandVars 展开 {{ .platform.xxx }} 形式的路径变量。
func expandVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{ .platform."+k+" }}", v)
		s = strings.ReplaceAll(s, "{{.platform."+k+"}}", v)
	}
	return s
}

// ---------- 日志 ----------

// Logs 读取插件日志末尾 N 行。
func (m *Manager) Logs(ctx context.Context, key string, lines int) (string, error) {
	p, err := m.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if lines <= 0 || lines > 5000 {
		lines = m.cfg.Plugin.LogTailLines
	}
	if p.Runtime == RuntimeDocker {
		return m.dockerLogs(ctx, key, lines)
	}
	logPath := filepath.Join(p.DataPath, "logs", "plugin.log")
	return tailFile(logPath, lines)
}

// ---------- 内部工具 ----------

func (m *Manager) manifestOf(p *entity.Plugin) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(p.InstallPath, "manifest.json"))
	if err != nil {
		// 回落到数据库里保存的清单
		if p.Manifest == "" {
			return nil, errs.Invalid("插件清单丢失，请重新安装插件")
		}
		man, perr := ParseManifest([]byte(p.Manifest))
		if perr != nil {
			return nil, errs.InvalidErr(perr)
		}
		return man, nil
	}
	return ParseManifest(raw)
}

func (m *Manager) setError(ctx context.Context, key string, cause error) {
	if cause == nil {
		return
	}
	_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).Where("key = ?", key).Updates(map[string]any{
		"status":     StatusError,
		"last_error": cause.Error(),
		"pid":        0,
	}).Error
}

func findManifestRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		sub := filepath.Join(dir, dirs[0])
		if _, err := os.Stat(filepath.Join(sub, "manifest.json")); err == nil {
			return sub, nil
		}
	}
	return "", fmt.Errorf("插件包内未找到 manifest.json（请把清单放在包根目录）")
}

func archiveExt(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip", ".tar"} {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return ".zip"
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件目录不允许包含符号链接: %s", rel)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// tailFile 读取文件末尾 N 行（不整文件读入内存）。
func tailFile(path string, lines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	const chunk = 32 * 1024
	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := stat.Size()
	var (
		buf      []byte
		offset   = size
		found    int
		acc      []byte
	)
	for offset > 0 && found <= lines {
		readSize := int64(chunk)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		part := make([]byte, readSize)
		if _, err := f.ReadAt(part, offset); err != nil && err != io.EOF {
			return "", err
		}
		buf = append(part, buf...)
		found = strings.Count(string(buf), "\n")
		acc = buf
	}
	text := string(acc)
	parts := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n"), nil
}

// AutoStartAll 平台启动时拉起需要自启的插件。
func (m *Manager) AutoStartAll(ctx context.Context) {
	if !m.cfg.Plugin.AutoStart {
		return
	}
	rows, err := m.List(ctx)
	if err != nil {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	for _, p := range rows {
		if p.Enabled && p.AutoStart {
			if err := m.Start(ctx, p.Key); err != nil {
				m.setError(ctx, p.Key, err)
			}
		}
	}
}

// Shutdown 平台退出时停止所有运行中的插件。
func (m *Manager) Shutdown() {
	m.mu.Lock()
	keys := make([]string, 0, len(m.procs))
	for k := range m.procs {
		keys = append(keys, k)
	}
	m.mu.Unlock()
	for _, k := range keys {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = m.stopByKey(ctx, k)
		cancel()
	}
}

func (m *Manager) stopByKey(ctx context.Context, key string) error {
	p, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	return m.stopLocked(ctx, p, true)
}

// Reconcile 平台启动时的状态校准：数据库里残留 running 但进程已不存在，纠正为 stopped。
func (m *Manager) Reconcile(ctx context.Context) {
	_ = m.db.WithContext(ctx).Model(&entity.Plugin{}).
		Where("status = ?", StatusRunning).
		Updates(map[string]any{"status": StatusStopped, "pid": 0}).Error
}
