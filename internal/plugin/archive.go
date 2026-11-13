package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 安装包解压上限：防止恶意包把设备磁盘写满。
const (
	maxArchiveFiles = 5000
	maxTotalBytes   = 512 << 20 // 512MB
)

// ExtractArchive 解压插件包到 destDir（自动识别 zip / tar.gz / tgz / tar）。
//
// 安全约束：拒绝绝对路径与 .. 路径穿越，限制文件数量与总大小。
func ExtractArchive(srcPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	lower := strings.ToLower(srcPath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(srcPath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"), strings.HasSuffix(lower, ".tar"):
		return extractTar(srcPath, destDir)
	default:
		return fmt.Errorf("不支持的插件包格式（支持 .zip / .tar.gz / .tgz / .tar）")
	}
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer zr.Close()
	if len(zr.File) > maxArchiveFiles {
		return fmt.Errorf("插件包文件数超过上限（%d）", maxArchiveFiles)
	}
	var total int64
	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		n, err := copyLimited(rc, target, f.Mode(), maxTotalBytes-total)
		rc.Close()
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func extractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(src), ".gz") || strings.HasSuffix(strings.ToLower(src), ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("解压 gzip 失败: %w", err)
		}
		defer gz.Close()
		r = gz
	}

	tr := tar.NewReader(r)
	var total, files int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 失败: %w", err)
		}
		files++
		if files > maxArchiveFiles {
			return fmt.Errorf("插件包文件数超过上限（%d）", maxArchiveFiles)
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode).Perm()
			if mode == 0 {
				mode = 0o644
			}
			n, err := copyLimited(tr, target, mode, maxTotalBytes-total)
			if err != nil {
				return err
			}
			total += n
		case tar.TypeSymlink, tar.TypeLink:
			// 符号链接直接拒绝：避免通过链接读到数据目录之外的文件
			return fmt.Errorf("插件包不允许包含符号链接：%s", hdr.Name)
		}
	}
	return nil
}

// safeJoin 拼接并校验路径，防止 zip slip。
func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
	target := filepath.Join(dest, filepath.FromSlash(clean))
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if targetAbs != destAbs && !strings.HasPrefix(targetAbs, destAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("插件包包含非法路径：%s", name)
	}
	return targetAbs, nil
}

func copyLimited(r io.Reader, target string, mode os.FileMode, limit int64) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("插件包解压后超过大小上限（%d MB）", maxTotalBytes>>20)
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(r, limit))
	if err != nil {
		return n, err
	}
	if n >= limit {
		return n, fmt.Errorf("插件包解压后超过大小上限（%d MB）", maxTotalBytes>>20)
	}
	return n, nil
}
