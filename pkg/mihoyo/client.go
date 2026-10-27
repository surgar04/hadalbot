// Package mihoyo 提供账号隔离的HoYoLAB/HoYoverse上游API，不包含旧项目本地HTTP业务服务。
//
// 构造只建立不可变配置，不登录账号、不签到、不兑换码。调用显式经过共享有界Gateway；
// 每次请求重新构造Cookie/签名，禁止共享全局CookieJar，避免不同微信用户串号。
package mihoyo

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/surgar04/hadalbot/pkg/gateway"
)

//go:embed contracts.json
var contracts embed.FS

// Secret 避免凭证被fmt、JSON或Go调试格式意外展开；传输时必须显式Reveal。
type Secret string

func (Secret) String() string               { return "[redacted]" }
func (Secret) GoString() string             { return "mihoyo.Secret([redacted])" }
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (s Secret) Reveal() string             { return string(s) }

// Credentials 是单个账号的授权上下文，SDK不负责持久化，不应跨所有者复用。
type Credentials struct {
	Cookie   Secret `json:"-"`
	DeviceID string `json:"-"`
	DeviceFP string `json:"-"`
}

// SigningInput 始终是将要实际发送的原始查询串和正文，签名后不再重排或重新编码。
type SigningInput struct {
	Endpoint Endpoint
	Method   string
	RawQuery string
	Body     []byte
}

// Signer 用于调用方提供已确认有效的App DS2签名能力，SDK不猜测未知salt。
type Signer func(context.Context, SigningInput) (string, error)

// Options 是账号实例的固定配置，传入map/header会复制，之后不会被调用方修改影响。
type Options struct {
	Credentials     Credentials
	Headers         http.Header
	DS2Signer       Signer
	AllowCandidates bool
	// OriginOverrides 用于明确的测试/代理映射；只允许HTTPS或回环HTTP，不改变原端点的凭证域判断。
	OriginOverrides map[string]string
}
type clientCore struct {
	gateway *gateway.Gateway
	options Options
}

// Endpoint 是一个有明确来源的上游契约，不接受运行时任意URL代理。
type Endpoint struct {
	ID          string          `json:"id"`
	Group       string          `json:"group"`
	Name        string          `json:"name"`
	GoGroup     string          `json:"go_group"`
	GoMethod    string          `json:"go_method"`
	Method      string          `json:"method"`
	Origin      string          `json:"origin"`
	Path        string          `json:"path"`
	Summary     string          `json:"summary"`
	Candidate   bool            `json:"candidate"`
	Signature   string          `json:"signature"`
	Idempotent  bool            `json:"idempotent"`
	Origins     []string        `json:"origins"`
	RawResponse bool            `json:"raw_response"`
	Source      string          `json:"source"`
	Operation   json.RawMessage `json:"operation"`
}
type catalog struct {
	list  []Endpoint
	index map[string]Endpoint
}

var loadCatalog = sync.OnceValue(func() catalog {
	b, e := contracts.ReadFile("contracts.json")
	if e != nil {
		panic(e)
	}
	var all []Endpoint
	if e = json.Unmarshal(b, &all); e != nil {
		panic(e)
	}
	index := make(map[string]Endpoint, len(all))
	for _, v := range all {
		if _, exists := index[v.ID]; exists {
			panic("米哈游契约ID重复")
		}
		index[v.ID] = v
	}
	return catalog{list: all, index: index}
})

// Endpoints 返回目录副本，包含候选/签名条件，供文档和覆盖检查使用。
func Endpoints() []Endpoint {
	b, _ := json.Marshal(loadCatalog().list)
	var out []Endpoint
	_ = json.Unmarshal(b, &out)
	return out
}

// EndpointCount 供状态轮询读取，不反复分配完整的契约副本。
func EndpointCount() int { return len(loadCatalog().list) }

