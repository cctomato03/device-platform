// Package jsonx 提供宽松的 JSON 数字类型：
// 雪花 ID 在 API 中以字符串序列化（避免 JS 精度丢失），但客户端也可能传数字，
// 这里统一兼容 `"123456789"` 与 `123456789` 两种写法。
package jsonx

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Uint64 兼容字符串/数字的 uint64。
type Uint64 uint64

// UnmarshalJSON 实现 json.Unmarshaler。
func (u *Uint64) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*u = 0
		return nil
	}
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		*u = 0
		return nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*u = Uint64(v)
	return nil
}

// MarshalJSON 以字符串输出，保持与前端 ID 精度一致。
func (u Uint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(u), 10))
}

// Uint64 转换为原生类型。
func (u Uint64) Uint64() uint64 { return uint64(u) }

// Uint64Slice 兼容 ["1","2"]、[1,2] 以及单个值 "1" / 1 的 uint64 切片。
type Uint64Slice []uint64

// UnmarshalJSON 实现 json.Unmarshaler。
func (s *Uint64Slice) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*s = nil
		return nil
	}
	if b[0] != '[' {
		// 允许传单个值
		var one Uint64
		if err := one.UnmarshalJSON(b); err != nil {
			return err
		}
		*s = []uint64{uint64(one)}
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make([]uint64, 0, len(raw))
	for _, item := range raw {
		var v Uint64
		if err := v.UnmarshalJSON(item); err != nil {
			return err
		}
		out = append(out, uint64(v))
	}
	*s = out
	return nil
}

// MarshalJSON 以字符串数组输出。
func (s Uint64Slice) MarshalJSON() ([]byte, error) {
	out := make([]string, 0, len(s))
	for _, v := range s {
		out = append(out, strconv.FormatUint(v, 10))
	}
	return json.Marshal(out)
}

// ToUint64 转换为原生切片（空安全）。
func (s Uint64Slice) ToUint64() []uint64 {
	if s == nil {
		return nil
	}
	return []uint64(s)
}
