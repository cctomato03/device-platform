// Package storage 文件存储抽象：本地磁盘（默认）与阿里云 OSS 两种实现。
//
// 设备平台默认走本地磁盘，落盘目录由配置决定（统一在数据根目录 <home>/storage 下），
// 通过 /static 路由直出；如需接对象存储，把 storage.type 改成 oss 并填 OSS 参数即可。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"device-platform/internal/config"
)

// ErrDirectUploadUnsupported 当前存储不支持前端直传。
var ErrDirectUploadUnsupported = errors.New("当前存储不支持前端直传")

// Storage 存储能力接口。
type Storage interface {
	// Type 存储类型：local | oss
	Type() string
	// Put 写入对象，返回可访问 URL。
	Put(ctx context.Context, key string, r io.Reader, size int64, mime string) (string, error)
	// Delete 删除对象。
	Delete(ctx context.Context, key string) error
	// URL 获取访问地址；ttl>0 时返回临时签名地址（本地存储忽略 ttl）。
	URL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Exists 判断对象是否存在。
	Exists(ctx context.Context, key string) (bool, error)
	// PresignPut 前端直传签名（仅 OSS 支持）。
	PresignPut(ctx context.Context, key string, expire time.Duration, mime string) (string, error)
}

// New 按配置创建存储实现。
func New(cfg config.Storage) (Storage, error) {
	switch strings.ToLower(cfg.Type) {
	case "", "local":
		return NewLocal(cfg.Local)
	case "oss":
		return newOSS(cfg.OSS)
	default:
		return nil, fmt.Errorf("不支持的存储类型: %s", cfg.Type)
	}
}

// ---------------- 本地磁盘 ----------------

// Local 本地磁盘存储。
type Local struct {
	baseDir   string
	urlPrefix string
}

// NewLocal 创建本地存储；目录不存在时自动创建。
func NewLocal(cfg config.LocalStore) (*Local, error) {
	base := cfg.BaseDir
	if base == "" {
		base = "./storage"
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	prefix := cfg.URLPrefix
	if prefix == "" {
		prefix = "/static"
	}
	return &Local{baseDir: base, urlPrefix: prefix}, nil
}

// Type 实现 Storage。
func (l *Local) Type() string { return "local" }

// abs 拼接并校验对象键，防止路径穿越（../../etc/passwd）。
func (l *Local) abs(key string) (string, error) {
	clean := filepath.Clean("/" + strings.ReplaceAll(key, "\\", "/"))
	p := filepath.Join(l.baseDir, filepath.FromSlash(clean))
	baseAbs, err := filepath.Abs(l.baseDir)
	if err != nil {
		return "", err
	}
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(pAbs, baseAbs) {
		return "", fmt.Errorf("非法对象键: %s", key)
	}
	return pAbs, nil
}

// Put 写文件到磁盘。
func (l *Local) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) (string, error) {
	p, err := l.abs(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return l.publicURL(key), nil
}

// Delete 删除文件。
func (l *Local) Delete(ctx context.Context, key string) error {
	p, err := l.abs(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// URL 本地文件直接走静态路由。
func (l *Local) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	p, err := l.abs(key)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return l.publicURL(key), nil
}

// Exists 判断文件是否存在。
func (l *Local) Exists(ctx context.Context, key string) (bool, error) {
	p, err := l.abs(key)
	if err != nil {
		return false, err
	}
	_, statErr := os.Stat(p)
	if statErr == nil {
		return true, nil
	}
	if os.IsNotExist(statErr) {
		return false, nil
	}
	return false, statErr
}

// PresignPut 本地存储不支持直传。
func (l *Local) PresignPut(ctx context.Context, key string, expire time.Duration, mime string) (string, error) {
	return "", ErrDirectUploadUnsupported
}

func (l *Local) publicURL(key string) string {
	clean := strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(key, "\\", "/")), "/")
	return strings.TrimSuffix(l.urlPrefix, "/") + "/" + clean
}

// ---------------- 阿里云 OSS ----------------

type ossStore struct {
	cfg    config.OSSStore
	bucket *oss.Bucket
}

func newOSS(cfg config.OSSStore) (*ossStore, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("storage.type=oss 时必须配置 endpoint 与 bucket")
	}
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("初始化 OSS 客户端失败: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS Bucket 失败: %w", err)
	}
	return &ossStore{cfg: cfg, bucket: bucket}, nil
}

// Type 实现 Storage。
func (o *ossStore) Type() string { return "oss" }

func (o *ossStore) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) (string, error) {
	opts := []oss.Option{}
	if mime != "" {
		opts = append(opts, oss.ContentType(mime))
	}
	if o.cfg.Private {
		opts = append(opts, oss.ObjectACL(oss.ACLPrivate))
	}
	if err := o.bucket.PutObject(key, r, opts...); err != nil {
		return "", err
	}
	return o.URL(ctx, key, 0)
}

func (o *ossStore) Delete(ctx context.Context, key string) error {
	return o.bucket.DeleteObject(key)
}

func (o *ossStore) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if o.cfg.Domain != "" && !o.cfg.Private {
		return strings.TrimSuffix(o.cfg.Domain, "/") + "/" + strings.TrimPrefix(key, "/"), nil
	}
	if o.cfg.Private || ttl > 0 {
		if ttl <= 0 {
			ttl = time.Duration(o.cfg.SignExpire) * time.Second
		}
		return o.bucket.SignURL(key, oss.HTTPGet, int64(ttl.Seconds()))
	}
	return o.bucket.SignURL(key, oss.HTTPGet, 3600)
}

func (o *ossStore) Exists(ctx context.Context, key string) (bool, error) {
	return o.bucket.IsObjectExist(key)
}

func (o *ossStore) PresignPut(ctx context.Context, key string, expire time.Duration, mime string) (string, error) {
	if expire <= 0 {
		expire = time.Duration(o.cfg.SignExpire) * time.Second
	}
	opts := []oss.Option{oss.ContentType(mime)}
	return o.bucket.SignURL(key, oss.HTTPPut, int64(expire.Seconds()), opts...)
}
