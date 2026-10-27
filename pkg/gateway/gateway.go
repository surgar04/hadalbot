// Package gateway 提供有界、可取消、复用连接的上游 HTTP 调度网关。
//
// 网关只负责传输，不解释米哈游 retcode 或千寻 code，也不保存账号凭证。
// 每个上游使用独立的并发/等待预算，避免慢接口阻塞其他服务；关闭时取消在途请求。
package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrBusy 表示并发和等待队列都已满；调用方应向上游业务施加背压，不应无限重排队。
	ErrBusy = errors.New("上游调度队列已满")
	// ErrClosed 表示网关已经关闭，不能提交新请求。
	ErrClosed = errors.New("上游网关已关闭")
	// ErrOrigin 表示请求目标未列入该服务的显式来源清单。
	ErrOrigin = errors.New("上游来源不在允许范围内")
	// ErrResponseTooLarge 表示解压后的响应超过资源预算。
	ErrResponseTooLarge = errors.New("上游响应超过大小限制")
)

// Limits 定义单个上游的固定预算；Origins 只接受 scheme://host[:port]，不允许携带用户信息。
type Limits struct {
	Concurrency int
	QueueSize   int
	Origins     []string
	// MinInterval 是两次实际尝试之间的最小间隔；零表示不限速。重试也计入限速。
	MinInterval time.Duration
}

// Options 是创建时的不可变配置。Transport 可注入测试传输，网关不修改调用方对象。
type Options struct {
	Services         map[string]Limits
	Timeout          time.Duration
	MaxResponseBytes int64
	MaxRetries       int
	Transport        http.RoundTripper
}

// Request 表示一次逻辑请求。Body 在 Do 返回前不得修改；Headers 会被复制。
type Request struct {
	Service   string
	Operation string
	Method    string
	URL       *url.URL
	Headers   http.Header
	Body      []byte
	// ResponseLimit 可缩小本次响应预算；零使用网关上限，不能扩大网关总预算。
	ResponseLimit int64
	sink          io.Writer
	// Idempotent 只能在接口明确无副作用时设置；POST 默认不会自动重试。
	Idempotent bool
}

// Response 保留 HTTP 状态、响应头及有界原始正文，由具体 SDK 解码。
// Headers/Body 可能包含凭证或个人数据，不应写入日志或共享到其他账号。
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Attempts   int
	BytesRead  int64
}

// StatusError 是脱敏的 HTTP 层错误。可用 errors.As 读取状态码；不包含 URL、正文或凭证。
type StatusError struct {
	Service, Operation string
	StatusCode         int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("上游HTTP失败: service=%s operation=%s status=%d", e.Service, e.Operation, e.StatusCode)
}

// TransportError 隐藏请求 URL，只通过 Unwrap 保留底层网络/上下文错误。
type TransportError struct {
	Service, Operation string
	Cause              error
}

func (e *TransportError) Error() string {
	return "上游网络请求失败: service=" + e.Service + " operation=" + e.Operation
}
func (e *TransportError) Unwrap() error { return e.Cause }

type pool struct {
	slots, waiting  chan struct{}
	origins         map[string]struct{}
	interval        time.Duration
	mu              sync.Mutex
	next            time.Time
	epoch           uint64
	running, queued atomic.Int64
}

// Stats 是不包含账号和地址的调度快照；计数为整个网关生命周期累计值。
type Stats struct {
	Running   int64  `json:"running"`
	Queued    int64  `json:"queued"`
	Completed uint64 `json:"completed"`
	Failed    uint64 `json:"failed"`
	Rejected  uint64 `json:"rejected"`
	Retries   uint64 `json:"retries"`
}

// Gateway 可并发使用。它不会为每次请求额外创建 worker goroutine，等待数量由各服务预算限制。
type Gateway struct {
	client                               *http.Client
	pools                                map[string]*pool
	timeout                              time.Duration
	maxBytes                             int64
	retries                              int
	ctx                                  context.Context
	cancel                               context.CancelFunc
	closed                               atomic.Bool
	completed, failed, rejected, retried atomic.Uint64
	owned                                *http.Transport
}

