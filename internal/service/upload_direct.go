package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/storage"
)

// 前端直传 OSS（Presigned PUT）方案：
//
//	浏览器 --(1 要签名)--> 后端 --(2 返回签名 URL)--> 浏览器
//	浏览器 --(3 直接把文件 PUT 到 OSS，不经过应用服务器)--> OSS
//	浏览器 --(4 回执 objectKey)--> 后端 --(5 落 sys_file 记录)--> DB
//
// 好处：上行带宽不再占用应用服务器与内网链路，大文件/高并发上传时应用只处理「签名 + 记账」。
// 代价：需要 OSS 侧配置 CORS 允许站点域名 PUT；文件安全校验（扩展名/大小）只能靠签名条件与前端预检，
// 因此签名时把 objectKey、有效期、内容长度写死，回执时再校验 objectKey 是否属于本租户本次签名规则。
// 本地磁盘存储不支持直传，前端会自动回退到「服务端上传」接口。

// UploadTicketReq 直传签名入参。
type UploadTicketReq struct {
	OriginName string `json:"originName" binding:"required,max=255"`
	BizType    string `json:"bizType" binding:"max=32"`
	Size       int64  `json:"size"`
}

// UploadTicketVO 直传签名出参。
type UploadTicketVO struct {
	// ObjectKey 对象键（回执时原样带回）
	ObjectKey string `json:"objectKey"`
	// UploadURL 签名上传地址（PUT）
	UploadURL string `json:"uploadUrl"`
	// Method 上传方法，固定 PUT
	Method string `json:"method"`
	// Headers 需要带上的请求头
	Headers map[string]string `json:"headers"`
	// URL 上传成功后的访问地址
	URL string `json:"url"`
	// ExpiresIn 签名有效期（秒）
	ExpiresIn int `json:"expiresIn"`
	// Storage 存储类型（oss）
	Storage string `json:"storage"`
}

// OSSRecordReq 直传完成回执入参。
type OSSRecordReq struct {
	ObjectKey  string `json:"objectKey" binding:"required,max=255"`
	OriginName string `json:"originName" binding:"required,max=255"`
	BizType    string `json:"bizType" binding:"max=32"`
	Size       int64  `json:"size"`
	Mime       string `json:"mime" binding:"max=128"`
}

// buildObjectKey 生成对象键：年/月/日/[业务]/日期时间-雪花ID.扩展名。
// 目录按上传日期分层，便于对象存储控制台与磁盘直接翻阅；文件名带日期时间，便于脱离数据库也能识别。
func buildObjectKey(bizType, ext string) string {
	now := time.Now()
	stamp := now.Format("20060102-150405")
	if bizType != "" {
		return fmt.Sprintf("%s/%s/%s-%s.%s",
			now.Format("2006/01/02"), strings.Trim(bizType, "/"), stamp, id.NewString(), ext)
	}
	return fmt.Sprintf("%s/%s-%s.%s", now.Format("2006/01/02"), stamp, id.NewString(), ext)
}

// normalizeBizType 规整业务类型：仅字母/数字/下划线/短横线，最长 32。
func normalizeBizType(bizType string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, bizType)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "default"
	}
	if len(clean) > 32 {
		return "", errs.ErrFieldTooLong
	}
	return clean, nil
}

// DirectUploadEnabled 当前是否允许前端直传（存储为 OSS 且配置开启）。
func (s *Service) DirectUploadEnabled() bool {
	if s.Storage == nil || s.Storage.Type() != "oss" {
		return false
	}
	return s.Cfg.Storage.OSS.DirectUpload
}

