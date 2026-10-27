# 上游 SDK、网关和账号接入

本轮实现千寻接口/事件、米哈游上游请求及账号绑定所需的客户端能力。所有运行时代码使用 Go。旧项目的控制台、绑定网页、数据库服务和业务 HTTP 路由不在本轮封装范围，具体用途见 [旧 HTTP 用途说明](legacy-http-purpose.md)。

## 覆盖范围与依据

| 范围 | 本轮实现 | 依据 |
|---|---|---|
| 千寻框架模式 | 49 项 HTTP 契约，其中48项 RPC、1项文件下载 | 官方 Apifox 当前页面 |
| 千寻微信直连模式 | 50 项 HTTP 契约，其中49项 RPC、1项文件下载 | 官方 Apifox 当前页面 |
| 千寻事件 | 框架13种事件编号、直连9种事件类型，HTTP回调与WebSocket接入 | 官方事件正文及完整示例 |
| 米哈游及相关上游 | 33组、179项命名契约；135项来自原上游规范，44项由旧源码补充 | 本地 OpenAPI、旧插件请求实现 |
| 账号能力 | App/Web密码登录、RSA加密、Aigis结果编码、票据交换、Cookie解析校验、角色绑定验证及续期 | 原账号Provider与绑定服务源码 |

179是命名契约数，包含不同端点族对同一URL的独立命名，不代表179个唯一URL。覆盖清单见 [逐项索引](upstream-coverage.md)；源规范和中文字段描述保存在 `docs/contracts/`，生成代码标注了来源。千寻快照包含125个API/事件/协议页面；原正文保留，不悄悄修正原始示例。勘误见 [千寻文档差异](contracts/qianxun/errata.md)。

千寻的框架/直连客户端独立生成。例如运行微信、列出微信只属于框架模式；修改版本、查询登录状态、设置图片下载只属于直连模式；同名killWeChat的两种参数也不相同。不能用一个无类型map抹平差异。

米哈游规范不完整的响应保留为 `json.RawMessage`，使用 `mihoyo.Decode[T]` 转成业务需要的类型，避免编造响应字段或丢弃上游新增字段。候选接口保留Candidate标记，默认禁止调用，只有显式设置AllowCandidates才可尝试。这里的完整覆盖指已盘点的项目接口与提供的千寻文档，不代表互联网所有米哈游私有接口。

## 共享网关与千寻调用

```go
g, err := gateway.New(gateway.Options{
    Timeout: 30 * time.Second, MaxResponseBytes: 32 << 20, MaxRetries: 1,
    Services: map[string]gateway.Limits{
        "qianxun": {Concurrency: 4, QueueSize: 64, MinInterval: 20 * time.Millisecond,
            Origins: []string{"http://127.0.0.1:7777"}},
        "mihoyo": {Concurrency: 8, QueueSize: 128, MinInterval: 100 * time.Millisecond,
            Origins: mihoyo.Origins()},
    },
})
if err != nil { return err }
defer g.Close()
qx, err := qianxun.NewFramework(g, qianxun.Config{
    BaseURL: "http://127.0.0.1:7777", SafeKey: safeKey, WXID: robotWXID,
})
if err != nil { return err }
reply, err := qx.SendText(ctx, qianxun.FrameworkSendTextRequest{
    WXID: "filehelper", Msg: "你好",
})
// err既可能是传输错误，也可能是千寻BusinessError；reply.Raw保留原始响应。
```

代码片段位于调用方函数内，需自行导入context/time及本模块pkg/gateway、pkg/qianxun、pkg/mihoyo。上例发送操作只作调用说明，测试不会连接真实微信。切换机器人使用单次 `qianxun.WithWXID(otherRobot)`，不要改写共享客户端。直接模式用NewDirect和对应端口；文件下载用DownloadFileTo写入调用方writer，避免整份文件驻留内存。

每个服务的并发、排队和请求间隔独立，等待、限速、网络及重试共用context期限。连接池复用、来源白名单、禁止重定向、解压后响应大小限制已统一处理。默认JSON单次8MiB，不能扩大网关总预算；文件使用流式写出，失败可能留下部分内容，调用方应写临时文件并在成功后重命名。HTTPS_PROXY遵守Go标准环境配置。

只有明确只读的契约才允许自动重试；签到、兑换码（即使采用GET）、发消息、登录等均不自动重放。只读请求遇到429/502/503/504或传输错误按预算重试，并尊重Retry-After；超过剩余期限直接交还错误。队列满返回ErrBusy。状态快照提供运行/排队/失败/重试计数，不带Cookie、Token或请求URL。

## 米哈游账号隔离与绑定

```go
client, err := mihoyo.New(g, mihoyo.Options{})
if err != nil { return err }
account := client.ForAccount(mihoyo.Credentials{
    Cookie: mihoyo.Secret(cookie), DeviceID: deviceID, DeviceFP: deviceFP,
})
roles, err := account.HoyoverseGameRoles.GetUserGameRolesByCookie(ctx,
    mihoyo.HoyoverseGameRolesGetUserGameRolesByCookieRequest{GameBiz: "hk4e_global"})
// roles.Data为保留原文的JSON；不要将roles或原始Cookie写到日志。
```

