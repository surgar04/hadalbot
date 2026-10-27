<div align="center">

<img src="docs/assets/hadalbot.svg" width="88" height="88" alt="HadalBot 标志" />

# HadalBot

**以 Go 为核心的模块化机器人框架**

统一上游接入 · 可控资源预算 · Go 插件 · TypeScript 兼容

[![Go](https://img.shields.io/badge/Go-1.27.3-00ADD8?logo=go&logoColor=white)](go.mod)
[![License](https://img.shields.io/badge/License-MIT-2563EB)](LICENSE)
[![Stage](https://img.shields.io/badge/Status-Alpha-F59E0B)](#项目状态)
[![Platforms](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-475569)](#构建与检查)

[快速开始](#快速开始) · [核心能力](#核心能力) · [插件开发](#插件开发) · [文档](#文档导航) · [贡献](CONTRIBUTING.md)

</div>

---

HadalBot 是一次独立的 Go 重写：将上游请求、事件接入、插件调度和资源管理收敛到清晰的模块边界，逐步替换旧版中耦合的业务与运行时依赖。

新功能以 Go 实现；已有 TypeScript 插件可通过按需启动的独立 worker 接入。数据库、Redis、上游连接和插件默认关闭，基础服务可独立运行。

> **重构进行中。** 当前已提供可运行的平台基础、上游 SDK 和插件系统，尚未完成旧版业务的全量迁移。旧绑定网页、账号持久化、游戏计算、渲染和历史数据不能视为已经兼容。

## 核心能力

| 能力 | 当前实现 |
| :--- | :--- |
| 千寻接入 | 框架 / 微信直连两种模式，99 项 HTTP 契约，支持流式文件下载 |
| 事件处理 | 13 类框架事件、9 类直连事件，HTTP 回调与 WebSocket、有界分片队列、会话内顺序及有限去重 |
| 米哈游上游 | 33 组、179 项命名契约，涵盖账号、游戏记录、签到、兑换码、抽卡及相关公开数据 |
| 账号 SDK | 登录、RSA 加密、票据交换、Cookie 绑定校验、角色查询与凭证续期；按账号隔离授权材料 |
| 请求网关 | 连接复用、来源限制、并发与队列预算、取消与超时、响应大小限制、只读重试 |
| 插件系统 | Go 工厂注册、TS 独立 worker、依赖与版本校验、启动回滚、排空停用及资源作用域 |
| 工程基础 | chi、Viper、slog、Prometheus，可选 PostgreSQL / Redis、显式迁移及优雅退出 |

接口数量按命名契约统计，包含不同模式和分组对同一地址的定义，不代表唯一 URL 数量。详见 [逐项覆盖清单](docs/upstream-coverage.md)。

## 项目状态

- [x] 独立 Go 工程、配置、日志、监控和构建流程
- [x] 请求网关、千寻 SDK 及事件接入
- [x] 米哈游上游 SDK 与账号会话工具
- [x] Go 插件、TS 兼容层及旧插件静态审计
- [ ] 账号所有权、凭证加密存储与绑定页面
- [ ] 米哈游及其他游戏插件的完整业务迁移
- [ ] 游戏计算、模板适配及图片渲染
- [ ] 历史数据导入、真实上游验收与生产切换

部分 App / BBS 接口需要调用方提供有效 DS2 签名能力；候选接口默认禁止调用。事件队列是进程内队列，不提供崩溃后的持久化投递保证。详见 [上游接入与边界](docs/upstreams.md)。

## 快速开始

### 环境要求

| 组件 | 要求 |
| :--- | :--- |
| Go | 项目工具链为 **1.27.3**，以 `go.mod` 为准；允许工具链自动下载时可由 Go 获取 |
| Node.js | **仅 TS 插件运行及其测试需要**，要求 24 或更高版本 |
| PostgreSQL / Redis | 可选，基础启动不依赖它们 |
| Docker | 可选，用于独立基础设施和容器部署 |

```sh
git clone https://github.com/surgar04/hadalbot.git
cd hadalbot

go run ./cmd/hadalbot -check-config -config configs/development.yaml
go run ./cmd/hadalbot -config configs/development.yaml
```

默认监听 **`127.0.0.1:18878`**，不连接数据库、Redis、千寻或米哈游，不加载插件或旧账号。

```sh
curl http://127.0.0.1:18878/healthz
curl http://127.0.0.1:18878/readyz
```

### 配置规则

优先级为 **环境变量 → 显式 YAML → 内置默认值**。未指定 `-config` 时不搜索当前目录的未知配置；未知字段、无效类型和不存在的显式配置会报错。

服务读取进程环境，**不会自动加载 `.env`**。变量参考见 [.env.example](.env.example)，Docker Compose 会读取本工程 `.env`。

```powershell
# PowerShell：替换为你自己生成的独立随机密钥。
$env:HADALBOT_HTTP_API_KEY = '替换为至少32字符的随机密钥'
go run ./cmd/hadalbot -config configs/development.yaml
```

生产模式及非回环监听必须配置至少 32 字符 API key。凭证、连接密码和上游密钥不要提交到仓库；上游密钥与事件回调 token 分别管理。

## 上游接入

`pkg/qianxun`、`pkg/mihoyo` 共用 `pkg/gateway`，调用使用 context 和资源预算，避免共享可变的“当前账号”。千寻两种模式分别生成类型；米哈游响应保留未公开字段，并区分传输错误、业务错误及验证挑战。

发消息、登录、签到和兑换码不会自动重放。从 [调用说明](docs/upstreams.md) 开始，结合 [契约目录](docs/contracts) 与 [覆盖清单](docs/upstream-coverage.md) 查阅参数及来源。

## 插件开发

| Go 插件 | TypeScript 插件 |
| :--- | :--- |
| 普通 Go 包，通过工厂编译注册 | 按需启动独立 Node worker |
| 按需实现初始化、就绪、观察、事件、消息和清理接口 | 兼容旧 `init / onReady / observeMessage / onEvent / onMessage / dispose` |
| 使用 Scope 管理任务、定时器和资源 | 使用有界 RPC 调用获批的 Go 宿主方法 |
| 更新代码后重新构建，启用时创建新实例 | 停用并重新启用后导入新代码 |

默认每插件并发 1、等待 0、钩子 5 秒；TS 共用最多 4 个存活 worker 的配额，每进程 V8 堆默认 128 MiB。堆限制不等于 RSS 硬上限，原生 Go 钩子必须遵守 context 取消契约。

**TS 契约兼容不等于全部旧业务即插即用。** 依赖旧数据库、渲染、权限或 Provider 的插件需要服务适配。旧项目 22 个插件的依赖见 [兼容性报告](docs/plugin-compatibility.json)。

- [插件开发与运维](docs/plugins.md)
- [Go 示例](internal/plugins/example/example.go)
- [TS 示例](examples/plugins/echo-ts) · [独立类型声明](sdks/typescript/index.d.ts)
- [插件配置示例](configs/plugins.example.yaml)

示例配置仅在显式指定时生效；真实收消息还需启用千寻事件入口。目录中新出现的文件不会自动获得执行权，宿主方法通过 `grants` 明确授权。

## 基础 HTTP 接口

| 方法 | 路径 | 用途 |
| :--- | :--- | :--- |
| GET | `/healthz`、`/readyz` | 公开存活与就绪探针 |
| GET | `/api/v1/system` | 构建信息及注册模块 |
| GET | `/api/v1/upstreams` | 上游状态、契约数量与资源统计 |
| GET | `/api/v1/plugins` | 插件状态及资源计数 |
| POST | `/api/v1/plugins/{id}/enable`、`disable` | 操作已登记插件 |
| GET | `/metrics` | Prometheus 指标 |
| POST | `/callbacks/qianxun` | 显式启用后的事件入口，独立 token |

配置密钥后，管理接口接受 `X-API-Key` 或 `Authorization: Bearer ...`，不接受 URL 查询串密钥；插件控制拒绝浏览器 Origin 请求。本地开发未配置密钥时允许访问管理路由。

完整规范见 [OpenAPI](docs/api/openapi.yaml)。旧数据库管理、绑定网页、活动报名等业务接口尚未迁移，具体用途见 [旧 HTTP 接口说明](docs/legacy-http-purpose.md)。

## 构建与检查

```sh
go run ./cmd/devtools fmt
go run ./cmd/devtools check
go run ./cmd/devtools lint
go run ./cmd/devtools vuln
go run ./cmd/devtools build
```

`check` 执行格式检查、依赖校验、`go vet` 和测试。构建输出到被 Git 忽略的 `bin/`，包含服务和迁移程序；开发工具按固定版本运行，不混入服务二进制。

```sh
# TS SDK 类型检查，仅开发 / CI 使用。
npm exec --yes --package=typescript@5.9.3 -- tsc --project sdks/typescript/tsconfig.json --noEmit

# 支持 CGO 和 C 编译器的环境可执行竞态检查。
go test -race ./...
```

TS 进程测试需要 Node，未安装时明确跳过。数据库 / Redis 集成测试仅在显式提供独立测试连接变量时执行。CI 覆盖 Windows / Linux、竞态、类型检查、静态分析和独立服务集成；配置存在不代表远程运行已通过。证据与未执行项目见 [验证记录](docs/verification.md)。

Windows 可用 [PowerShell 包装命令](scripts/project.ps1)，其他环境可用 [Makefile](Makefile)。

## 数据库与部署

PostgreSQL 使用 pgx，Redis 使用 go-redis；关闭的依赖不初始化连接池。Goose 迁移显式执行，服务启动不自动改表，不导入旧数据。

```sh
go run ./cmd/migrate -command status
# 确认独立数据库连接后再执行迁移。
go run ./cmd/migrate -command up
```

[Compose](compose.yaml) 使用独立数据库名、数据卷和宿主端口：PostgreSQL **15432**、Redis **16379**。先按 `.env.example` 创建自己的 `.env`，填写独立密码与经过 URL 编码的连接字符串。

```sh
docker compose up -d postgres redis
# 配好 app 的 API key 和独立数据库 DSN 后：
docker compose --profile app up -d --build
```

容器以非 root 用户运行，基础镜像不带 Node。TS 部署需另行准备 Node 及插件依赖；访问宿主千寻时配置容器可达地址。

## 目录结构

```text
cmd/                 服务、迁移、开发、契约生成及审计命令
pkg/gateway/         上游 HTTP 调度与资源预算
pkg/qianxun/         千寻客户端、事件、回调及 WebSocket
pkg/mihoyo/          米哈游上游与账号会话 SDK
pkg/plugin/          插件 SDK、生命周期及进程适配器
internal/app/        组合根与应用生命周期
internal/pluginhost/ 插件发现、授权、管理及事件接入
internal/plugins/    编译注册的 Go 插件
internal/storage/    可选 PostgreSQL / Redis 连接
sdks/typescript/     TS 兼容类型契约
examples/            插件示例
configs/             开发、生产与插件配置示例
migrations/          显式版本化 SQL 迁移
docs/                架构、契约、兼容边界与验证记录
licenses/            第三方依赖许可文本
```

## 文档导航

| 想了解 | 阅读 |
| :--- | :--- |
| 架构与模块边界 | [架构约定](docs/architecture.md) |
| 上游调用和事件语义 | [上游接入](docs/upstreams.md) · [覆盖清单](docs/upstream-coverage.md) |
| 插件生命周期与授权 | [插件开发](docs/plugins.md) |
| 旧版迁移边界 | [旧 HTTP 用途](docs/legacy-http-purpose.md) · [插件审计](docs/plugin-compatibility.json) |
| 下一阶段工作 | [迁移路线](docs/migration-plan.md) |
| 已验证与未验证项目 | [验证记录](docs/verification.md) |
| 资料来源 | [来源说明](docs/reference-sources.md) · [千寻勘误](docs/contracts/qianxun/errata.md) |

## 参与贡献

欢迎通过 [Issues](https://github.com/surgar04/hadalbot/issues) 反馈问题，或提交 Pull Request。先阅读 [贡献指南](CONTRIBUTING.md)，保持中文注释、资源预算及可验证的行为边界。

涉及账号或安全问题时，遵循 [安全说明](SECURITY.md)，不要在公开 Issue 中提交 Cookie、Token、HAR 原件或完整私有日志。

## 开源协议与来源

本项目原创代码采用 **[MIT License](LICENSE)**，允许使用、修改和分发，保留相应版权及许可声明。

第三方依赖、接口文档、字段描述、示例及品牌标识保留各自的权利和适用条款，不因本仓库的 MIT 协议而被重新授权。依赖许可和上游资料说明见 [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md)。千寻、米哈游 / HoYoverse 等名称仅说明兼容对象，本项目与其不存在官方隶属或背书关系。
