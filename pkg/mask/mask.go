// Package mask 敏感信息脱敏。
package mask

import "strings"

// Mobile 手机号脱敏：138****8888
func Mobile(s string) string { return keep(s, 3, 4) }

// IDCard 身份证脱敏：110***********1234
func IDCard(s string) string { return keep(s, 3, 4) }

// Email 邮箱脱敏：ab***@example.com
func Email(s string) string {
	at := strings.IndexByte(s, '@')
	if at <= 0 {
		return s
	}
	name, domain := s[:at], s[at:]
	if len(name) <= 2 {
		return "*" + domain
	}
	return name[:2] + strings.Repeat("*", 3) + domain
}

// Name 姓名脱敏：张*
func Name(s string) string {
	r := []rune(s)
	if len(r) <= 1 {
		return s
	}
	first := string(r[0])
	star := strings.Repeat("*", len(r)-1)
	if len(r) > 2 { // 复姓等保留首字
		return first + star
	}
	return first + "*"
}

// BankCard 银行卡脱敏：**** **** **** 1234
func BankCard(s string) string { return keep(s, 0, 4) }

// keep 保留前 front 位与后 back 位，中间用 * 填充。
func keep(s string, front, back int) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= front+back {
		return s
	}
	return string(r[:front]) + strings.Repeat("*", len(r)-front-back) + string(r[len(r)-back:])
}
