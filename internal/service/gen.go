package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"device-platform/internal/generator"
	"device-platform/pkg/errs"
)

// currentSchema 取当前连接所在库名（代码生成器的数据源）。
//
// SQLite 没有 MySQL 的 DATABASE()，库名固定为 main（PRAGMA database_list）；
// 这里保留 MySQL 兼容分支，便于以后换驱动。
func (s *Service) currentSchema(ctx context.Context) (string, error) {
	sqlDB, err := s.Repo.DB.DB()
	if err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	var name string
	// SQLite：main / temp / attached 三个库，业务表都在 main
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA database_list").Scan(new(int), &name, new(string)); err == nil {
		return defaultStr(name, "main"), nil
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	if name == "" {
		return "", errs.ErrDB.WithErr(fmt.Errorf("无法确定当前数据库名，请检查 DSN 是否指定了 dbname"))
	}
	return name, nil
}

// GenTables 代码生成器：库表清单。
func (s *Service) GenTables(ctx context.Context) ([]generator.TableMeta, error) {
	schema, err := s.currentSchema(ctx)
	if err != nil {
		return nil, err
	}
	sqlDB, err := s.Repo.DB.DB()
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	list, err := generator.LoadTables(ctx, sqlDB, schema)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// GenTableDetail 代码生成器：单表字段详情。
func (s *Service) GenTableDetail(ctx context.Context, table string) (*generator.TableMeta, error) {
	schema, err := s.currentSchema(ctx)
	if err != nil {
		return nil, err
	}
	sqlDB, err := s.Repo.DB.DB()
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	meta, err := generator.LoadTable(ctx, sqlDB, schema, table)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if meta == nil {
		return nil, errs.ErrNotFound
	}
	return meta, nil
}

// GenPreview 代码生成器：预览单表生成结果。
func (s *Service) GenPreview(ctx context.Context, table string) ([]generator.File, error) {
	meta, err := s.GenTableDetail(ctx, table)
	if err != nil {
		return nil, err
	}
	files, err := generator.Generate(meta)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return files, nil
}

// GenDownload 代码生成器：打包下载（zip：按建议路径保留目录结构）。
func (s *Service) GenDownload(ctx context.Context, tables []string) ([]byte, string, error) {
	if len(tables) == 0 {
		return nil, "", errs.InvalidErr(fmt.Errorf("请至少选择一张表"))
	}
	if len(tables) > 30 {
		return nil, "", errs.InvalidErr(fmt.Errorf("单次最多生成 30 张表"))
	}
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, t := range tables {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		files, err := s.GenPreview(ctx, t)
		if err != nil {
			_ = zw.Close()
			return nil, "", err
		}
		for _, f := range files {
			w, err := zw.Create(f.Path)
			if err != nil {
				_ = zw.Close()
				return nil, "", errs.ErrInternal.WithErr(err)
			}
			if _, err := w.Write([]byte(f.Content)); err != nil {
				_ = zw.Close()
				return nil, "", errs.ErrInternal.WithErr(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", errs.ErrInternal.WithErr(err)
	}
	name := fmt.Sprintf("codegen_%s.zip", time.Now().Format("20060102_150405"))
	return buf.Bytes(), name, nil
}
