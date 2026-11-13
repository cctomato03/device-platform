package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 本文件：配置文件写入的公共辅助（备份 + 原子写）。
//
// 设备上「写错一个配置文件就可能失联」，因此所有系统配置文件写入都走这里：
//  1. 先备份原文件到 <home>/backup/<name>/ 下（带时间戳）；
//  2. 用「临时文件 + rename」原子替换，避免写一半被中断；
//  3. 写入前统一设置权限，含密钥的配置用 0600。

// fileBackup 备份文件，返回备份路径。
func fileBackup(src, backupDir string) string {
	body, err := os.ReadFile(src)
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return ""
	}
	name := filepath.Base(src) + "." + time.Now().Format("20060102-150405")
	dst := filepath.Join(backupDir, name)
	if err := os.WriteFile(dst, body, 0o600); err != nil {
		return ""
	}
	return dst
}

// writeFileAtomic 原子写入文件（自动建目录）。
func writeFileAtomic(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// readFileIfExists 读取文件，不存在返回空串。
func readFileIfExists(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

// splitCSV 拆分逗号分隔列表并去空。
func splitCSV(v string) []string {
	out := []string{}
	for _, item := range strings.Split(v, ",") {
		if s := strings.TrimSpace(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// joinCSV 合并列表为逗号分隔字符串。
func joinCSV(items []string) string {
	clean := make([]string, 0, len(items))
	for _, it := range items {
		if s := strings.TrimSpace(it); s != "" {
			clean = append(clean, s)
		}
	}
	return strings.Join(clean, ",")
}

// mustResultFail 生成一条失败结果。
func mustResultFail(item string, err error) ApplyResult {
	msg := "未知错误"
	if err != nil {
		msg = err.Error()
	}
	return ApplyResult{Item: item, OK: false, Message: fmt.Sprintf("%s", msg)}
}
