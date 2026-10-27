# 工程验证记录

验证日期：2026-10-27，Asia/Shanghai。本记录涵盖新Go工程基础及上游SDK，不代表旧业务完成迁移。

## 已通过

- 项目工具链 `go version`：go1.27.3 windows/amd64；go.mod、容器和说明统一使用1.27.3。
- `go run ./cmd/devtools check`：gofmt检查、go mod verify、go vet、全量本地测试通过。配置、鉴权、404/405/500、请求体上限、依赖就绪、生命周期和模块重复注册边界均有行为测试。
- 单元测试通过 testutil 清除非测试 HADALBOT_* 环境变量，避免开发终端配置导致测试连接真实依赖。
- `go run ./cmd/devtools lint`：golangci-lint v2.15.0，最终0 issues。
- `go run ./cmd/devtools vuln`：govulncheck v1.9.0，使用Go1.27.3复查后 No vulnerabilities found。初次扫描定位标准库 GO-2026-6809，已升级到[官方修复版本](https://go.dev/doc/devel/release#go1.27.3)后再验证。
- `go run ./cmd/devtools build`：Windows服务和迁移程序构建通过。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build`：服务与迁移程序交叉编译通过，生成对应Linux二进制。
- 编译后的 `hadalbot.exe` 在随机本地端口启动并实测：healthz/readyz为200，未授权system/metrics为401，带X-API-Key时均为200，不存在路径为404。
- 容器使用的 `hadalbot.exe -healthcheck` 命令返回0。
- `go version -m bin/hadalbot.exe` 确认实际可执行文件使用Go1.27.3。
- 开发配置校验通过；探针响应不暴露连接信息。日志仅保留请求ID、方法、注册路由、状态和耗时。
- 基础阶段HTTP关键包曾达到100%语句覆盖；加入回调挂载后以当前coverage.out为准，不把覆盖率等同于真实上游验收。
- 启动测试实例已关闭，未留下新服务后台进程，旧项目未改动或重启。

## 已配置，尚未本地验证

- 千寻/米哈游真实上游：本轮只获取官方公开文档，不连接真实微信，不登录真实账号、不签到、不发送消息、不兑换。部分StarRail App及存在历史冲突的Genshin BBS签名要求显式提供DS2Signer；候选接口默认关闭。绑定UI、所有权及加密存储仍未迁移。

- PostgreSQL/Redis及Goose真实集成：本机未提供独立测试服务，也没有Docker命令，因此2项集成测试明确跳过；已在CI中配置独立PostgreSQL18/Redis7及真实集成测试。未连接旧项目数据库或Redis。
- Docker镜像构建、Compose解析和容器实际运行：本机没有Docker，尚未执行；Linux二进制交叉编译通过不等于容器运行已通过。
- `go test -race`：本机CGO禁用且未找到C编译器，命令提示race requires cgo，未把它记录为通过。CI Linux任务已配置竞态检查。
- GitHub Actions：仅创建工作流配置，尚未推送或触发远程CI。
- Air热重载和VS Code调试：已配置固定版本和入口，未在本轮启动长期监听或调试会话。

## 本轮上游SDK专项验证

- 共享网关：来源/重定向限制、限流排队与取消、关闭取消在途、响应字节限制、流式下载、只读重试和写请求不重试。
- 千寻：99项契约逐项发往httptest服务，检查模式/路径/查询/正文，并解码官方响应；13类框架及9类直连事件示例、3个原文勘误、空WXID、长消息ID、未知字段、回调鉴权、消息顺序/去重、队列和保留字节上限均有测试。
- WebSocket：本机临时服务器验证乱序响应关联、事件入队、断线后pending释放及不重放写请求，不代表真实框架版本验收。
- 米哈游：179项命名契约逐项发往httptest服务；验证精确签名原文、GET兑换不重试、账号间授权隔离、RSA登录流程、票据交换、UID不一致拒绝、Cookie绑定三游戏角色校验、挑战交还及敏感错误脱敏。
- 全量check、lint（0 issues）与vuln（No vulnerabilities found）通过；本轮语句覆盖率gateway69.7%、mihoyo77.9%、qianxun82.8%、httpserver96.6%。命令入口与外部真实服务未用覆盖率冒充验证。
- Windows及Linux amd64服务/迁移程序构建成功。Windows新二进制在随机回环端口验证：healthz/readyz/system/upstreams/metrics为200；未鉴权upstreams/metrics为401；旧千寻代理、旧wxid与默认关闭回调为404。目录返回99/179且两个上游均disabled；日志无测试密钥，healthcheck退出0，临时进程已停止。
- 单条消息解码基准：Go1.27.3/windows amd64、Xeon E5-2450v2@2.50GHz，200ms采样，17726ns/op、2197B/op、31allocs/op。仅表示本机解码成本，不是网络吞吐、全服务内存占用或与旧项目的对比。

## 插件系统专项验证

- Go生命周期测试通过：依赖排序/版本/循环拒绝、失败实例清理和逆序回滚、实例重建、观察先于拦截、panic脱敏、能力深拷贝、执行满载背压、排空超时隔离、作用域任务及逆序清理。
- 实际Node24.14.0 worker测试通过：旧example/index.ts源码原样运行并回复pong、不同机器人上下文关联、TS枚举转换与核心别名、未授权能力拒绝、死循环超时、崩溃与非法超大stdout回收、未知旧模块明确失败、临时目录删除。
- Go宿主集成测试通过：默认不扫描/不初始化、Go示例经真实httptest HTTP服务发送请求、事件机器人透传、管理API鉴权及Origin拒绝、旧清单字段保留、入口逃逸拒绝、嵌套YAML授权和插件配置解析。
- 全量check通过；最终lint为0 issues，依赖变更后的vuln为No vulnerabilities found。新增大数据预算分支后又执行插件专项测试与lint，均通过。
- TypeScript5.9.3对独立SDK声明与echo-ts示例进行strict/noEmit检查通过；仅使用旧目录中的编译器读取新源码，没有运行旧机器人业务。CI已配置Node24和固定版本TS类型检查，未宣称远程CI已经执行。
- Windows服务/迁移程序构建、Linux amd64交叉构建通过；新插件配置样例的二进制check-config通过。
- 当前原生单插件事件分发基准：Go1.27.3/windows amd64、Xeon E5-2450v2@2.50GHz、300ms采样，3425ns/op、680B/op、10allocs/op。它不代表消息解析、网络吞吐、TS IPC或全服务RSS，不与旧项目做无依据性能比较。
- 只读静态审计覆盖22个旧插件，报告列出未适配模块；除原始example契约验收外，不把全部旧业务标为已经迁移。未连接真实千寻或真实账号。
- 附加“启动新二进制并调用插件管理接口”的PowerShell联调命令被自动审批拦截，仅返回blocked by policy，没有具体原因；命令未执行，不记为通过。已有Go宿主集成测试和真实worker测试作为对应层级的验证证据。
- 本机竞态、Docker和真实数据库验证边界沿用前文，未把这些未执行项目改为通过。

## 构建产物位置（更新）

`bin/hadalbot.exe`、`bin/migrate.exe`、`bin/hadalbot-linux-amd64`、`bin/migrate-linux-amd64`。目录被Git忽略，可随时通过构建命令重新生成；Go主服务不包含旧项目资源包、Node或Python运行时。

后续业务迁移仍需契约、算法、数据和真实上游验收；目前注册upstreams及plugins基础设施模块，正式游戏业务尚未完成迁移。纯Go默认部署不需要Node，TS兼容启用时依赖外部Node运行时。