`LoginApp`执行明确的单次密码登录，获得SToken后顺序交换CookieToken/LToken；`LoginWeb`检查本次响应的会话材料。SessionComplete=false不能当作绑定成功，不混入旧Cookie来伪造完整会话。Device须由调用方为该账号维护，不用全局固定设备指纹。密码原样加密，不TrimSpace。

`BindCookie(ctx, Credentials)`负责解析、检查UID一致性并请求上游校验，返回账号与游戏角色；`RefreshCredentials`返回新的凭证副本，失败不覆盖旧值。登录与票据交换逐项检查账号身份，ForAccount共享传输但隔离授权上下文，不使用全局CookieJar。第三方公开数据域不携带米哈游Cookie/授权头。

本轮没有实现微信所有者绑定表、数据库加密保存、绑定网页/二维码UI、后台续期任务、通行证Provider管理和旧绑定HTTP协议。这些属于下一阶段账号应用服务，应在用户身份及所有权校验后调用SDK，只有完整成功才提交存储。HoYoLAB账号凭证也不能视为游戏客户端登录票据。

SDK提供DS1/DS2算法并使用确定的协议salt。StarRail App DS2需要调用方注入已确认有效的DS2Signer或本次DS请求头；没有依据时返回错误。原Genshin BBS规范的DS1说明和参考实现的Gen2/DS2存在冲突，同样要求显式签名，不拿另一端点salt猜测。生成DS必须使用SigningInput提供的实际RawQuery和Body；签名后不重新排序。旧端点的salt和协议仍需真实账号验收，不能由离线测试证明有效。

遇到上游要求Aigis/二次验证时返回ChallengeError，由应用把挑战交给官方验证流程。AigisSolution只编码已有验证结果；不循环提交密码或猜测验证码。HTTP状态、JSON格式、业务retcode和挑战分开处理；错误字符串不带上游正文，完整数据只供受控业务处理。Secret提供脱敏格式化和JSON输出，但显式Reveal及原始响应仍需调用方严格管理。

## 事件投递与资源预算

`DecodeEvent`保留Raw、Payload和未知字段。Scalar兼容数字/字符串且不经过float64，长消息ID不失真。框架accountChange采用直接data对象；注入成功允许空WXID。MessageData等结构保留原协议拼写。新事件Known=false，可订阅EventAny按原文处理。

`NewEventBus`创建固定分片worker；Subscribe(eventID, handler)返回取消订阅函数。相同账号、相同会话进入同一分片，按成功入队顺序处理；不同会话可并行。它不保证所有账号状态事件与聊天事件的全局先后。默认4分片，每片16排队位、每处理器5秒协作超时，原文和载荷保留预算合计32MiB（含处理中事件）。处理器必须遵守context，Go无法强行终止忽略取消的函数。

消息去重窗口默认4096项/5分钟。去重表示进程内接收去重，处理器报错不自动重放；订单/转账等业务仍需自行持久化幂等键。队列不持久化，进程崩溃会丢失已接收的内存事件。可靠交付需要后续显式增加持久化收件箱；目前不宣称exactly-once。

HTTP回调使用独立至少32字符token，优先 `X-QX-Callback-Token`，兼容千寻只能配置URL时的 `?token=...`；日志不记查询串。默认单请求512KiB、最多16个并行解析，队列/字节/解析预算满返回503及Retry-After。200仅代表已入队或窗口内重复，不等于业务写库成功。未知事件也可由EventAny订阅接收。回调开启前应先注册实际业务处理器；工程当前只有记录安全元数据的传输层订阅。

WebSocket.Run只由明确开启的配置启动，单读取循环关联并发请求ID，默认最多32个待响应请求，支持乱序响应。断线结束在途请求，不自动重发写操作；重连退避最大5秒。事件无法入队时明确断开并报告错误，不静默丢弃，但官方协议没有持久化ACK，重连不能保证补齐事件。HTTP文件下载仍使用共享网关。Close负责取消、等待并按context排空事件。

## 工程集成和维护

组合根 `internal/integrations` 统一装配客户端。development配置中的qianxun.enabled、mihoyo.enabled、callback_enabled、websocket_enabled默认false，关闭时不创建上游连接池或事件worker。密钥用环境注入，见 `.env.example`。普通HTTP API key与回调token相互独立。`GET /api/v1/upstreams`只显示已启用状态、契约数量和资源统计，不提供任意请求代理。

契约生成器是Go工具，生成文件不要手工修改：

```powershell
go run ./cmd/contractgen generate-qx docs/contracts/qianxun/official.json pkg/qianxun
go run ./cmd/contractgen supplement-mihoyo docs/contracts/mihoyo
go run ./cmd/contractgen generate-mihoyo docs/contracts/mihoyo pkg/mihoyo
go run ./cmd/devtools fmt
go run ./cmd/devtools check
```

上游更新时应先获取官方快照，再审查契约和生成diff；不要把示例中的账号、票据当成运行配置。fetch-qx可依据解码后的官方目录抓取，输入/中间HTML放入被忽略的tmp目录。逐项HTTP契约、事件示例、账号身份边界、签名原文和并发背压都有测试；真实账号、真实微信及Docker/竞态检查的验证界限见 [验证记录](verification.md)。
