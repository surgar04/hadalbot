// Package plugin 定义跨运行时的插件契约、生命周期和有界调度。
// Go插件采用工厂编译注册；进程型插件通过同一契约接入，业务无需感知底层传输。
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"log/slog"
	"time"
)

const APIVersion = 1 // APIVersion 是宿主协议主版本，破坏兼容的修改必须升级。
var (
	ErrBusy     = errors.New("插件执行预算已满")
	ErrNotReady = errors.New("插件尚未就绪或正在停止")
	ErrDenied   = errors.New("插件能力未授权")
	ErrClosed   = errors.New("插件管理器已关闭")
	ErrTooLarge = errors.New("插件RPC数据超过帧预算")
	ErrHook     = errors.New("插件钩子执行失败，详细原因由插件自身受控处理")
)

// Manifest 兼容旧plugin.json展示字段；新增字段负责依赖、资源及权限声明。
// ConfigSchema/Commands/Features保留原始描述，不执行清单中的代码。
type Manifest struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Runtime         string            `json:"runtime,omitempty"` // Runtime：go或typescript；旧清单省略时由发现器补typescript。
	API             int               `json:"apiVersion,omitempty"`
	Description     string            `json:"description,omitempty"`
	Entry           string            `json:"entry,omitempty"`        // Entry 相对插件目录，禁止绝对路径及目录逃逸。
	Priority        int               `json:"priority,omitempty"`     // Priority 越大越先执行；依赖顺序优先于此值。
	Dependencies    map[string]string `json:"dependencies,omitempty"` // Dependencies 支持精确版本、>=、^、~和*。
	Capabilities    []string          `json:"capabilities,omitempty"` // Capabilities 为显式宿主方法名，例如qx.sendText，不隐式授予通配权限。
	Limits          Limits            `json:"limits,omitempty"`
	Features        []string          `json:"features,omitempty"`
	Commands        json.RawMessage   `json:"commands,omitempty"`
	ConfigSchema    json.RawMessage   `json:"configSchema,omitempty"`
	RenderTemplates json.RawMessage   `json:"renderTemplates,omitempty"`
}

// Limits 限制单插件并发、等待和后台资源；零值采用保守默认。
// 超时以毫秒存储，JSON不依赖Go的纳秒duration编码。
type Limits struct {
	Concurrency        int `json:"concurrency,omitempty"`
	Queue              int `json:"queue,omitempty"`
	HookTimeoutMS      int `json:"hookTimeoutMs,omitempty"`
	LifecycleTimeoutMS int `json:"lifecycleTimeoutMs,omitempty"`
	MaxTasks           int `json:"maxTasks,omitempty"`
	MaxCleanups        int `json:"maxCleanups,omitempty"`
}

func (l Limits) hookTimeout() time.Duration { return time.Duration(l.HookTimeoutMS) * time.Millisecond }
func (l Limits) lifecycleTimeout() time.Duration {
	return time.Duration(l.LifecycleTimeoutMS) * time.Millisecond
}

// Plugin只强制声明元数据，其余钩子通过可选接口发现，避免空实现。
type Plugin interface{ Manifest() Manifest }
type Factory func() Plugin // Factory 每次启用都创建新实例；不能复用已经Dispose的对象。
type Initializer interface {
	Init(context.Context, *Context) error
}
type Readier interface {
	Ready(context.Context, *Context) error
}
type Observer interface {
	ObserveMessage(context.Context, Message, *Context) error
}
type EventHandler interface {
	OnEvent(context.Context, qianxun.Event, *Context) (bool, error)
}
type MessageHandler interface {
	OnMessage(context.Context, Message, *Context) (bool, error)
}
type Disposer interface{ Dispose(context.Context) error }

// HealthChecker供进程适配器报告不可恢复的故障；失败后隔离实例，需显式重新启用。
type HealthChecker interface{ Healthy() bool }

type accountKey struct{}

// WithAccount由可信入口绑定当前机器人；宿主方法应读取该上下文，不能信任插件正文指定的机器人。
func WithAccount(ctx context.Context, wxid string) context.Context {
	return context.WithValue(ctx, accountKey{}, wxid)
}
func AccountWXID(ctx context.Context) string { v, _ := ctx.Value(accountKey{}).(string); return v }

// Message 与旧ParsedMessage对应；Raw和Event不丢弃未知字段，不允许处理器原地修改切片。
type Message struct {
	Source     string          `json:"source"`
	Content    string          `json:"content"`
	SenderWXID string          `json:"senderWxid"`
	TargetWXID string          `json:"targetWxid"`
	GroupWXID  string          `json:"groupWxid,omitempty"`
	AtWXIDList []string        `json:"atWxidList"`
	MsgID      string          `json:"msgId"`
	MsgType    int             `json:"msgType"`
	Raw        json.RawMessage `json:"raw"`
	Event      json.RawMessage `json:"event"`
}

// Caller由组合根提供显式的方法白名单，不提供任意URL、SQL或跨账号凭证查询。
type Caller interface {
	Call(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
}
type CallFunc func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)

func (f CallFunc) Call(c context.Context, id, method string, args json.RawMessage) (json.RawMessage, error) {
	return f(c, id, method, args)
}

// Policy同时约束宿主调用和事件分发；应由权限/群配置服务实现。nil时不额外过滤事件，能力仍需清单声明。
type Policy interface {
	AllowCall(context.Context, string, string) bool
	AllowEvent(context.Context, string, qianxun.Event, *Message) bool
}

// Context作用域只属于一次插件实例；Config返回副本，Call逐次检查能力和宿主策略。
type Context struct {
	ID           string
	Log          *slog.Logger
	Scope        *Scope
	config       json.RawMessage
	capabilities map[string]bool
	caller       Caller
	policy       Policy
}

func (c *Context) Config() json.RawMessage { return append(json.RawMessage(nil), c.config...) }
func (c *Context) Call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	if !c.capabilities[method] || c.caller == nil || (c.policy != nil && !c.policy.AllowCall(ctx, c.ID, method)) {
		return nil, ErrDenied
	}
	if len(args) > 1<<20 || !json.Valid(args) {
		return nil, errors.New("插件宿主调用参数无效或过大")
	}
	if c.Scope.ctx.Err() != nil {
		return nil, ErrNotReady
	}
	return c.caller.Call(ctx, c.ID, method, args)
}

type State string

const (
	Registered  State = "registered"
	Loading     State = "loading"
	Initialized State = "initialized"
	Ready       State = "ready"
	Draining    State = "draining"
	Stopped     State = "stopped"
	Failed      State = "failed"
	Blocked     State = "blocked" // Blocked为依赖不可用时派生的只读状态，实际实例保留以便有序停用。
	Disabled    State = "disabled"
)

// Status只暴露安全元数据，不包含配置、消息、目录、环境变量或错误原文。
type Status struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Runtime      string `json:"runtime"`
	State        State  `json:"state"`
	InFlight     int    `json:"in_flight"`
	Calls        uint64 `json:"calls"`
	Failures     uint64 `json:"failures"`
	Rejected     uint64 `json:"rejected"`
	Tasks        int    `json:"tasks"`
	TaskFailures uint64 `json:"task_failures"`
}
