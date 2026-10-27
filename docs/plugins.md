# 插件系统开发与运维

已实现可运行的插件内核、Go编译插件、TS兼容worker、组合根、管理接口和行为测试。新业务继续以Go为主；TS是本轮用户明确要求的兼容运行时，默认关闭。没有启动旧机器人、复制旧环境或连接旧数据库。

## 架构

| 层 | 职责 |
|---|---|
| pkg/plugin | Go SDK、清单、依赖图、生命周期、资源作用域、能力检查和消息管线 |
| pkg/plugin/process | 独立Node worker、有界双向RPC、TS加载器、故障回收及旧钩子适配 |
| internal/pluginhost | 发现、工厂登记、操作员授权、上游适配、管理HTTP与事件订阅 |
| internal/plugins | 编译进宿主的Go插件，目前仅有默认不启用的示例 |

Go插件采用普通包和Factory注册，不使用Windows不支持的Go.so。重新启用会创建新实例；更新Go代码需要重新构建，更新TS代码后可停用再启用。没有默认文件监听器或后台编译器。依据：[Go官方plugin限制](https://pkg.go.dev/plugin)、[Node官方TS机制](https://nodejs.org/api/typescript.html)。

纯Go部署无需Node；启用TS需Node24以上及插件自己的npm依赖。加载器支持类型擦除、枚举和参数属性、插件内部.js引用回退.ts；不执行tsc类型检查，不宣称支持所有装饰器和前端构建语法。复杂插件可自行编译成JS入口。启动时不会自动npm install。

sdks/typescript/index.d.ts提供独立中文类型契约，echo-ts示例使用该声明并通过strict类型检查；此SDK不依赖旧项目的TS类型目录。

TS SDK检查命令为 `npm exec --yes --package=typescript@5.9.3 -- tsc --project sdks/typescript/tsconfig.json --noEmit`，仅供开发/CI，不在服务启动阶段下载编译器。Go本身的构建仍使用cmd/devtools。

## 生命周期

状态流为registered → loading → initialized → ready → draining → stopped；未启用为disabled，失败为failed。Init初始化资源；Ready表示宿主依赖已装配，**不承诺微信已登录或WS已连接**。依赖旧“微信连上才onReady”时序的业务应处理相应连接事件。

依赖在运行中失效时，使用者显示派生状态blocked并暂停分发，保留实例以便按依赖顺序清理。

启动前冻结注册表，校验ID、完整语义版本、宿主API版本、依赖及循环关系。依赖先启动；无依赖时按priority降序、ID升序。约束明确支持*、精确版本、>=、^、~，不接受未实现的npm复杂表达式。

启用依次执行Init、Ready，失败实例按单阶段生命周期预算清理，再逆序回滚本次已启动实例。默认全体启动预算30秒，已启动实例的回滚另有30秒总预算。停用先拒绝新调用并排空在途调用，再取消/等待受管任务、执行Dispose、逆序清理资源。被就绪插件依赖的插件不能直接停用，需先处理使用者。

旧原生调用超时仍未退出时保留failed实例，不创建新实例与其重叠。Go不能强杀忽略context的函数，因此不用“超时后再开goroutine”伪造退出；插件必须遵守取消契约。TS超时可杀死worker并回收进程配额，不自动重放消息或业务写操作。

Init应主要准备资源；宿主不能事务性撤销已经发出的消息等外部副作用。Dispose和资源清理应幂等，便于继续处理之前超时的实例。启动后不修改工厂注册表，新增工厂、权限和持久配置在重启时生效。

## Go插件开发

实现Manifest()，按需实现Initializer、Readier、Observer、EventHandler、MessageHandler、Disposer接口，无需空钩子。完整示例见internal/plugins/example/example.go：收到go ping，通过授权qx.sendText回复go pong。

```go
meta := myplugin.Plugin{}.Manifest()
err := manager.Register(meta, func() plugin.Plugin {
    return &myplugin.Plugin{} // 每次启用创建新实例。
}, json.RawMessage(`{"feature":true}`), true)
```

此片段位于组合根，调用方需检查err。当前工厂在internal/pluginhost.New登记。Context.Config返回副本；Context.Call逐次核对清单能力、宿主策略及方法白名单。通过plugin.AccountWXID(ctx)取得事件机器人，不使用共享可变当前账号。

后台任务用Scope.Go，周期任务用Scope.Every，资源清理用Scope.Defer。定时器每个只占一个受管任务，慢任务不积累tick。任务panic计入task_failures，钩子panic计入failures；不输出panic原值。自行创建的goroutine/监听器无法被宿主自动发现，必须由Dispose关闭。

Standalone SDK的Register是可信宿主授权入口；工程组合根把清单能力与操作员grants相交。Caller/Policy由领域服务注入，不跨模块读取私有表。不要记录消息正文、Cookie、Token或完整上游响应。

## 分发与并发

私聊/群聊转换为旧ParsedMessage对应的Message，保留实际发送人、群、机器人、@列表、原文及字符串消息ID，不经float64。先运行全部观察者，再onEvent拦截链，最后onMessage拦截链。观察者错误不阻断其他观察者，但返回给总线统计；拦截钩子返回true或报错会结束本次链，避免部分副作用后继续执行其他命令。

每插件默认执行1、等待0，满载返回ErrBusy，不创建无限等待goroutine。钩子执行不持全局锁。提高并发后插件自行保护状态；会话顺序由千寻EventBus分片负责。Policy提供AllowEvent及AllowCall，供群总开关、群插件开关及所有权服务接入；当前没有伪造旧Root身份或旧权限数据库。

## TS兼容范围

| 旧契约 | 当前支持 |
|---|---|
| plugin.json | 保留id/name/version/description/features/commands/configSchema/renderTemplates；省略runtime按typescript发现 |
| default或具名plugin导出 | 对象、已实例化PluginBase子类；校验name与清单ID一致 |
| init/onReady/observeMessage/onEvent/onMessage/dispose | 生命周期及顺序适配；未实现的钩子不发IPC |
| ctx.api/sendText/api.request | Go宿主能力调用；request按实际操作授权，不提供绕过白名单的通用代理 |
| ctx.apis.get/getDynamic/facade/call/callDynamic | 代理已适配服务；工程默认提供qx JSON RPC，其他服务由Go领域适配器注册 |
| ctx.botWxid | AsyncLocalStorage关联当前钩子；Go反向调用再核对父请求的账号和期限 |
| ctx.log | 带插件ID的结构化日志，单消息最多2048字节，任意meta不转发；console不混入RPC |
| 同步ctx.perms查询 | 可由授权permissions.snapshot适配器提供每钩子快照；缺失时明确报错，不能以Promise冒充布尔值 |
| 核心别名 | PluginBase、QxEventId、QxApiType、parseMessageEvent、api/types/index及本插件@/plugins/<id>/ |
| 旧Prisma、文档仓储、浏览器渲染、Provider等内部模块 | 不自动导入旧核心或返回假数据，需对应Go适配器或业务迁移 |

权限快照字段为rootUsers、superUsers、groupDelegates（群到用户数组）、disabledGroups；支持getLevel/isRoot/isSuper/isSuperOnly/isDelegate/check/hasSpecialPermission/isGroupBotEnabled。工程默认没有该旧权限适配器。权限修改、旧API Hub注册/注销、Buffer下载及任意多参数服务未伪装实现；千寻Go SDK的流式下载可直接调用。

反向调用必须关联仍存活的钩子，继承其账号与取消期限。旧setInterval脱离钩子的ctx.api调用会被拒绝，不退回默认机器人。此类后台逻辑应移植为Go Scope任务，或单独设计任务身份适配器。直接创建的Node计时器/网络由插件Dispose管理，进程退出负责最终回收。

旧example/index.ts未修改源码的真实Node测试已验证pong及不同机器人上下文，副本在process/testdata/legacy。**不代表22个旧业务插件已全部迁移。** [静态审计](plugin-compatibility.json)列出未适配引用；它不证明npm依赖、动态导入、持久化和业务行为兼容。

```powershell
go run ./cmd/plugincheck -root ../HadalBot/src/plugins -out docs/plugin-compatibility.json
# -strict会在发现未适配引用或跳过源码时返回非零。
```

## 配置与预算

默认plugins.enabled=false、typescript=false、enabled_ids为空：不扫描目录、不创建Manager/Node。TS发现仅读取一级目录和有限清单，不加载全部资源。目录存在不代表有执行许可，grants默认空。完整示例：configs/plugins.example.yaml、examples/plugins/echo-ts。

```powershell
go run ./cmd/hadalbot -check-config -config configs/plugins.example.yaml
go run ./cmd/hadalbot -config configs/plugins.example.yaml
```

真实收消息仍需按upstreams.md启用千寻回调/WS并注入密钥。示例本身不会连接旧账号。

| 预算 | 默认 |
|---|---|
| 注册插件总数 | 128 |
| 存活TS worker | 4，共享配额 |
| 每worker V8堆 | 128MiB；不是RSS硬上限 |
| 单插件执行/等待 | 1 / 0 |
| 钩子/单阶段生命周期 | 5秒 / 10秒 |
| 后台任务/清理函数 | 8 / 64 |
| RPC单帧/待响应/反向并发 | 1MiB / 32 / 8 |
| 清单/配置 | 各256KiB |

异常stdout、过大帧、崩溃和超时都会失败，不积累无限日志。Node不继承HADALBOT_*、数据库密钥、NODE_OPTIONS等宿主环境，仅保留必要系统路径及临时目录变量。默认Docker镜像保持纯Go，TS需另行准备Node环境。

进程隔离和能力网关不是恶意代码沙箱：Go同进程，TS仍可用Node文件/网络API和创建子进程。可信插件应清理自己的子资源；不可信代码需要额外操作系统隔离，本系统没有宣称提供容器/作业对象/网络沙箱。

## 管理与更新

GET /api/v1/plugins仅显示安全状态与计数；POST /api/v1/plugins/{id}/enable和disable复用管理API key，拒绝浏览器Origin，同时最多两个控制请求。不能通过HTTP上传代码或指定任意入口。

停用再启用即可重建实例，依赖使用者需先停用，不隐式级联。HTTP操作只改变当前进程状态，不改写YAML；持久启用状态由enabled_ids管理。验证证据及真实服务/竞态检查界限见verification.md。