// New 创建独立网关。非法配置立即失败，构造阶段不连接任何上游。
func New(o Options) (*Gateway, error) {
	if len(o.Services) == 0 {
		return nil, errors.New("必须配置至少一个上游服务")
	}
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxResponseBytes == 0 {
		o.MaxResponseBytes = 8 << 20
	}
	if o.Timeout < 0 || o.MaxResponseBytes < 1 || o.MaxResponseBytes > 64<<20 || o.MaxRetries < 0 || o.MaxRetries > 3 {
		return nil, errors.New("网关超时、响应上限或重试次数无效")
	}
	ctx, cancel := context.WithCancel(context.Background())
	g := &Gateway{pools: make(map[string]*pool, len(o.Services)), timeout: o.Timeout, maxBytes: o.MaxResponseBytes, retries: o.MaxRetries, ctx: ctx, cancel: cancel}
	for name, l := range o.Services {
		if name == "" || l.Concurrency < 1 || l.QueueSize < 0 || l.MinInterval < 0 || len(l.Origins) == 0 {
			cancel()
			return nil, errors.New("上游并发、队列或来源配置无效")
		}
		p := &pool{slots: make(chan struct{}, l.Concurrency), waiting: make(chan struct{}, l.QueueSize), origins: make(map[string]struct{}), interval: l.MinInterval}
		for _, s := range l.Origins {
			u, err := url.Parse(s)
			if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
				cancel()
				return nil, errors.New("上游来源格式无效")
			}
			p.origins[origin(u)] = struct{}{}
		}
		g.pools[name] = p
	}
	transport := o.Transport
	if transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
		t.MaxIdleConns = 64
		t.MaxIdleConnsPerHost = 16
		t.MaxConnsPerHost = 32
		t.ResponseHeaderTimeout = 10 * time.Second
		t.TLSHandshakeTimeout = 5 * time.Second
		t.IdleConnTimeout = 90 * time.Second
		t.MaxResponseHeaderBytes = 64 << 10
		transport = t
		g.owned = t
	}
	// 禁止自动跨域重定向，防止 safekey、Cookie 或 Authorization 被发送到第三方。
	g.client = &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return g, nil
}
func origin(u *url.URL) string { return strings.ToLower(u.Scheme + "://" + u.Host) }

// Do 的截止时间包含排队、限速、退避和网络阶段。返回 HTTP 错误时仍保留有界 Response，
// 便于 SDK 识别风控状态；外层日志应只记录安全的错误摘要。
func (g *Gateway) Do(parent context.Context, r Request) (out Response, err error) {
	if g.closed.Load() {
		return out, ErrClosed
	}
	p, ok := g.pools[r.Service]
	if !ok {
		return out, ErrOrigin
	}
	if r.URL == nil || r.URL.User != nil || r.URL.Fragment != "" {
		return out, ErrOrigin
	}
	if _, ok = p.origins[origin(r.URL)]; !ok {
		return out, ErrOrigin
	}
	if r.Method == "" {
		return out, errors.New("请求必须显式指定HTTP方法")
	}
	if r.ResponseLimit < 0 {
		return out, ErrResponseTooLarge
	}
	ctx, cancel := context.WithTimeout(parent, g.timeout)
	defer cancel()
	stop := context.AfterFunc(g.ctx, cancel)
	defer stop()
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = g.acquire(ctx, p); err != nil {
		g.rejected.Add(1)
		return out, err
	}
	defer func() {
		<-p.slots
		p.running.Add(-1)
		if err != nil {
			g.failed.Add(1)
		} else {
			g.completed.Add(1)
		}
	}()
	for attempt := 0; attempt <= g.retries; attempt++ {
		if err = pace(ctx, p); err != nil {
			return out, err
		}
		out, err = g.attempt(ctx, r)
		out.Attempts = attempt + 1
		serverDelay := time.Duration(0)
		if out.StatusCode == 429 || out.StatusCode == 503 {
			value := out.Headers.Get("Retry-After")
			if seconds, e := strconv.ParseInt(value, 10, 64); e == nil && seconds > 0 {
				if seconds > 86400 {
					return out, err
				}
				serverDelay = time.Duration(seconds) * time.Second
			} else if date, e := http.ParseTime(value); e == nil {
				serverDelay = time.Until(date)
			}
			if serverDelay > 0 {
				postpone(p, serverDelay)
			}
		}
		var statusError *StatusError
		retry := r.Idempotent && errors.As(err, &statusError) && attempt < g.retries && (out.StatusCode == 429 || out.StatusCode == 502 || out.StatusCode == 503 || out.StatusCode == 504)
		// 网络断开可能发生在上游已执行之后；仅无副作用请求允许网络级重试。
		var network *TransportError
		if errors.As(err, &network) && r.Idempotent && attempt < g.retries && ctx.Err() == nil {
			retry = true
		}
		if !retry {
			return out, err
		}
		delay := time.Duration(1<<attempt) * 100 * time.Millisecond
		if serverDelay > delay {
			delay = serverDelay
		}
		if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
			return out, err
		}
		g.retried.Add(1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return out, ctx.Err()
		case <-timer.C:
		}
	}
	return out, err
}

