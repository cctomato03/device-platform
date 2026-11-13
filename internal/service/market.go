package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"device-platform/internal/model/entity"
	"device-platform/pkg/errs"
	"device-platform/pkg/id"
)

// 本文件：插件在线市场（远程索引 + 下载安装 + 版本升级）。
//
// 索引格式（GET 一个 HTTPS JSON）：
//
//	{
//	  "version": "1",
//	  "updatedAt": "2026-01-01T00:00:00Z",
//	  "plugins": [
//	    {
//	      "id": "frp", "name": "FRP 内网穿透", "version": "1.0.1",
//	      "category": "network", "arch": ["amd64","arm64"], "runtime": "process",
//	      "description": "...", "icon": "ri:global-line",
//	      "downloadUrl": "https://example.com/plugins/frp-1.0.1.zip",
//	      "sha256": "<zip 的 sha256>", "sizeBytes": 12345678
//	    }
//	  ]
//	}
//
// 完整性：索引响应头 `X-Index-Signature` 为「HMAC-SHA256(索引原始字节, trustKey)」的十六进制串；
// 插件包必须提供 sha256，平台下载后逐个校验，校验不过直接拒绝安装。

const (
	marketIndexTimeout = 20 * time.Second
	marketPkgTimeout   = 300 * time.Second
	maxIndexBytes      = 5 << 20   // 5MB
	maxPackageBytes    = 300 << 20 // 300MB
)

// MarketPlugin 索引中的一条插件。
type MarketPlugin struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Vendor      string   `json:"vendor"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Runtime     string   `json:"runtime"`
	Arch        []string `json:"arch"`
	Homepage    string   `json:"homepage"`
	DownloadURL string   `json:"downloadUrl"`
	SHA256      string   `json:"sha256"`
	SizeBytes   int64    `json:"sizeBytes"`
}

// MarketIndex 索引根结构。
type MarketIndex struct {
	Version   string         `json:"version"`
	UpdatedAt string         `json:"updatedAt"`
	Plugins   []MarketPlugin `json:"plugins"`
}

// MarketEntry 融合「市场条目 + 本地安装状态」的视图。
type MarketEntry struct {
	MarketPlugin
	SourceID         uint64 `json:"sourceId,string"`
	SourceName       string `json:"sourceName"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	Upgradable       bool   `json:"upgradable"`
	Compatible       bool   `json:"compatible"`
}

// marketCache 索引内存缓存（避免每次进页面都打远端）。
type marketCache struct {
	mu      sync.Mutex
	entries map[uint64][]MarketEntry
	at      map[uint64]time.Time
}

var marketMem = &marketCache{entries: map[uint64][]MarketEntry{}, at: map[uint64]time.Time{}}

// marketCacheTTL 索引缓存时间。
const marketCacheTTL = 5 * time.Minute

// PluginSourceReq 插件源入参。
type PluginSourceReq struct {
	ID       uint64 `json:"id,string"`
	Name     string `json:"name" binding:"required,max=64"`
	URL      string `json:"url"`
	TrustKey string `json:"trustKey"`
	Enabled  bool   `json:"enabled"`
	Remark   string `json:"remark"`
}

// PluginSourceView 插件源视图（不回传密钥，但告知是否已配置）。
type PluginSourceView struct {
	entity.PluginSource
	TrustKeyConfigured bool `json:"trustKeyConfigured"`
}

// ListPluginSources 插件源列表。
func (s *Service) ListPluginSources(ctx context.Context) ([]PluginSourceView, error) {
	var rows []entity.PluginSource
	if err := s.Repo.DB.WithContext(ctx).Order("create_time asc").Find(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]PluginSourceView, 0, len(rows))
	for i := range rows {
		out = append(out, PluginSourceView{PluginSource: rows[i], TrustKeyConfigured: rows[i].TrustKey != ""})
	}
	return out, nil
}

// SavePluginSource 新建/修改插件源。
func (s *Service) SavePluginSource(ctx context.Context, req *PluginSourceReq) (*entity.PluginSource, error) {
	if req.Enabled && strings.TrimSpace(req.URL) == "" {
		return nil, errs.Invalid("启用插件源必须填写索引地址")
	}
	if req.URL != "" && !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "http://") {
		return nil, errs.Invalid("索引地址需以 http(s):// 开头")
	}
	// 留空表示保持原密钥
	trustKey := req.TrustKey
	if trustKey == "" && req.ID != 0 {
		var old entity.PluginSource
		_ = s.Repo.DB.WithContext(ctx).Where("id = ?", req.ID).First(&old).Error
		trustKey = old.TrustKey
	}
	row := entity.PluginSource{
		Base:     entity.Base{ID: id.Next(), Remark: req.Remark},
		Name:     req.Name,
		URL:      strings.TrimSpace(req.URL),
		TrustKey: trustKey,
		Enabled:  req.Enabled,
		TenantID: s.Cfg.Tenant.DefaultID,
	}
	row.CreateTime = nowTime()
	if req.ID == 0 {
		if err := s.Repo.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
		return &row, nil
	}
	row.ID = req.ID
	res := s.Repo.DB.WithContext(ctx).Model(&entity.PluginSource{}).Where("id = ?", req.ID).Updates(map[string]any{
		"name": row.Name, "url": row.URL, "trust_key": row.TrustKey, "enabled": row.Enabled,
		"remark": row.Remark, "update_time": nowTime(),
	})
	if res.Error != nil {
		return nil, errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, errs.ErrNotFound
	}
	return &row, nil
}

