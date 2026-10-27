package qianxun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

var ErrWSDisconnected = errors.New("千寻WebSocket未连接或已断开，请勿自动重放写操作")
var ErrWSPendingFull = errors.New("千寻WebSocket等待响应数量已达上限")

// WebSocketOptions 为长连接设置固定pending和帧大小预算。URL必须由部署配置明确提供。
type WebSocketOptions struct {
	URL              string
	SafeKey          string
	MaxPending       int
	ReadLimit        int64
	RequestTimeout   time.Duration
	HeartbeatTimeout time.Duration
	DialTimeout      time.Duration
	NoReconnect      bool
	// Events 接收所有回调。满载时连接显式失败，不静默丢弃后假装成功。
	Events *EventBus
}
type wsResult struct {
	body json.RawMessage
	err  error
}

// WebSocket 既可接收事件，也可作为强类型客户端的RPCTransport。
// Run拥有唯一读取循环；调用方不得为同一个实例重复运行Run。
type WebSocket struct {
	options    WebSocketOptions
	mu         sync.Mutex
	conn       *websocket.Conn
	pending    map[string]chan wsResult
	ctx        context.Context
	cancel     context.CancelFunc
	running    atomic.Bool
	closed     atomic.Bool
	reconnects atomic.Uint64
}

// NewWebSocket 不建立网络连接，连接与自动重连在Run的生命周期内发生。
func NewWebSocket(o WebSocketOptions) (*WebSocket, error) {
	u, e := url.Parse(o.URL)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "ws" && u.Scheme != "wss") {
		return nil, errors.New("千寻WebSocket地址无效")
	}
	if o.MaxPending == 0 {
		o.MaxPending = 32
	}
	if o.ReadLimit == 0 {
		o.ReadLimit = 2 << 20
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 30 * time.Second
	}
	if o.HeartbeatTimeout == 0 {
		o.HeartbeatTimeout = 45 * time.Second
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = 5 * time.Second
	}
	if o.Events == nil || o.MaxPending < 1 || o.MaxPending > 1024 || o.ReadLimit < 1 || o.ReadLimit > 8<<20 || o.RequestTimeout <= 0 || o.HeartbeatTimeout <= 0 || o.DialTimeout <= 0 {
		return nil, errors.New("千寻WebSocket预算或事件总线配置无效")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocket{options: o, pending: make(map[string]chan wsResult), ctx: ctx, cancel: cancel}, nil
}