// acquire 先尝试立即执行，再占用固定等待预算；不分配无界的排队任务或 goroutine。
func (g *Gateway) acquire(ctx context.Context, p *pool) error {
	select {
	case p.slots <- struct{}{}:
		p.running.Add(1)
		return nil
	default:
	}
	select {
	case p.waiting <- struct{}{}:
		p.queued.Add(1)
	default:
		return ErrBusy
	}
	defer func() { <-p.waiting; p.queued.Add(-1) }()
	select {
	case p.slots <- struct{}{}:
		p.running.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func pace(ctx context.Context, p *pool) error {
	for {
		p.mu.Lock()
		now := time.Now()
		if p.interval == 0 && !p.next.After(now) {
			p.mu.Unlock()
			return ctx.Err()
		}
		due := p.next
		if due.Before(now) {
			due = now
		}
		p.next = due.Add(p.interval)
		epoch := p.epoch
		p.mu.Unlock()
		timer := time.NewTimer(time.Until(due))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		p.mu.Lock()
		valid := epoch == p.epoch
		p.mu.Unlock()
		if valid {
			return nil
		}
	}
}
func postpone(p *pool, delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	until := time.Now().Add(delay)
	if p.next.Before(until) {
		p.next = until
	}
	p.epoch++
}
func (g *Gateway) attempt(ctx context.Context, r Request) (out Response, err error) {
	req, e := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(r.Body))
	if e != nil {
		return out, errors.New("上游请求格式无效")
	}
	req.Header = r.Headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	resp, e := g.client.Do(req)
	if e != nil {
		var ue *url.Error
		if errors.As(e, &ue) {
			e = ue.Err
		}
		return out, &TransportError{Service: r.Service, Operation: r.Operation, Cause: e}
	}
	defer resp.Body.Close()
	out.StatusCode = resp.StatusCode
	out.Headers = resp.Header.Clone()
	limit := g.maxBytes
	if r.ResponseLimit > 0 && r.ResponseLimit < limit {
		limit = r.ResponseLimit
	}
	if resp.ContentLength > limit {
		return out, ErrResponseTooLarge
	}
	if r.sink != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		out.BytesRead, e = io.Copy(r.sink, io.LimitReader(resp.Body, limit))
		if e != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			return out, errors.New("上游响应流读取或写入失败")
		}
		var probe [1]byte
		n, readErr := io.ReadFull(resp.Body, probe[:])
		if n > 0 {
			return out, ErrResponseTooLarge
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return out, errors.New("上游响应流未完整结束")
		}
		return out, nil
	}
	out.Body, e = io.ReadAll(io.LimitReader(resp.Body, limit+1))
	out.BytesRead = int64(len(out.Body))
	if e != nil {
		return out, &TransportError{Service: r.Service, Operation: r.Operation, Cause: e}
	}
	if int64(len(out.Body)) > limit {
		out.Body = nil
		return out, ErrResponseTooLarge
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, &StatusError{Service: r.Service, Operation: r.Operation, StatusCode: resp.StatusCode}
	}
	return out, nil
}

// DoTo 将成功响应直接写入调用方 Writer，不在内存中保留整个文件。
// 流式响应从不自动重试；发生错误时 Writer 可能已包含部分数据，调用方应使用临时文件并在成功后原子改名。
func (g *Gateway) DoTo(ctx context.Context, r Request, destination io.Writer) (Response, error) {
	if destination == nil {
		return Response{}, errors.New("响应目标不能为空")
	}
	r.sink = destination
	r.Idempotent = false
	return g.Do(ctx, r)
}

// Close 可重复调用；取消在途/排队请求，并释放网关自己创建的空闲连接。
func (g *Gateway) Close() {
	if g.closed.CompareAndSwap(false, true) {
		g.cancel()
		if g.owned != nil {
			g.owned.CloseIdleConnections()
		}
	}
}

// Snapshot 返回固定预算下的资源统计，不泄漏请求正文、来源地址或账号信息。
func (g *Gateway) Snapshot() Stats {
	out := Stats{Completed: g.completed.Load(), Failed: g.failed.Load(), Rejected: g.rejected.Load(), Retries: g.retried.Load()}
	for _, p := range g.pools {
		out.Running += p.running.Load()
		out.Queued += p.queued.Load()
	}
	return out
}
