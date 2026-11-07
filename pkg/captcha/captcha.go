// Package captcha 图形验证码：base64 输出 + Redis 存储，支持过期与一次性校验。
package captcha

import (
	"context"
	"time"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

// RedisStore 验证码存储实现（base64Captcha.Store）。
type RedisStore struct {
	rdb    *redis.Client
	prefix string
	ttl    time.Duration
}

func NewRedisStore(rdb *redis.Client, prefix string, ttl time.Duration) *RedisStore {
	return &RedisStore{rdb: rdb, prefix: prefix, ttl: ttl}
}

func (s *RedisStore) key(id string) string { return s.prefix + "captcha:" + id }

func (s *RedisStore) Set(id, value string) error {
	return s.rdb.Set(context.Background(), s.key(id), value, s.ttl).Err()
}

func (s *RedisStore) Get(id string, clear bool) string {
	ctx := context.Background()
	v, err := s.rdb.Get(ctx, s.key(id)).Result()
	if err != nil {
		return ""
	}
	if clear {
		s.rdb.Del(ctx, s.key(id))
	}
	return v
}

func (s *RedisStore) Verify(id, answer string, clear bool) bool {
	want := s.Get(id, clear)
	return want != "" && want == answer
}

// Service 验证码服务。
type Service struct {
	captcha *base64Captcha.Captcha
	enabled bool
}

// NewService 创建验证码服务；store 传入 RedisStore。
func NewService(store base64Captcha.Store, enabled bool) *Service {
	// 数字+字母混合，宽 240 高 80，长度 5，干扰强度 0.6
	driver := base64Captcha.NewDriverString(80, 240, 0, base64Captcha.OptionShowSlimeLine, 5,
		"23456789abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ", nil, nil, nil)
	return &Service{captcha: base64Captcha.NewCaptcha(driver, store), enabled: enabled}
}

// Enabled 是否开启验证码。
func (s *Service) Enabled() bool { return s.enabled }

// Generate 生成验证码，返回 id 与 base64 图片（data:image/png;base64,...）。
func (s *Service) Generate() (id, image string, err error) {
	id, image, _, err = s.captcha.Generate()
	return
}

// Verify 校验验证码（一次性，校验后即删除）。
func (s *Service) Verify(id, answer string) bool {
	if !s.enabled {
		return true
	}
	if id == "" || answer == "" {
		return false
	}
	return s.captcha.Verify(id, answer, true)
}
