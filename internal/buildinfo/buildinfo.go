package buildinfo

// 由 ldflags 注入；本地 `go run .` 时保持默认 dev/unknown
var (
	Version = "dev"
	Commit  = "unknown"
)
