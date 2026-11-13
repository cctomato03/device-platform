// 变更审计辅助：更新前抓旧值 → 更新 → 计算字段差异 → 交给 audit 包落库。
package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"

	"device-platform/internal/audit"
)

// Snapshot 把 GORM 结构体实例转成「列名 → 值」的 map，用于前后对比。
// 只取 `gorm:"column:xxx"` 标签里的列名，嵌入字段（Base/BaseLite）自动展开。
func Snapshot(row any) map[string]any {
	if row == nil {
		return nil
	}
	v := reflect.ValueOf(row)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[string]any, 24)
	collectColumns(v, out)
	return out
}

// collectColumns 递归收集字段（处理匿名嵌入结构体）。
func collectColumns(v reflect.Value, out map[string]any) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // 未导出字段
			continue
		}
		fv := v.Field(i)
		if f.Anonymous {
			collectColumns(fv, out)
			continue
		}
		tag := f.Tag.Get("gorm")
		if tag == "" || tag == "-" {
			continue
		}
		col := ""
		for _, part := range strings.Split(tag, ";") {
			if strings.HasPrefix(part, "column:") {
				col = strings.TrimPrefix(part, "column:")
				break
			}
		}
		if col == "" || col == "-" {
			continue
		}
		out[col] = fv.Interface()
	}
}

// LoadSnapshot 读取一行数据并转成列快照；记录不存在返回 (nil, nil)。
func LoadSnapshot(ctx context.Context, db *gorm.DB, model any, id uint64) (map[string]any, error) {
	ptr := reflect.New(reflect.TypeOf(model).Elem()).Interface()
	err := db.WithContext(ctx).Model(model).Where("id = ?", id).Limit(1).Find(ptr).Error
	if err != nil {
		return nil, err
	}
	snap := Snapshot(ptr)
	// Find 未命中时是零值结构体：用 id 列判断是否存在，避免把全零快照当旧值。
	if v, ok := snap["id"]; !ok || isEmptyID(v) {
		return nil, nil
	}
	return snap, nil
}

func isEmptyID(v any) bool {
	switch n := v.(type) {
	case uint64:
		return n == 0
	case uint:
		return n == 0
	case int64:
		return n == 0
	case int:
		return n == 0
	}
	return false
}

// UpdateByID 带审计差异的更新：
//  1. 先按 id 取旧值快照；
//  2. 执行更新（fields 为「列名 → 新值」，需自行带上 update_by/update_time）；
//  3. 计算差异写入 audit collector（无收集器时自动忽略）。
//
// 影响行数为 0 时返回 gorm.ErrRecordNotFound（与 Result 语义一致，由 errs.Normalize 转 404）。
func UpdateByID(ctx context.Context, db *gorm.DB, model any, id uint64, fields map[string]any) error {
	before, err := LoadSnapshot(ctx, db, model, id)
	if err != nil {
		return err
	}
	if before == nil {
		return gorm.ErrRecordNotFound
	}
	if err := Result(db.WithContext(ctx).Model(model).Where("id = ?", id).Updates(fields)); err != nil {
		return err
	}
	if changes := audit.Diff(before, fields); len(changes) > 0 {
		audit.Note(ctx, changes)
	}
	return nil
}

// IsNotFound 判断是否为「记录不存在」（None 影响行数 / gorm.ErrRecordNotFound）。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// nowPtr 便于业务层复用。
func nowPtr() *time.Time {
	n := time.Now()
	return &n
}
