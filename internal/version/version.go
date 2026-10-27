package version

// 构建时通过 -ldflags 注入，开发模式也可独立运行。
var Version = "dev"
var Commit = "unknown"
var BuiltAt = "unknown"
