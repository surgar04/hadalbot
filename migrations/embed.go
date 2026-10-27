// Package migrations 把版本化 SQL 嵌入迁移二进制，部署时不依赖工作目录。
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