// Run 持续接收上游事件并在断线后有界退避重连。断线立即失败所有pending，绝不在新连接重发请求。
// 框架WebSocket没有事件确认/重放协议，因此不能承诺断线期间的可靠消息交付；需要重试语义时优先HTTP回调。
func (w *WebSocket) Run(parent context.Context) error {
	if w.closed.Load() {
		return ErrWSDisconnected
	}
	if !w.running.CompareAndSwap(false, true) {
		return errors.New("同一WebSocket实例不能重复运行")
	}
	defer w.running.Store(false)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(w.ctx, cancel)
	defer stop()
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		dialCtx, stopDial := context.WithTimeout(ctx, w.options.DialTimeout)
		u, _ := url.Parse(w.options.URL)
		q := u.Query()
		if w.options.SafeKey != "" {
			q.Set("safekey", w.options.SafeKey)
		}
		u.RawQuery = q.Encode()
		// 不跟随握手重定向，防止URL查询密钥跨域泄漏。
		conn, _, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{HTTPClient: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}})
		stopDial()
		connectionError := ErrWSDisconnected
		if err == nil {
			conn.SetReadLimit(w.options.ReadLimit)
			w.mu.Lock()
			w.conn = conn
			w.mu.Unlock()
			attempt = 0
			connectionError = w.read(ctx, conn)
			_ = conn.CloseNow()
			w.mu.Lock()
			if w.conn == conn {
				w.conn = nil
			}
			for id, ch := range w.pending {
				delete(w.pending, id)
				ch <- wsResult{err: ErrWSDisconnected}
			}
			w.mu.Unlock()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if w.options.NoReconnect {
			return connectionError
		}
		// 失败后最多5秒退避，不创建重连goroutine；取消会立即释放timer。
		attempt++
		shift := attempt
		if shift > 5 {
			shift = 5
		}
		delay := time.Duration(1<<shift) * 100 * time.Millisecond
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
		w.reconnects.Add(1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (w *WebSocket) read(ctx context.Context, conn *websocket.Conn) error {
	for {
		readCtx, cancel := context.WithTimeout(ctx, w.options.HeartbeatTimeout)
		_, raw, e := conn.Read(readCtx)
		cancel()
		if e != nil {
			return ErrWSDisconnected
		}
		var envelope struct {
			RequestID string          `json:"requestId"`
			Code      *int            `json:"code"`
			Event     json.RawMessage `json:"event"`
			Type      json.RawMessage `json:"type"`
		}
		if json.Unmarshal(raw, &envelope) != nil {
			return ErrInvalidResponse
		}
		if envelope.Code != nil && envelope.RequestID != "" {
			w.mu.Lock()
			ch := w.pending[envelope.RequestID]
			if ch != nil {
				delete(w.pending, envelope.RequestID)
				ch <- wsResult{body: raw}
			}
			w.mu.Unlock()
			continue
		}
		if len(envelope.Event) > 0 || len(envelope.Type) > 0 {
			event, e := DecodeEvent(raw)
			if e != nil {
				return e
			}
			if event.ID == Heartbeat {
				continue
			}
			if _, e = w.options.Events.TryPublish(event); e != nil {
				return e
			}
		}
	}
}

// RoundTrip 实现RPCTransport，pending有固定上限。取消/超时会移除关联项，迟到响应不会误配其他调用。
func (w *WebSocket) RoundTrip(parent context.Context, raw []byte, wxid string) (json.RawMessage, error) {
	if w.closed.Load() {
		return nil, ErrWSDisconnected
	}
	ctx, cancel := context.WithTimeout(parent, w.options.RequestTimeout)
	defer cancel()
	stop := context.AfterFunc(w.ctx, cancel)
	defer stop()
	var frame map[string]json.RawMessage
	if len(raw) > 2<<20 || json.Unmarshal(raw, &frame) != nil || frame == nil {
		return nil, errors.New("WebSocket RPC请求不是有界JSON对象")
	}
	idBytes := make([]byte, 16)
	if _, e := rand.Read(idBytes); e != nil {
		return nil, e
	}
	id := hex.EncodeToString(idBytes)
	frame["requestId"], _ = json.Marshal(id)
	if wxid != "" {
		frame["wxid"], _ = json.Marshal(wxid)
	}
	if w.options.SafeKey != "" {
		frame["safekey"], _ = json.Marshal(w.options.SafeKey)
	}
	body, e := json.Marshal(frame)
	if e != nil {
		return nil, errors.New("WebSocket RPC无法编码")
	}
	result := make(chan wsResult, 1)
	w.mu.Lock()
	conn := w.conn
	if conn == nil {
		w.mu.Unlock()
		return nil, ErrWSDisconnected
	}
	if len(w.pending) >= w.options.MaxPending {
		w.mu.Unlock()
		return nil, ErrWSPendingFull
	}
	w.pending[id] = result
	w.mu.Unlock()
	defer func() { w.mu.Lock(); delete(w.pending, id); w.mu.Unlock() }()
	if e = conn.Write(ctx, websocket.MessageText, body); e != nil {
		return nil, ErrWSDisconnected
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case value := <-result:
		return value.body, value.err
	}
}

// Close 取消连接、重连等待和在途RPC；实例关闭后不能复用。
func (w *WebSocket) Close() {
	if w.closed.CompareAndSwap(false, true) {
		w.cancel()
		w.mu.Lock()
		conn := w.conn
		w.mu.Unlock()
		if conn != nil {
			_ = conn.CloseNow()
		}
	}
}

// WebSocketStats 不包含URL、密钥、账号或响应内容。
type WebSocketStats struct {
	Connected  bool   `json:"connected"`
	Pending    int    `json:"pending"`
	Reconnects uint64 `json:"reconnects"`
}

func (w *WebSocket) Snapshot() WebSocketStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return WebSocketStats{Connected: w.conn != nil, Pending: len(w.pending), Reconnects: w.reconnects.Load()}
}