// DeletePluginSource 删除插件源。
func (s *Service) DeletePluginSource(ctx context.Context, srcID uint64) error {
	res := s.Repo.DB.WithContext(ctx).Where("id = ?", srcID).Delete(&entity.PluginSource{})
	if res.Error != nil {
		return errs.ErrDB.WithErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	marketMem.mu.Lock()
	delete(marketMem.entries, srcID)
	delete(marketMem.at, srcID)
	marketMem.mu.Unlock()
	return nil
}

// ListMarket 拉取所有启用源的插件条目（带本地安装状态与兼容性判断）。
func (s *Service) ListMarket(ctx context.Context, force bool) ([]MarketEntry, error) {
	var sources []entity.PluginSource
	if err := s.Repo.DB.WithContext(ctx).Where("enabled = ?", true).Find(&sources).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	installed := map[string]string{}
	if s.Plugins != nil {
		if rows, err := s.Plugins.List(ctx); err == nil {
			for _, p := range rows {
				installed[p.Key] = p.Version
			}
		}
	}
	out := make([]MarketEntry, 0, 16)
	failures := make([]string, 0, 2)
	for _, src := range sources {
		entries, err := s.fetchIndex(ctx, &src, force)
		if err != nil {
			// 单个源失败不影响其它源；全部失败时把原因汇总返回，避免页面只显示空列表
			failures = append(failures, src.Name+"："+err.Error())
			continue
		}
		for i := range entries {
			e := entries[i]
			if v, ok := installed[e.ID]; ok {
				e.Installed = true
				e.InstalledVersion = v
				e.Upgradable = versionGreater(e.Version, v)
			}
			e.Compatible = archSupported(e.Arch, goArch())
			out = append(out, e)
		}
	}
	if len(out) == 0 && len(failures) > 0 {
		return nil, errs.Invalid("在线插件索引获取失败：" + strings.Join(failures, "；"))
	}
	return out, nil
}

// fetchIndex 拉取并校验索引（带内存缓存）。
func (s *Service) fetchIndex(ctx context.Context, src *entity.PluginSource, force bool) ([]MarketEntry, error) {
	marketMem.mu.Lock()
	if !force {
		if entries, ok := marketMem.entries[src.ID]; ok {
			if at := marketMem.at[src.ID]; time.Since(at) < marketCacheTTL {
				marketMem.mu.Unlock()
				return entries, nil
			}
		}
	}
	marketMem.mu.Unlock()

	body, sig, err := httpGetBytes(ctx, src.URL, marketIndexTimeout, maxIndexBytes)
	if err != nil {
		s.markSourceError(ctx, src.ID, err.Error())
		return nil, errs.InvalidErr(err)
	}
	// 签名校验（配置了 trustKey 就必须验，验不过直接拒绝）
	if src.TrustKey != "" {
		want := hmacHex(body, src.TrustKey)
		if !strings.EqualFold(strings.TrimSpace(sig), want) {
			err := fmt.Errorf("索引签名校验失败（请检查信任密钥或索引是否被篡改）")
			s.markSourceError(ctx, src.ID, err.Error())
			return nil, errs.InvalidErr(err)
		}
	}
	var idx MarketIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		s.markSourceError(ctx, src.ID, "索引 JSON 解析失败: "+err.Error())
		return nil, errs.Invalid("索引格式不正确：需要 JSON 对象（含 plugins 数组）")
	}
	entries := make([]MarketEntry, 0, len(idx.Plugins))
	for _, p := range idx.Plugins {
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.DownloadURL) == "" {
			continue
		}
		entries = append(entries, MarketEntry{
			MarketPlugin: p,
			SourceID:     src.ID,
			SourceName:   src.Name,
		})
	}

	marketMem.mu.Lock()
	marketMem.entries[src.ID] = entries
	marketMem.at[src.ID] = time.Now()
	marketMem.mu.Unlock()

	_ = s.Repo.DB.WithContext(ctx).Model(&entity.PluginSource{}).Where("id = ?", src.ID).Updates(map[string]any{
		"last_sync_at": nowTime().Format("2006-01-02 15:04:05"),
		"last_error":   "",
		"item_count":   len(entries),
	}).Error
	return entries, nil
}

