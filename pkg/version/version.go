// Package version 统一存放编译期注入的版本信息，避免各包互相引用。
package version

// 以下变量由 -ldflags 注入（见 Makefile / scripts/build.sh）。
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// Set 运行时注入版本信息（main 里调用一次）。
func Set(v, commit, built string) {
	if v != "" {
		Version = v
	}
	if commit != "" {
		Commit = commit
	}
	if built != "" {
		BuildTime = built
	}
}
