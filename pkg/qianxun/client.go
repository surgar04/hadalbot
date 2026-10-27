// Package qianxun 封装千寻框架及微信客户端两种官方接入模式。
//
// 同一 Gateway 可以服务多个账号；账号由不可变默认配置或单次 WithWXID 指定，
// 不使用共享可变“当前账号”，避免并发请求串号。构造客户端不会启动微信或发起网络请求。
package qianxun

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/surgar04/hadalbot/pkg/gateway"
)

//go:embed contracts.json
var contracts embed.FS

// Scalar 接受上游版本间可能变化的字符串/数字，不经 float64，避免消息ID精度损失。
type Scalar string

func (s *Scalar) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	var text string
	if json.Unmarshal(b, &text) == nil {
		*s = Scalar(text)
		return nil
	}
	var n json.Number
	if json.Unmarshal(b, &n) != nil {
		return errors.New("字段必须是字符串或数字")
	}
	*s = Scalar(n)
	return nil
}
func (s Scalar) String() string { return string(s) }

// Ptr 为生成请求中的可选字段构造指针，区分“省略”与显式的零值。
func Ptr[T any](value T) *T { return &value }

// Reply 保留完整响应；Result 是按官方契约生成的结果类型。
// Raw/Result/Headers 可能含个人数据，日志只应使用 Code、Operation 等安全元数据。
type Reply[T any] struct {
	Code      int             `json:"code"`
	Message   string          `json:"msg"`
	Result    T               `json:"result"`
	WXID      string          `json:"wxid"`
	Port      Scalar          `json:"port"`
	PID       Scalar          `json:"pid"`
	Flag      string          `json:"flag"`
	Timestamp Scalar          `json:"timestamp"`
	Raw       json.RawMessage `json:"-"`
	Headers   http.Header     `json:"-"`
}

// BusinessError 表示 HTTP 成功但千寻 code 非200；Message 供受控业务处理，Error 不输出上游正文。
type BusinessError struct {
	Operation string
	Code      int
	Message   string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("千寻业务失败: operation=%s code=%d", e.Operation, e.Code)
}

var ErrUnsupported = errors.New("当前千寻接入模式不支持该接口")
var ErrInvalidResponse = errors.New("千寻响应不符合协议")

// Config 只描述一个接入地址及默认账号，真实 safekey 不得提交到仓库。
type Config struct {
	BaseURL   string
	SafeKey   string
	WXID      string
	Transport RPCTransport
}

// RPCTransport 可替换JSON RPC传输；HTTP文件下载仍经过Gateway。实现必须遵守ctx且不能重放写请求。
type RPCTransport interface {
	RoundTrip(context.Context, []byte, string) (json.RawMessage, error)
}
type callOptions struct{ wxid *string }

// CallOption 是单次调用选项，不修改客户端共享状态。
type CallOption func(*callOptions)

// WithWXID 选择本次框架调用的机器人；空字符串可显式清除默认账号。
func WithWXID(wxid string) CallOption { return func(o *callOptions) { o.wxid = &wxid } }

type Client struct {
	gateway       *gateway.Gateway
	base          url.URL
	mode          string
	safeKey, wxid string
	transport     RPCTransport
}

// FrameworkClient 调用7777端口的 /qianxun/httpapi，通过查询参数选择机器人。
type FrameworkClient struct{ client *Client }

// DirectClient 调用指定微信进程端口的 /wechat/httpapi；多开时每个端口构造独立句柄。
type DirectClient struct{ client *Client }

// NewFramework 创建千寻框架模式客户端；Gateway 的生命周期由组合根管理。
func NewFramework(g *gateway.Gateway, c Config) (*FrameworkClient, error) {
	client, e := newClient(g, c, "framework")
	if e != nil {
		return nil, e
	}
	return &FrameworkClient{client: client}, nil
}

// NewDirect 创建不经过千寻框架进程的微信客户端模式句柄。
func NewDirect(g *gateway.Gateway, c Config) (*DirectClient, error) {
	client, e := newClient(g, c, "direct")
	if e != nil {
		return nil, e
	}
	return &DirectClient{client: client}, nil
}
func newClient(g *gateway.Gateway, c Config, mode string) (*Client, error) {
	if g == nil {
		return nil, errors.New("千寻客户端需要共享网关")
	}
	u, e := url.Parse(c.BaseURL)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("千寻基址必须是纯HTTP来源地址")
	}
	return &Client{gateway: g, base: *u, mode: mode, safeKey: c.SafeKey, wxid: c.WXID, transport: c.Transport}, nil
}

// Operation 描述一个官方HTTP契约，两个模式相同名称的参数可能不同，不能混用。
type Operation struct {
	Mode, Method, Name, GoMethod, URL, Source string
	RequestSchema, ResponseSchema             map[string]any
	RequestExample, ResponseExample           json.RawMessage
}
type contractData struct {
	Operations []Operation       `json:"operations"`
	Events     []json.RawMessage `json:"events"`
}

var loadContracts = sync.OnceValue(func() contractData {
	b, e := contracts.ReadFile("contracts.json")
	if e != nil {
		panic(e)
	}
	var c contractData
	if e = json.Unmarshal(b, &c); e != nil {
		panic(e)
	}
	return c
})

