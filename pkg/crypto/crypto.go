// Package crypto 密码哈希、AES-GCM 对称加解密等。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 生成 bcrypt 哈希。
func HashPassword(raw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword 校验明文与哈希是否匹配。
func VerifyPassword(hash, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// CheckStrength 密码复杂度校验：长度 + 至少包含字母与数字。
// weakPasswords 常见弱口令黑名单（小写比较），命中直接拒绝并提示规则。
var weakPasswords = map[string]bool{
	"12345678": true, "123456789": true, "1234567890": true, "87654321": true,
	"password": true, "passw0rd": true, "password1": true, "p@ssw0rd": true,
	"admin123": true, "admin@123": true, "administrator": true, "qwerty123": true,
	"1qaz2wsx": true, "abc12345": true, "a1234567": true, "iloveyou": true,
	"welcome1": true, "changeme": true, "letmein1": true, "test1234": true,
}

// PasswordPolicy 密码强度规则（给前端展示用，与 CheckStrength 保持一致）。
type PasswordPolicy struct {
	// MinLength 最小长度（取配置 auth.pwd_min_len，且不小于 8）
	MinLength int `json:"minLength"`
	// RequireUpper 必须包含大写字母
	RequireUpper bool `json:"requireUpper"`
	// RequireLower 必须包含小写字母
	RequireLower bool `json:"requireLower"`
	// RequireDigit 必须包含数字
	RequireDigit bool `json:"requireDigit"`
	// Description 一句话规则说明（前端直接展示）
	Description string `json:"description"`
}

// PasswordPolicyOf 返回当前生效的密码策略。
func PasswordPolicyOf(minLen int) PasswordPolicy {
	if minLen < 8 {
		minLen = 8
	}
	return PasswordPolicy{
		MinLength:    minLen,
		RequireUpper: true,
		RequireLower: true,
		RequireDigit: true,
		Description: fmt.Sprintf(
			"至少 %d 位，且同时包含大写字母、小写字母和数字；不能使用常见弱口令，也不能出现 3 个以上连续相同字符", minLen),
	}
}

// CheckStrength 校验密码强度。
//
// 规则（与前端提示保持一致）：长度、大写/小写/数字三类都必须有、不在弱口令黑名单、
// 不存在 3 个以上连续相同字符。返回的错误文本会**原样展示给用户**。
func CheckStrength(pwd string, minLen int) error {
	policy := PasswordPolicyOf(minLen)
	runes := []rune(pwd)
	if len(runes) < policy.MinLength {
		return fmt.Errorf("密码长度不能少于 %d 位（当前 %d 位）", policy.MinLength, len(runes))
	}
	if len(runes) > 64 {
		return errors.New("密码长度不能超过 64 位")
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range runes {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		missing := make([]string, 0, 3)
		if !hasUpper {
			missing = append(missing, "大写字母")
		}
		if !hasLower {
			missing = append(missing, "小写字母")
		}
		if !hasDigit {
			missing = append(missing, "数字")
		}
		return fmt.Errorf("密码还需要包含：%s", strings.Join(missing, "、"))
	}

	if weakPasswords[strings.ToLower(pwd)] {
		return errors.New("该密码属于常见弱口令，请更换为更难猜测的密码")
	}

	// 连续相同字符（aaa / 111）直接拒绝
	same := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			same++
			if same >= 3 {
				return errors.New("密码不能包含 3 个及以上连续相同的字符")
			}
		} else {
			same = 1
		}
	}
	return nil
}

// RandomPassword 生成满足复杂度策略（至少含字母与数字）的随机密码。
// 注意：直接用 RandomString 得到的字符串约 1/5 概率不含数字，
// 会让"重置密码"随机失败（10001 参数校验失败），必须走这里。
func RandomPassword(n int) string {
	if n < 8 {
		n = 8
	}
	for i := 0; i < 100; i++ {
		s := RandomString(n)
		if CheckStrength(s, n) == nil {
			return s
		}
	}
	// 极端兜底：保证一定含数字
	base := RandomString(n - 1)
	if base == "" {
		return "Abc12345"
	}
	return base + "7"
}

// Encrypt AES-GCM 加密（key 长度必须为 16/24/32 字节）。
func Encrypt(plain, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt AES-GCM 解密。
func Decrypt(cipherText string, key []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("密文长度不足")
	}
	nonce, data := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, data, nil)
}

// RandomString 生成随机字符串（用于密码盐、临时密码等）。
func RandomString(n int) string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