// InstallFromMarket 从市场安装/升级插件（下载 → 校验 sha256 → 走统一安装流程）。
func (s *Service) InstallFromMarket(ctx context.Context, sourceID uint64, pluginID string) (*PluginView, error) {
	if s.Plugins == nil {
		return nil, errs.Invalid("插件管理器未初始化")
	}
	var src entity.PluginSource
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", sourceID).First(&src).Error; err != nil {
		return nil, errs.Invalid("插件源不存在")
	}
	entries, err := s.fetchIndex(ctx, &src, false)
	if err != nil {
		return nil, err
	}
	var target *MarketEntry
	for i := range entries {
		if entries[i].ID == pluginID {
			target = &entries[i]
			break
		}
	}
	if target == nil {
		return nil, errs.Invalid("该插件不在索引中：" + pluginID)
	}
	if !archSupported(target.Arch, goArch()) {
		return nil, errs.Invalid(fmt.Sprintf("插件不支持当前架构 %s（支持：%s）", goArch(), strings.Join(target.Arch, ", ")))
	}
	if netErr := ensureHTTPSScheme(target.DownloadURL); netErr != nil {
		return nil, netErr
	}

	// 下载到临时目录
	if err := os.MkdirAll(s.Paths.Tmp, 0o755); err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	tmp := filepath.Join(s.Paths.Tmp, "market-"+pluginID+"-"+nowTime().Format("20060102150405")+".zip")
	if err := downloadToFile(ctx, target.DownloadURL, tmp, maxPackageBytes); err != nil {
		return nil, errs.InvalidErr(fmt.Errorf("下载插件包失败: %w", err))
	}
	defer os.Remove(tmp)

	// sha256 校验（索引必须给出）
	if strings.TrimSpace(target.SHA256) == "" {
		return nil, errs.Invalid("索引未提供插件包 sha256，出于安全考虑拒绝安装")
	}
	sum, err := fileSHA256(tmp)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	if !strings.EqualFold(sum, target.SHA256) {
		return nil, errs.Invalid("插件包校验失败（sha256 不一致），已取消安装")
	}

	f, err := os.Open(tmp)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	defer f.Close()
	p, err := s.Plugins.InstallArchive(ctx, filepath.Base(tmp), f)
	if err != nil {
		return nil, err
	}
	marketMem.mu.Lock()
	delete(marketMem.at, sourceID) // 让下次列表刷新安装状态
	marketMem.mu.Unlock()
	v := s.toView(ctx, p, true)
	return &v, nil
}

// ---------- 内部工具 ----------

func (s *Service) markSourceError(ctx context.Context, id uint64, msg string) {
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.PluginSource{}).Where("id = ?", id).Updates(map[string]any{
		"last_error": truncateText(msg, 300),
		"last_sync_at": nowTime().Format("2006-01-02 15:04:05"),
	}).Error
}

// httpGetBytes 带超时与大小限制的 GET，返回 body 与指定响应头。
func httpGetBytes(ctx context.Context, url string, timeout time.Duration, maxBytes int64) ([]byte, string, error) {
	if err := ensureHTTPSScheme(url); err != nil {
		return nil, "", err
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "device-platform-market/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求索引失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("索引接口返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("索引体积超过上限（%d KB）", maxBytes>>10)
	}
	return body, resp.Header.Get("X-Index-Signature"), nil
}

// downloadToFile 下载到本地文件（限制大小）。
func downloadToFile(ctx context.Context, url, dst string, maxBytes int64) error {
	cctx, cancel := context.WithTimeout(ctx, marketPkgTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "device-platform-market/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回 %d", resp.StatusCode)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if n > maxBytes {
		return fmt.Errorf("插件包体积超过上限（%d MB）", maxBytes>>20)
	}
	return nil
}

// fileSHA256 计算文件 sha256（十六进制小写）。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hmacHex 计算 HMAC-SHA256 十六进制串。
func hmacHex(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ensureHTTPSScheme 只允许 http/https（防止 file:// 之类协议被滥用）。
func ensureHTTPSScheme(url string) error {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return nil
	}
	return errs.Invalid("仅支持 http(s) 地址：" + url)
}

// archSupported 判断插件是否支持当前架构（空列表表示不限）。
func archSupported(archs []string, arch string) bool {
	if len(archs) == 0 {
		return true
	}
	for _, a := range archs {
		if strings.EqualFold(a, arch) {
			return true
		}
	}
	return false
}

// versionGreater 比较语义化版本 a > b（只比较数字段，够用）。
func versionGreater(a, b string) bool {
	pa, pb := versionParts(a), versionParts(b)
	if pa == nil || pb == nil {
		return a != b
	}
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	segs := strings.Split(v, ".")
	out := make([]int, 0, len(segs))
	for _, s := range segs {
		// 去掉 -beta.1 之类后缀
		if i := strings.IndexAny(s, "-+ "); i >= 0 {
			s = s[:i]
		}
		n := 0
		for _, c := range s {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
