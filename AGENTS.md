# HadalBot-Go 开发约定

- 该目录是独立 Go 重写工程。新业务、服务、任务、API 客户端及算法使用 Go 实现。本轮用户明确要求向下兼容TS插件，因此允许按需启动独立Node兼容worker；它仅运行显式启用的TS插件，不把新Go业务转回TS，不启动旧机器人或Python，不自动导入旧环境和数据库。
- 构建及质量命令也提供纯 Go 入口 `go run ./cmd/devtools ...`；PowerShell、Makefile 仅作为可选启动包装。
- 模块路径为用户指定仓库 `github.com/surgar04/hadalbot`。发布前核对README、许可证、来源及待上传文件；用户要求README先审核，未经确认不得推送远程。
- 保持模块化单体。组合根为 `internal/app`；业务模块放入 `internal/modules/<领域>`。模块内部按需要拆分 domain/application/adapters，避免没有行为的接口和泛化抽象。
- 业务模块的私有仓储不能被其他模块直接读取。通行证凭证读取由所属 Provider 授权；账号配置、请求日志和公开响应不能包含 Cookie、Token、密钥或完整 DSN。
- 所有外部请求必须使用 context、明确超时和响应体上限。自动重试仅适用于幂等请求，涉及签到、兑换码、发消息等写操作需明确去重语义。
- goroutine、定时任务和队列必须有容量上限、取消路径及退出等待。不得在启动阶段自动加载全部游戏大资源；按需读取并设置缓存预算。
- 新工程默认端口 18878；独立 Compose PostgreSQL/Redis 使用 15432/16379，不连接旧工程数据库、Redis 或复制旧 .env。
- 数据库迁移通过 `cmd/migrate` 显式执行，服务启动不自动改表。业务表和旧数据迁移须先有字段/版本/幂等映射、独立测试和回滚设计。
- 原始 HTTP API、事件字段、SSE 行为、错误响应和命令行为以重构资料及旧源码交叉核对；当前基础工程的 `/api/v1/system` 不代表旧接口已经兼容。
- 旧 HTML 使用 art-template，不能直接当 Go html/template 执行。模板语法、布局、图片、字体、授权脚本与 CSP 均需适配和验证；上游 JS 计算逻辑应完整移植到 Go，不能在运行时嵌入 JS 引擎绕过重写。
- 不复制数 GB 的旧资料到本仓库；参考 `docs/reference-sources.md`。原项目和重构资料保持只读。
- 默认检查：`go run ./cmd/devtools check`、`go run ./cmd/devtools lint`、`go run ./cmd/devtools build`。依赖变更再运行 `go run ./cmd/devtools vuln`，关键并发路径在支持 CGO 的环境运行 `go test -race ./...`。
- 新逻辑需要覆盖真实边界的测试；不为纯文件移动、注释或模拟实现编写镜像测试。外部集成未实测时明确说明，不把跳过测试写成已通过。
- 不未经用户要求自动启动生产业务、连接旧账号或重启旧项目。
