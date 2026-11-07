package crypto

import "testing"

// TestRandomPassword 保证随机密码永远满足复杂度策略（历史 bug：约 1/5 概率不含数字导致重置密码 10001）。
func TestRandomPassword(t *testing.T) {
	for i := 0; i < 2000; i++ {
		pwd := RandomPassword(12)
		if len(pwd) != 12 {
			t.Fatalf("长度不符: %q", pwd)
		}
		if err := CheckStrength(pwd, 12); err != nil {
			t.Fatalf("随机密码不满足策略: %q err=%v", pwd, err)
		}
	}
	for i := 0; i < 200; i++ {
		if err := CheckStrength(RandomPassword(4), 8); err != nil {
			t.Fatalf("短长度入参应被抬到 8 位: err=%v", err)
		}
	}
}

func TestCheckStrength(t *testing.T) {
	cases := []struct {
		pwd  string
		min  int
		want bool
	}{
		{"Abc12345x", 8, true},
		{"Aa1!aaaa", 8, false}, // 连续相同字符
		{"abcdefgh", 8, false}, // 缺大写与数字
		{"abcdefgH", 8, false}, // 缺数字
		{"abcd1234", 8, false}, // 缺大写
		{"Abc12345", 8, false}, // 弱口令黑名单
		{"12345678", 8, false},
		{"Ab1", 8, false},
	}
	for _, c := range cases {
		if got := CheckStrength(c.pwd, c.min) == nil; got != c.want {
			t.Fatalf("CheckStrength(%q,%d)=%v want %v", c.pwd, c.min, got, c.want)
		}
	}
}