// CreateUploadTicket 为前端直传生成签名。
func (s *Service) CreateUploadTicket(ctx context.Context, req *UploadTicketReq) (*UploadTicketVO, error) {
	if !s.DirectUploadEnabled() {
		// 用可读文案：前端据此判断「直传不可用」并回退服务端上传
		return nil, errs.Invalid("当前存储不支持前端直传（需配置 storage.type=oss），请改用服务端上传")
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(req.OriginName), "."))
	if ext == "" || !s.extAllowed(ext) {
		return nil, errs.ErrFileType
	}
	maxSize := int64(s.Cfg.Upload.MaxSizeMB) * 1024 * 1024
	if maxSize > 0 && req.Size > maxSize {
		return nil, errs.ErrFileTooLarge
	}

	bizType := ""
	if req.BizType != "" {
		clean, err := normalizeBizType(req.BizType)
		if err != nil {
			return nil, err
		}
		bizType = clean
	}

	objectKey := buildObjectKey(bizType, ext)
	expire := time.Duration(defaultInt(s.Cfg.Storage.OSS.SignExpire, 900)) * time.Second

	uploadURL, err := s.Storage.PresignPut(ctx, objectKey, expire, "")
	if err != nil {
		return nil, errs.ErrStorage.WithErr(err)
	}
	url, err := s.Storage.URL(ctx, objectKey, expire)
	if err != nil {
		return nil, errs.ErrStorage.WithErr(err)
	}

	return &UploadTicketVO{
		ObjectKey: objectKey,
		UploadURL: uploadURL,
		Method:    "PUT",
		Headers:   map[string]string{},
		URL:       url,
		ExpiresIn: int(expire.Seconds()),
		Storage:   s.Storage.Type(),
	}, nil
}

// RecordDirectUpload 直传完成后落库（等价于服务端上传写 sys_file 的那一步）。
func (s *Service) RecordDirectUpload(ctx context.Context, req *OSSRecordReq, p *principal.Info) (*entity.File, error) {
	if s.Storage == nil || s.Storage.Type() != "oss" {
		return nil, errs.InvalidErr(storage.ErrDirectUploadUnsupported)
	}

	objectKey := strings.TrimPrefix(strings.TrimSpace(req.ObjectKey), "/")
	// 对象键必须是我们生成的规则（年/月/日/…/日期时间-雪花ID.扩展名），防止前端写入任意路径
	if !validObjectKey(objectKey) {
		return nil, errs.InvalidErr(fmt.Errorf("非法的对象键: %s", objectKey))
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(objectKey), "."))
	if !s.extAllowed(ext) {
		return nil, errs.ErrFileType
	}
	maxSize := int64(s.Cfg.Upload.MaxSizeMB) * 1024 * 1024
	if maxSize > 0 && req.Size > maxSize {
		return nil, errs.ErrFileTooLarge
	}

	// 可选：向 OSS 校验对象确实存在，避免「只记账没文件」
	if s.Cfg.Storage.OSS.VerifyExists {
		ok, err := s.Storage.Exists(ctx, objectKey)
		if err != nil || !ok {
			return nil, errs.ErrUploadFailed.WithErr(fmt.Errorf("对象不存在或不可访问: %s", objectKey))
		}
	}

	url, err := s.Storage.URL(ctx, objectKey, 0)
	if err != nil {
		return nil, errs.ErrStorage.WithErr(err)
	}

	bizType := "default"
	if req.BizType != "" {
		clean, err := normalizeBizType(req.BizType)
		if err != nil {
			return nil, err
		}
		bizType = clean
	}

	file := &entity.File{
		Base:        entity.Base{ID: id.Next()},
		StorageType: s.Storage.Type(),
		Bucket:      s.Cfg.Storage.OSS.Bucket,
		ObjectKey:   objectKey,
		OriginName:  req.OriginName,
		Ext:         ext,
		Mime:        req.Mime,
		Size:        req.Size,
		URL:         url,
		BizType:     bizType,
		Status:      1,
		TenantID:    p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(file).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return file, nil
}

// validObjectKey 校验对象键是否符合「年/月/日/…」分层规则。
func validObjectKey(key string) bool {
	parts := strings.Split(key, "/")
	if len(parts) < 4 {
		return false
	}
	// 前三段必须是 YYYY / MM / DD
	if len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return false
	}
	for _, p := range parts[:3] {
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	if _, err := time.Parse("2006/01/02", strings.Join(parts[:3], "/")); err != nil {
		return false
	}
	// 不得包含 .. 之类的穿越片段
	for _, p := range parts {
		if p == ".." || strings.Contains(p, "\\") {
			return false
		}
	}
	return true
}
