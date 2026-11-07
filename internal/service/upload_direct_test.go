package service

import (
	"strings"
	"testing"
)

// 对象键规则：年/月/日/[业务]/日期时间-雪花ID.扩展名
func TestBuildObjectKey(t *testing.T) {
	key := buildObjectKey("document", "txt")
	parts := strings.Split(key, "/")
	if len(parts) != 5 {
		t.Fatalf("期望 5 段（年/月/日/业务/文件），实际 %d: %s", len(parts), key)
	}
	if !validObjectKey(key) {
		t.Fatalf("生成的键应通过校验: %s", key)
	}
	if parts[3] != "document" {
		t.Fatalf("业务目录错误: %s", key)
	}
	if !strings.HasSuffix(key, ".txt") {
		t.Fatalf("扩展名丢失: %s", key)
	}

	// 无业务类型时少一层目录
	noBiz := buildObjectKey("", "png")
	if len(strings.Split(noBiz, "/")) != 4 {
		t.Fatalf("无业务类型应为 4 段: %s", noBiz)
	}
}

// validObjectKey 必须拦住越权路径，避免前端直传回执写入任意对象键。
func TestValidObjectKey(t *testing.T) {
	bad := []string{
		"",
		"abc.txt",
		"2026/11/06",
		"2026/13/06/a/b.txt",
		"../etc/passwd",
		"2026/11/06/../secret.txt",
		"2026/11/06/doc/a\\b.txt",
	}
	for _, k := range bad {
		if validObjectKey(k) {
			t.Fatalf("应判定为非法: %s", k)
		}
	}
	if !validObjectKey("2026/11/06/document/20261106-150405-123.txt") {
		t.Fatal("合法对象键被拒")
	}
}

// Shell 任务参数校验：解释器白名单 + 脚本必填。
func TestParseShellParams(t *testing.T) {
	if _, err := parseShellParams(`{"interpreter":"python3","script":"print(1)"}`); err != nil {
		t.Fatalf("合法参数被拒: %v", err)
	}
	if p, err := parseShellParams(`{"script":"echo hi"}`); err != nil || p.Interpreter != "sh" {
		t.Fatalf("缺省解释器应为 sh: %v %+v", err, p)
	}
	for _, raw := range []string{
		"",
		"{}",
		`{"interpreter":"rm","script":"x"}`,
		`{"interpreter":"sh","script":"  "}`,
		"not-json",
	} {
		if _, err := parseShellParams(raw); err == nil {
			t.Fatalf("非法参数应被拒: %s", raw)
		}
	}
}

// 业务类型规整：非法字符替换为短横线，超长拒绝。
func TestNormalizeBizType(t *testing.T) {
	if got, _ := normalizeBizType("a/b c"); got != "a-b-c" {
		t.Fatalf("规整结果错误: %s", got)
	}
	if got, _ := normalizeBizType("///"); got != "default" {
		t.Fatalf("空值应回落 default: %s", got)
	}
	if _, err := normalizeBizType(strings.Repeat("a", 33)); err == nil {
		t.Fatal("超长业务类型应被拒")
	}
}
