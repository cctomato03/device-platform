// Package audit 变更审计：在请求上下文中收集"字段级前后差异"，由操作日志中间件落库。
//
// 用法（service 层）：
//
//	changes := audit.Diff(oldMap, newMap)   // 计算差异
//	audit.Note(ctx, changes)                // 挂到当前请求（无 collector 时自动忽略）
//
// 中间件在 handler 结束读取 audit.Changes(ctx) 写入 sys_oper_log.diff，
// 前端"操作日志"详情里以「字段 / 修改前 / 修改后」表格展示。
package audit

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// maxChanges 单次操作最多记录多少条字段差异，避免大表单把日志撑爆。
const maxChanges = 80

// Change 一条字段级变更。
type Change struct {
	Field  string `json:"field"`  // 列名（业务友好名由前端映射）
	Before string `json:"before"` // 修改前
	After  string `json:"after"`  // 修改后
}

type collectorKey struct{}

// Collector 一次请求内的变更收集器（并发安全）。
type Collector struct {
	mu      sync.Mutex
	changes []Change
}

// Note 向当前请求的收集器追加变更；ctx 中没有收集器时静默丢弃。
func Note(ctx context.Context, changes []Change) {
	c := FromContext(ctx)
	if c == nil || len(changes) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	remain := maxChanges - len(c.changes)
	if remain <= 0 {
		return
	}
	if len(changes) > remain {
		changes = changes[:remain]
	}
	c.changes = append(c.changes, changes...)
}

// FromContext 取出收集器（可能为 nil）。
func FromContext(ctx context.Context) *Collector {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(collectorKey{}).(*Collector); ok {
		return c
	}
	return nil
}

// NewContext 把一个新的收集器挂到上下文上（由中间件调用）。
func NewContext(ctx context.Context) (context.Context, *Collector) {
	c := &Collector{}
	return context.WithValue(ctx, collectorKey{}, c), c
}

// Changes 取出已收集的变更（拷贝）。
func (c *Collector) Changes() []Change {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Change, len(c.changes))
	copy(out, c.changes)
	return out
}

// ChangesFromContext 便捷取数。
func ChangesFromContext(ctx context.Context) []Change {
	c := FromContext(ctx)
	return c.Changes()
}

// sensitiveFields 这些字段的值不写进差异（只标记为已修改）。
var sensitiveFields = map[string]bool{
	"password":      true,
	"pwd":           true,
	"secret":        true,
	"access_key":    true,
	"secret_key":    true,
	"token":         true,
	"refresh_token": true,
}

// Diff 计算两个"列名 → 值"快照的差异，返回字段级变更列表。
// before 为空表示新增（不产生差异）；after 中缺失的键不参与比较。
func Diff(before, after map[string]any) []Change {
	if len(before) == 0 || len(after) == 0 {
		return nil
	}
	keys := make([]string, 0, len(after))
	for k := range after {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	changes := make([]Change, 0, len(keys))
	for _, k := range keys {
		oldV, ok := before[k]
		if !ok {
			continue // 新字段（不在旧快照里）不比较，避免误报
		}
		oldS := stringify(oldV)
		newS := stringify(after[k])
		if oldS == newS {
			continue
		}
		if sensitiveFields[k] {
			changes = append(changes, Change{Field: k, Before: "******", After: "******"})
			continue
		}
		changes = append(changes, Change{Field: k, Before: truncateValue(oldS), After: truncateValue(newS)})
	}
	return changes
}

// stringify 值 → 字符串（时间、指针、数字统一处理，便于比较）。
//
// 坑：*time.Time 为 nil 时断言成 fmt.Stringer 会得到"非 nil 接口 + nil 指针"，
// 直接调 String() 会 panic（Go 的方法包装器解引用 nil）。所以先解指针再比较。
func stringify(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	v = rv.Interface()
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05")
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// truncateValue 单值最多 500 字符，超长截断（大文本字段不至于刷屏）。
func truncateValue(s string) string {
	s = strings.TrimSpace(s)
	const limit = 500
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