// Origins 返回全部固定来源，便于组合根创建Gateway白名单；不含用户账号或授权查询串。
func Origins() []string {
	set := map[string]struct{}{}
	for _, v := range loadCatalog().list {
		for _, origin := range v.Origins {
			set[origin] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for origin := range set {
		out = append(out, origin)
	}
	sort.Strings(out)
	return out
}

// New 创建上游客户端；只有真正调用具体方法时才访问网络。
func New(g *gateway.Gateway, o Options) (*Client, error) {
	if g == nil {
		return nil, errors.New("米哈游客户端需要共享网关")
	}
	normalized := make(http.Header)
	mergeHeaders(normalized, o.Headers)
	if normalized.Get("Cookie") != "" || normalized.Get("Authorization") != "" {
		return nil, errors.New("账号凭证必须使用Credentials，不能放入共享默认头")
	}
	copyOptions := o
	copyOptions.Headers = normalized
	copyOptions.OriginOverrides = make(map[string]string, len(o.OriginOverrides))
	for from, to := range o.OriginOverrides {
		u, err := url.Parse(to)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return nil, errors.New("上游来源映射无效")
		}
		host := u.Hostname()
		ip := net.ParseIP(host)
		localHTTP := u.Scheme == "http" && (host == "localhost" || (ip != nil && ip.IsLoopback()))
		if u.Scheme != "https" && !localHTTP {
			return nil, errors.New("凭证上游必须使用HTTPS或本机测试HTTP")
		}
		copyOptions.OriginOverrides[from] = strings.TrimSuffix(to, "/")
	}
	return clientFrom(&clientCore{gateway: g, options: copyOptions}), nil
}

// ForAccount 返回独立账号句柄，复用连接和队列但不修改原实例的凭证。
func (c *Client) ForAccount(credentials Credentials) *Client {
	o := c.core.options
	o.Credentials = credentials
	return clientFrom(&clientCore{gateway: c.core.gateway, options: o})
}

// Parameters 用于底层受控调用；普通业务应优先使用各分组生成的强类型方法。
type Parameters struct {
	Query   url.Values
	Path    map[string]string
	Headers http.Header
	Body    any
}
type callOptions struct {
	headers        http.Header
	origin         string
	allowCandidate bool
	signer         Signer
}

// CallOption 只影响本次请求，不修改共享实例。
type CallOption func(*callOptions)

// WithHeaders 复制附加头；不得用它替代账号所有权检查。
func WithHeaders(h http.Header) CallOption {
	copy := h.Clone()
	return func(o *callOptions) {
		if o.headers == nil {
			o.headers = make(http.Header)
		}
		mergeHeaders(o.headers, copy)
	}
}

// WithOrigin 只能选择当前端点目录明确列出的备用来源，不能指定任意地址。
func WithOrigin(origin string) CallOption { return func(o *callOptions) { o.origin = origin } }

// AllowCandidate 明确允许调用资料中标为未验证的候选接口，不意味着接口已得到真实验收。
func AllowCandidate() CallOption { return func(o *callOptions) { o.allowCandidate = true } }

// WithDS2Signer 提供本次请求的真实签名实现；未知App签名不会自动套用历史salt。
func WithDS2Signer(s Signer) CallOption { return func(o *callOptions) { o.signer = s } }

// Reply 保留上游原始JSON与响应头；包含凭证的结果不能序列化到公开接口。
type Reply[T any] struct {
	Retcode int             `json:"retcode"`
	Message string          `json:"message"`
	Data    T               `json:"data"`
	Raw     json.RawMessage `json:"-"`
	Headers http.Header     `json:"-"`
}

// BusinessError 保留上游错误码与消息，但Error只给出脱敏摘要。
type BusinessError struct {
	Operation string
	Retcode   int
	Message   string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("米哈游业务失败: operation=%s retcode=%d", e.Operation, e.Retcode)
}

// ChallengeError 显式交还Aigis或账号二次验证，不自动绕过、代答或循环提交验证码。
type ChallengeError struct {
	Operation string
	Kind      string
	Data      json.RawMessage
}

func (e *ChallengeError) Error() string {
	return "米哈游接口需要安全验证: operation=" + e.Operation + " kind=" + e.Kind
}

var ErrDS2Required = errors.New("该App接口需要调用方提供当前有效的DS2签名")
var ErrCandidate = errors.New("候选接口尚未验证，需要显式AllowCandidate")
var ErrInvalidReply = errors.New("米哈游响应不符合JSON业务协议")

func parameterString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return x.String()
	case json.RawMessage:
		return string(x)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
func encodeBody(body any) ([]byte, error) {
	if raw, ok := body.(json.RawMessage); ok {
		if !json.Valid(raw) {
			return nil, errors.New("请求正文不是有效JSON")
		}
		return bytes.Clone(raw), nil
	}
	if body == nil {
		body = struct{}{}
	}
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		return nil, errors.New("请求正文无法编码")
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
}
func mergeHeaders(destination, source http.Header) {
	for key, values := range source {
		destination[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
	}
}
func officialCredentialHost(origin string) bool {
	u, e := url.Parse(origin)
	if e != nil {
		return false
	}
	h := u.Hostname()
	return strings.HasSuffix(h, ".hoyolab.com") || strings.HasSuffix(h, ".hoyoverse.com") || strings.HasSuffix(h, ".mihoyo.com")
}

func (c *clientCore) call(ctx context.Context, id string, p Parameters, options ...CallOption) (out Reply[json.RawMessage], err error) {
	endpoint, ok := loadCatalog().index[id]
	if !ok {
		return out, errors.New("未知米哈游上游接口")
	}
	opts := callOptions{}
	for _, apply := range options {
		if apply != nil {
			apply(&opts)
		}
	}
	if endpoint.Candidate && !c.options.AllowCandidates && !opts.allowCandidate {
		return out, ErrCandidate
	}
	selected := endpoint.Origin
	if opts.origin != "" {
		allowed := false
		for _, origin := range endpoint.Origins {
			if origin == opts.origin {
				allowed = true
			}
		}
		if !allowed {
			return out, gateway.ErrOrigin
		}
		selected = opts.origin
	}
	target := selected
	if override, exists := c.options.OriginOverrides[selected]; exists {
		target = override
	}
	path := endpoint.Path
	for key, value := range p.Path {
		if value == "" || value == "." || value == ".." {
			return out, errors.New("上游路径参数无效")
		}
		path = strings.ReplaceAll(path, "{"+key+"}", url.PathEscape(value))
	}
	if strings.ContainsAny(path, "{}") {
		return out, errors.New("上游路径参数缺失")
	}
	u, err := url.Parse(target + path)
	if err != nil {
		return out, errors.New("上游地址构建失败")
	}
	u.RawQuery = p.Query.Encode()
	var body []byte
	if endpoint.Method != http.MethodGet {
		body, err = encodeBody(p.Body)
		if err != nil {
			return out, err
		}
		if len(body) > 2<<20 {
			return out, errors.New("上游请求体超过2MiB")
		}
	}
	headers := http.Header{"Accept": []string{"application/json"}, "User-Agent": []string{"HadalBot-Go/0.1"}, "X-Rpc-Language": []string{"zh-cn"}, "X-Rpc-Client_type": []string{"4"}, "X-Rpc-App_version": []string{"2.54.0"}}
	mergeHeaders(headers, c.options.Headers)
	mergeHeaders(headers, p.Headers)
	mergeHeaders(headers, opts.headers)
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	}
	if officialCredentialHost(endpoint.Origin) {
		credentials := c.options.Credentials
		if credentials.Cookie != "" {
			headers.Set("Cookie", credentials.Cookie.Reveal())
		}
		if credentials.DeviceID != "" {
			headers.Set("x-rpc-device_id", credentials.DeviceID)
		}
		if credentials.DeviceFP != "" {
			headers.Set("x-rpc-device_fp", credentials.DeviceFP)
		}
	} else {
		// Enka等第三方公开上游绝不能接收到HoYoLAB的Cookie、设备ID和账号授权头。
		for key := range headers {
			if strings.EqualFold(key, "Cookie") || strings.EqualFold(key, "Authorization") || strings.HasPrefix(strings.ToLower(key), "x-rpc-") {
				headers.Del(key)
			}
		}
	}
	if endpoint.Signature == "hsr-ds1" && headers.Get("DS") == "" {
		ds, e := RandomDS1(LegacyHSRSalt)
		if e != nil {
			return out, e
		}
		headers.Set("DS", ds)
		headers.Set("x-rpc-app_version", "1.5.0")
		headers.Set("x-rpc-client_type", "5")
	}
	if endpoint.Signature == "legacy-ds2" && headers.Get("DS") == "" {
		ds, e := RandomDS2(LegacyOverseasSalt, u.RawQuery, body)
		if e != nil {
			return out, e
		}
		headers.Set("DS", ds)
	}
	if endpoint.Signature == "app-ds2" && headers.Get("DS") == "" {
		signer := opts.signer
		if signer == nil {
			signer = c.options.DS2Signer
		}
		if signer == nil {
			return out, ErrDS2Required
		}
		ds, e := signer(ctx, SigningInput{Endpoint: endpoint, Method: endpoint.Method, RawQuery: u.RawQuery, Body: bytes.Clone(body)})
		if e != nil {
			return out, errors.New("应用DS2签名失败")
		}
		if ds == "" {
			return out, ErrDS2Required
		}
		headers.Set("DS", ds)
	}
	response, transportErr := c.gateway.Do(ctx, gateway.Request{Service: "mihoyo", Operation: id, Method: endpoint.Method, URL: u, Headers: headers, Body: body, ResponseLimit: 8 << 20, Idempotent: endpoint.Idempotent})
	out.Raw = response.Body
	out.Headers = response.Headers
	for _, header := range []string{"x-rpc-aigis", "x-rpc-verify"} {
		if value := response.Headers.Get(header); value != "" && json.Valid([]byte(value)) {
			return out, &ChallengeError{Operation: id, Kind: header, Data: json.RawMessage(value)}
		}
	}
	if transportErr != nil {
		return out, transportErr
	}
	if endpoint.RawResponse {
		if !json.Valid(response.Body) {
			return out, ErrInvalidReply
		}
		out.Data = response.Body
		return out, nil
	}
	var envelope struct {
		Retcode *int            `json:"retcode"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(response.Body, &envelope) != nil || envelope.Retcode == nil {
		return out, ErrInvalidReply
	}
	out.Retcode = *envelope.Retcode
	out.Message = envelope.Message
	out.Data = envelope.Data
	if out.Retcode != 0 {
		return out, &BusinessError{Operation: id, Retcode: out.Retcode, Message: out.Message}
	}
	return out, nil
}

// Invoke 是白名单内的底层调用口，不提供任意URL代理；建议业务优先调用生成的具体方法。
func (c *Client) Invoke(ctx context.Context, id string, p Parameters, options ...CallOption) (Reply[json.RawMessage], error) {
	return c.core.call(ctx, id, p, options...)
}

// Decode 将通用上游data转换为业务选定的类型，Raw和Headers继续保留。
func Decode[T any](input Reply[json.RawMessage]) (Reply[T], error) {
	out := Reply[T]{Retcode: input.Retcode, Message: input.Message, Raw: input.Raw, Headers: input.Headers}
	if len(input.Data) > 0 && json.Unmarshal(input.Data, &out.Data) != nil {
		return out, ErrInvalidReply
	}
	return out, nil
}
