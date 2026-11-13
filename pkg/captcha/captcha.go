// Package captcha 图形验证码：base64 输出 + 进程内存储，支持过期与一次性校验。
package captcha

import (
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
)

// MemoryStore 验证码存储实现（base64Captcha.Store），单机部署无需 Redis。
type MemoryStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	data map[string]memItem
}

type memItem struct {
	value string
	exp   time.Time
}

// NewMemoryStore 创建内存验证码存储。
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	s := &MemoryStore{ttl: ttl, data: make(map[string]memItem, 32)}
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for now := range t.C {
			s.mu.Lock()
			for k, v := range s.data {
				if now.After(v.exp) {
					delete(s.data, k)
				}
			}
			s.mu.Unlock()
		}
	}()
	return s
}

// Set 保存验证码答案。
func (s *MemoryStore) Set(id, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = memItem{value: value, exp: time.Now().Add(s.ttl)}
	return nil
}

// Get 读取验证码答案（clear=true 时一次性删除）。
func (s *MemoryStore) Get(id string, clear bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.data[id]
	if !ok || time.Now().After(it.exp) {
		delete(s.data, id)
		return ""
	}
	if clear {
		delete(s.data, id)
	}
	return it.value
}

// Verify 校验验证码。
func (s *MemoryStore) Verify(id, answer string, clear bool) bool {
	want := s.Get(id, clear)
	return want != "" && want == answer
}

// Service 验证码服务。
type Service struct {
	captcha *base64Captcha.Captcha
	enabled bool
}

// NewService 创建验证码服务。
func NewService(store base64Captcha.Store, enabled bool) *Service {
	driver := base64Captcha.NewDriverString(80, 240, 0, base64Captcha.OptionShowSlimeLine, 5,
		"23456789abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ", nil, nil, nil)
	return &Service{captcha: base64Captcha.NewCaptcha(driver, store), enabled: enabled}
}

// Enabled 是否开启验证码。
func (s *Service) Enabled() bool { return s.enabled }

// SetEnabled 运行期开关（站点设置里改验证码开关后立即生效）。
func (s *Service) SetEnabled(v bool) { s.enabled = v }

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
