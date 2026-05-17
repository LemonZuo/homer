package buildinfo

// 由 ldflags 注入；本地 `go run .` 时保持默认值。
// Commit 为短 git hash；BuildID 为每次构建唯一的短 hash（区分同 commit 的多次构建）。
var (
	Version = "dev"
	Commit  = "unknown"
	BuildID = "none"
)