// Operations 返回可独立修改的契约副本，用于文档、覆盖核对和受控接口注册。
func Operations() []Operation {
	b, _ := json.Marshal(loadContracts().Operations)
	var copy []Operation
	_ = json.Unmarshal(b, &copy)
	return copy
}

// OperationCount 直接读取已缓存目录的数量，不为健康/指标轮询复制整份契约。
func OperationCount() int { return len(loadContracts().Operations) }

var operationSet = sync.OnceValue(func() map[string]struct{} {
	set := make(map[string]struct{}, len(loadContracts().Operations))
	for _, o := range loadContracts().Operations {
		set[o.Mode+":"+o.Name] = struct{}{}
	}
	return set
})

func known(mode, operation string) bool { _, ok := operationSet()[mode+":"+operation]; return ok }
func (c *Client) endpoint(file bool, options []CallOption) url.URL {
	u := c.base
	if c.mode == "framework" {
		u.Path = "/qianxun/httpapi"
	} else {
		u.Path = "/wechat/httpapi"
	}
	if file {
		u.Path += "/file"
	}
	q := url.Values{}
	if c.safeKey != "" {
		q.Set("safekey", c.safeKey)
	}
	opts := callOptions{}
	for _, apply := range options {
		if apply != nil {
			apply(&opts)
		}
	}
	wxid := c.wxid
	if opts.wxid != nil {
		wxid = *opts.wxid
	}
	if c.mode == "framework" && wxid != "" {
		q.Set("wxid", wxid)
	}
	u.RawQuery = q.Encode()
	return u
}

var readonly = map[string]bool{"getWeChatList": true, "checkWeChat": true, "getAuthInfo": true, "getSelfInfo": true, "getLabelList": true, "getFriendList": true, "getGroupList": true, "getPublicList": true, "queryNewFriend": true, "queryObj": true, "queryGroup": true, "getMemberList": true, "getMemberNick": true, "getLoginStatus": true}

func call[T any](ctx context.Context, c *Client, operation string, data any, options ...CallOption) (out Reply[T], err error) {
	if !known(c.mode, operation) || operation == "downloadFile" {
		return out, ErrUnsupported
	}
	if data == nil {
		data = struct{}{}
	}
	body, e := json.Marshal(struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}{operation, data})
	if e != nil {
		return out, errors.New("千寻请求无法编码为JSON")
	}
	u := c.endpoint(false, options)
	var r gateway.Response
	if c.transport != nil {
		r.Body, e = c.transport.RoundTrip(ctx, body, u.Query().Get("wxid"))
	} else {
		r, e = c.gateway.Do(ctx, gateway.Request{Service: "qianxun", Operation: operation, Method: http.MethodPost, URL: &u, Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: body, ResponseLimit: 8 << 20, Idempotent: readonly[operation]})
	}
	if e != nil {
		return out, e
	}
	var envelope struct {
		Code      *int            `json:"code"`
		Message   string          `json:"msg"`
		Result    json.RawMessage `json:"result"`
		WXID      string          `json:"wxid"`
		Port      Scalar          `json:"port"`
		PID       Scalar          `json:"pid"`
		Flag      string          `json:"flag"`
		Timestamp Scalar          `json:"timestamp"`
	}
	if json.Unmarshal(r.Body, &envelope) != nil || envelope.Code == nil {
		return out, ErrInvalidResponse
	}
	out = Reply[T]{Code: *envelope.Code, Message: envelope.Message, WXID: envelope.WXID, Port: envelope.Port, PID: envelope.PID, Flag: envelope.Flag, Timestamp: envelope.Timestamp, Raw: r.Body, Headers: r.Headers}
	if out.Code != 200 {
		return out, &BusinessError{Operation: operation, Code: out.Code, Message: out.Message}
	}
	if len(envelope.Result) > 0 && json.Unmarshal(envelope.Result, &out.Result) != nil {
		return out, ErrInvalidResponse
	}
	return out, nil
}

// Call 为未指定结果类型的受控调用保留全部 result；接口名仍按当前模式白名单验证。
func (c *FrameworkClient) Call(ctx context.Context, operation string, data any, options ...CallOption) (Reply[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c.client, operation, data, options...)
}
func (c *DirectClient) Call(ctx context.Context, operation string, data any, options ...CallOption) (Reply[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c.client, operation, data, options...)
}
func (c *Client) download(ctx context.Context, path string, w io.Writer, options ...CallOption) (int64, error) {
	if strings.TrimSpace(path) == "" {
		return 0, errors.New("下载路径不能为空")
	}
	u := c.endpoint(true, options)
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()
	result, err := c.gateway.DoTo(ctx, gateway.Request{Service: "qianxun", Operation: "downloadFile", Method: http.MethodGet, URL: &u}, w)
	return result.BytesRead, err
}

// DownloadFileTo 有界流式下载上游文件，不把整张图片或附件读入内存；错误时目标可能包含部分内容。
func (c *FrameworkClient) DownloadFileTo(ctx context.Context, path string, destination io.Writer, options ...CallOption) (int64, error) {
	return c.client.download(ctx, path, destination, options...)
}

// DownloadFileTo 在直接微信模式下下载文件，不附加框架专属的 wxid 查询参数。
func (c *DirectClient) DownloadFileTo(ctx context.Context, path string, destination io.Writer, options ...CallOption) (int64, error) {
	return c.client.download(ctx, path, destination, options...)
}
