package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"web-basic/internal/logger"
	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
)

// ============================ 岗位 ============================

// PostReq 岗位入参。
type PostReq struct {
	Code   string `json:"code" binding:"required,max=64"`
	Name   string `json:"name" binding:"required,max=64"`
	Sort   int    `json:"sort"`
	Status int    `json:"status"`
	Remark string `json:"remark"`
}

// ListPosts 岗位列表。
func (s *Service) ListPosts(ctx context.Context, p *principal.Info) ([]entity.Post, error) {
	list, err := s.Repo.ListPosts(ctx, p.TenantID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreatePost 新增岗位。
func (s *Service) CreatePost(ctx context.Context, req *PostReq, p *principal.Info) (uint64, error) {
	now := time.Now()
	post := &entity.Post{
		BaseLite: entity.BaseLite{ID: id.Next(), CreateTime: now, UpdateTime: &now, Remark: req.Remark},
		Code:     req.Code,
		Name:     req.Name,
		Sort:     req.Sort,
		Status:   req.Status,
		TenantID: p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(post).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return post.ID, nil
}

// UpdatePost 修改岗位。
func (s *Service) UpdatePost(ctx context.Context, postID uint64, req *PostReq, p *principal.Info) error {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Post{}).Where("id = ?", postID)
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	err := repository.Result(tx.Updates(map[string]any{
		"code": req.Code, "name": req.Name, "sort": req.Sort, "status": req.Status, "remark": req.Remark,
	}))
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeletePost 删除岗位。
func (s *Service) DeletePost(ctx context.Context, postID uint64, p *principal.Info) error {
	var post entity.Post
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", postID)
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if err := tx.First(&post).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Post{}, postID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("post_id = ?", postID).Delete(&entity.UserPost{}).Error
	return nil
}

// ============================ 字典 ============================

// DictTypeReq 字典类型入参。
type DictTypeReq struct {
	Name   string `json:"name" binding:"required,max=100"`
	Code   string `json:"code" binding:"required,max=100"`
	Status int    `json:"status"`
	Remark string `json:"remark"`
}

// DictDataReq 字典项入参。
type DictDataReq struct {
	DictCode  string `json:"dictCode" binding:"required,max=100"`
	Label     string `json:"label" binding:"required,max=100"`
	Value     string `json:"value" binding:"required,max=100"`
	Sort      int    `json:"sort"`
	TagType   string `json:"tagType"`
	CssClass  string `json:"cssClass"`
	IsDefault bool   `json:"isDefault"`
	Status    int    `json:"status"`
	Remark    string `json:"remark"`
}

// ListDictTypes 字典类型列表。
func (s *Service) ListDictTypes(ctx context.Context) ([]entity.DictType, error) {
	var list []entity.DictType
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.DictType{}).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateDictType 新增字典类型。
func (s *Service) CreateDictType(ctx context.Context, req *DictTypeReq) (uint64, error) {
	now := time.Now()
	item := &entity.DictType{
		BaseLite: entity.BaseLite{ID: id.Next(), CreateTime: now, UpdateTime: &now, Remark: req.Remark},
		Name:     req.Name, Code: req.Code, Status: req.Status,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(item).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return item.ID, nil
}

// UpdateDictType 修改字典类型。
func (s *Service) UpdateDictType(ctx context.Context, id uint64, req *DictTypeReq) error {
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.DictType{}, id, map[string]any{
		"name": req.Name, "code": req.Code, "status": req.Status, "remark": req.Remark,
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeleteDictType 删除字典类型及其字典项。
func (s *Service) DeleteDictType(ctx context.Context, id uint64) error {
	var code string
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.DictType{}).Where("id = ?", id).Pluck("code", &code).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.DictType{}, id)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("dict_code = ?", code).Delete(&entity.DictData{}).Error
	return nil
}

// DictData 按字典编码取数据（前端字典组件高频调用）。
func (s *Service) DictData(ctx context.Context, code string) ([]entity.DictData, error) {
	list, err := s.Repo.DictDataByCode(ctx, code)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateDictData 新增字典项。
func (s *Service) CreateDictData(ctx context.Context, req *DictDataReq) (uint64, error) {
	now := time.Now()
	item := &entity.DictData{
		BaseLite: entity.BaseLite{ID: id.Next(), CreateTime: now, UpdateTime: &now, Remark: req.Remark},
		DictCode: req.DictCode, Label: req.Label, Value: req.Value, Sort: req.Sort,
		TagType: req.TagType, CssClass: req.CssClass, IsDefault: req.IsDefault, Status: req.Status,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(item).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return item.ID, nil
}

// UpdateDictData 修改字典项。
func (s *Service) UpdateDictData(ctx context.Context, id uint64, req *DictDataReq) error {
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.DictData{}, id, map[string]any{
		"label": req.Label, "value": req.Value, "sort": req.Sort, "tag_type": req.TagType,
		"css_class": req.CssClass, "is_default": req.IsDefault, "status": req.Status, "remark": req.Remark,
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeleteDictData 删除字典项。
func (s *Service) DeleteDictData(ctx context.Context, id uint64) error {
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.DictData{}, id)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ============================ 参数 ============================

// ConfigReq 参数入参。
type ConfigReq struct {
	Name      string `json:"name" binding:"required,max=100"`
	Key       string `json:"key" binding:"required,max=100"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	IsPublic  bool   `json:"isPublic"`
	Remark    string `json:"remark"`
}

// ListConfigs 参数列表。
func (s *Service) ListConfigs(ctx context.Context, p *principal.Info) ([]entity.Config, error) {
	var list []entity.Config
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Config{}).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateConfig 新增参数。
func (s *Service) CreateConfig(ctx context.Context, req *ConfigReq, p *principal.Info) (uint64, error) {
	now := time.Now()
	item := &entity.Config{
		Base: entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		Name: req.Name, Key: req.Key, Value: req.Value, ValueType: req.ValueType, IsPublic: req.IsPublic,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(item).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	return item.ID, nil
}

// UpdateConfig 修改参数。
func (s *Service) UpdateConfig(ctx context.Context, id uint64, req *ConfigReq, p *principal.Info) error {
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Config{}, id, map[string]any{
		"name": req.Name, "config_key": req.Key, "value": req.Value, "value_type": req.ValueType,
		"is_public": req.IsPublic, "remark": req.Remark, "update_by": p.UserID, "update_time": time.Now(),
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 安全类开关改完立即生效，不必等缓存过期
	InvalidateSwitchCache(req.Key)
	return nil
}

// DeleteConfig 删除参数（内置参数不可删）。
func (s *Service) DeleteConfig(ctx context.Context, id uint64) error {
	var builtin bool
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Config{}).Where("id = ?", id).Pluck("is_builtin", &builtin).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if builtin {
		return errs.ErrRecordUsed
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Config{}, id)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// PublicConfigs 免登录公开参数（登录页标题、Logo、验证码/滑块开关等）。
//
// 除了 sys_config 里 is_public=1 的键值，这里额外把「运行时开关」按最终生效值写回，
// 保证前端看到的就是后端真正执行的策略（例如配置文件总闸关闭时，前端不必再显示验证码）。
func (s *Service) PublicConfigs(ctx context.Context) (map[string]string, error) {
	m, err := s.Repo.PublicConfigs(ctx)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if m == nil {
		m = map[string]string{}
	}
	m[ConfigKeyLoginCaptcha] = strconv.FormatBool(s.loginCaptchaEnabled(ctx))
	m[ConfigKeyLoginSlider] = strconv.FormatBool(s.LoginSliderEnabled(ctx))
	// 是否支持前端直传（存储为 OSS 且已开启）：前端据此决定要不要先申请签名
	m[ConfigKeyUploadDirect] = strconv.FormatBool(s.DirectUploadEnabled())
	return m, nil
}

// ============================ 租户 ============================

// TenantReq 租户入参。
type TenantReq struct {
	Name         string `json:"name" binding:"required,max=100"`
	Code         string `json:"code" binding:"required,max=64"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	MaxUsers     int    `json:"maxUsers"`
	ExpireAt     string `json:"expireAt"`
	Status       int    `json:"status"`
	Remark       string `json:"remark"`
}

// ListTenants 租户列表。
func (s *Service) ListTenants(ctx context.Context) ([]entity.Tenant, error) {
	var list []entity.Tenant
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateTenant 新增租户（自动创建根部门）。
func (s *Service) CreateTenant(ctx context.Context, req *TenantReq, p *principal.Info) (uint64, error) {
	now := time.Now()
	t := &entity.Tenant{
		Base: entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		Name: req.Name, Code: req.Code, ContactName: req.ContactName, ContactPhone: req.ContactPhone,
		MaxUsers: req.MaxUsers, ExpireAt: req.ExpireAt, Status: req.Status,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(t).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Create(&entity.Dept{
		Base:     entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
		ParentID: 0, Ancestors: "0", Name: req.Name, Code: "HQ", Sort: 1, Status: 1, TenantID: t.ID,
	}).Error
	return t.ID, nil
}

// UpdateTenant 修改租户。
func (s *Service) UpdateTenant(ctx context.Context, tenantID uint64, req *TenantReq, p *principal.Info) error {
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Tenant{}, tenantID, map[string]any{
		"name": req.Name, "code": req.Code, "contact_name": req.ContactName, "contact_phone": req.ContactPhone,
		"max_users": req.MaxUsers, "expire_at": req.ExpireAt, "status": req.Status, "remark": req.Remark,
		"update_by": p.UserID, "update_time": time.Now(),
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// DeleteTenant 删除租户（默认租户与有用户的租户不可删）。
func (s *Service) DeleteTenant(ctx context.Context, tenantID uint64) error {
	if tenantID == s.Cfg.Tenant.DefaultID {
		return errs.ErrRecordUsed
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.User{}).Where("tenant_id = ?", tenantID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.ErrRecordUsed
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Tenant{}, tenantID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 级联清理该租户的部门（含建租户时自动生成的根部门），避免留下孤儿数据
	if err := s.Repo.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Delete(&entity.Dept{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ============================ 文件 ============================

// UploadFile 上传文件：大小 / 扩展名 / 内容类型三重校验 + SHA-256 记录。
func (s *Service) UploadFile(ctx context.Context, fh *multipart.FileHeader, bizType string, p *principal.Info) (*entity.File, error) {
	maxSize := int64(s.Cfg.Upload.MaxSizeMB) * 1024 * 1024
	if maxSize > 0 && fh.Size > maxSize {
		return nil, errs.ErrFileTooLarge
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	if !s.extAllowed(ext) {
		return nil, errs.ErrFileType
	}

	// bizType 来自前端，做白名单化：仅字母/数字/下划线/短横线，最长 32。
	// （目录穿越已由 storage.safePath 兜底，这里再加业务层约束，避免脏值污染对象键与统计口径）
	if bizType != "" {
		clean, err := normalizeBizType(bizType)
		if err != nil {
			return nil, err
		}
		bizType = clean
	}

	src, err := fh.Open()
	if err != nil {
		return nil, errs.ErrUploadFailed.WithErr(err)
	}
	defer src.Close()

	hasher := sha256.New()
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	head = head[:n]
	detected := http.DetectContentType(head)
	if isDangerous(detected) {
		return nil, errs.ErrFileType.WithErr(fmt.Errorf("检测到不安全的内容类型: %s", detected))
	}

	body := io.TeeReader(io.MultiReader(bytes.NewReader(head), src), hasher)

	// 对象键：年/月/日/[业务]/日期时间-雪花ID.扩展名
	// 统一由 buildObjectKey 生成，保证「服务端上传」与「前端直传」路径规则一致。
	objectKey := buildObjectKey(bizType, ext)

	url, err := s.Storage.Put(ctx, objectKey, body, fh.Size, detected)
	if err != nil {
		return nil, errs.ErrStorage.WithErr(err)
	}

	now := time.Now()
	rec := &entity.File{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
		StorageType: s.Storage.Type(),
		ObjectKey:   objectKey,
		OriginName:  fh.Filename,
		Ext:         ext,
		Mime:        detected,
		Size:        fh.Size,
		Hash:        hex.EncodeToString(hasher.Sum(nil)),
		URL:         url,
		BizType:     bizType,
		Status:      1,
		TenantID:    p.TenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(rec).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return rec, nil
}

func (s *Service) extAllowed(ext string) bool {
	if ext == "" {
		return false
	}
	for _, allow := range s.Cfg.Upload.AllowExt {
		if strings.EqualFold(strings.TrimPrefix(allow, "."), ext) {
			return true
		}
	}
	return false
}

// isDangerous 拦截可执行/脚本类内容（防 webshell）。
func isDangerous(contentType string) bool {
	switch {
	case strings.Contains(contentType, "x-msdownload"),
		strings.Contains(contentType, "x-executable"),
		strings.Contains(contentType, "x-sharedlib"),
		strings.Contains(contentType, "x-shellscript"),
		contentType == "text/html":
		return true
	}
	return false
}

// FileQuery 文件列表查询。
//
// StartDate / EndDate 为「上传日期」区间（YYYY-MM-DD，含首含尾），
// 用于按日期目录（对象键 年/月/日/业务）查找历史文件。
type FileQuery struct {
	entity.PageQuery
	OriginName string `form:"originName"`
	BizType    string `form:"bizType"`
	StartDate  string `form:"startDate"`
	EndDate    string `form:"endDate"`
}

// ListFiles 文件列表。
func (s *Service) ListFiles(ctx context.Context, q *FileQuery, p *principal.Info) ([]entity.File, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.File{}).Where("tenant_id = ?", p.TenantID)
	if q.OriginName != "" {
		tx = tx.Where("origin_name LIKE ?", q.OriginName+"%")
	}
	if q.BizType != "" {
		tx = tx.Where("biz_type = ?", q.BizType)
	}
	if day, err := time.ParseInLocation("2006-01-02", q.StartDate, time.Local); err == nil {
		tx = tx.Where("create_time >= ?", day)
	}
	if day, err := time.ParseInLocation("2006-01-02", q.EndDate, time.Local); err == nil {
		tx = tx.Where("create_time < ?", day.AddDate(0, 0, 1)) // 含当天
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.File, 0, q.PageSize)
	if err := tx.Order("create_time DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// DeleteFile 删除文件：先删库记录（按租户校验），再尽力删除存储对象。
// 顺序很重要：若先删对象而删库失败，会留下"记录在、文件没了"的坏数据。
func (s *Service) DeleteFile(ctx context.Context, fileID uint64, p *principal.Info) error {
	var f entity.File
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", fileID)
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if err := tx.First(&f).Error; err != nil {
		return errs.ErrFileNotFound.WithErr(err)
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.File{}, fileID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if err := s.Storage.Delete(ctx, f.ObjectKey); err != nil {
		logger.S().Warnw("delete storage object failed", "id", fileID, "key", f.ObjectKey, "err", err)
	}
	return nil
}

// FileURL 获取文件访问地址（私有文件返回签名地址）。
func (s *Service) FileURL(ctx context.Context, fileID uint64, p *principal.Info) (string, error) {
	var f entity.File
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", fileID)
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if err := tx.First(&f).Error; err != nil {
		return "", errs.ErrFileNotFound.WithErr(err)
	}
	if !f.IsPrivate {
		return f.URL, nil
	}
	ttl := time.Hour
	if s.Cfg.Storage.OSS.SignExpire > 0 {
		ttl = time.Duration(s.Cfg.Storage.OSS.SignExpire) * time.Second
	}
	return s.Storage.URL(ctx, f.ObjectKey, ttl)
}

// ============================ 日志 ============================

// LogQuery 日志查询。
type LogQuery struct {
	entity.PageQuery
	Username string `form:"username"`
	Status   *int   `form:"status"`
	StartAt  string `form:"startTime"`
	EndAt    string `form:"endTime"`
}

// ListLoginLogs 登录日志。
func (s *Service) ListLoginLogs(ctx context.Context, q *LogQuery, p *principal.Info) ([]entity.LoginLog, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.LoginLog{})
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if q.Username != "" {
		tx = tx.Where("username LIKE ?", q.Username+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.StartAt != "" {
		tx = tx.Where("login_time >= ?", q.StartAt)
	}
	if q.EndAt != "" {
		tx = tx.Where("login_time <= ?", q.EndAt)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.LoginLog, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ListOperLogs 操作日志。
func (s *Service) ListOperLogs(ctx context.Context, q *LogQuery, p *principal.Info) ([]entity.OperLog, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.OperLog{})
	if p != nil && p.TenantID != 0 {
		tx = tx.Where("tenant_id = ?", p.TenantID)
	}
	if q.Username != "" {
		tx = tx.Where("oper_name LIKE ?", q.Username+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.OperLog, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ============================ 监控 ============================

// ServerInfo 服务器运行状态。
type ServerInfo struct {
	GoVersion   string  `json:"goVersion"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	NumCPU      int     `json:"numCpu"`
	Goroutines  int     `json:"goroutines"`
	HeapAllocMB float64 `json:"heapAllocMb"`
	SysMB       float64 `json:"sysMb"`
	UptimeSec   int64   `json:"uptimeSec"`
	MySQLOK     bool    `json:"mysqlOk"`
	RedisOK     bool    `json:"redisOk"`
}

var startedAt = time.Now()

// ServerInfo 采集运行时与依赖状态。
func (s *Service) ServerInfo(ctx context.Context) *ServerInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mysqlOK := false
	if sqlDB, err := s.Repo.DB.DB(); err == nil {
		mysqlOK = sqlDB.PingContext(ctx) == nil
	}
	return &ServerInfo{
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		NumCPU:      runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: float64(m.HeapAlloc) / 1024 / 1024,
		SysMB:       float64(m.Sys) / 1024 / 1024,
		UptimeSec:   int64(time.Since(startedAt).Seconds()),
		MySQLOK:     mysqlOK,
		RedisOK:     s.Cache.Ping(ctx) == nil,
	}
}
