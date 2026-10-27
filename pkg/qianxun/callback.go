package qianxun

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// CallbackOptions 描述回调入口，与上游safekey分离，避免复用同一密钥。
type CallbackOptions struct {
	Token        string
	MaxBodyBytes int64
	WXID         string
	MaxInFlight  int
}

// NewCallbackHandler 返回可挂入任意net/http路由的处理器。Header优先于query token；
// 查询参数用于兼容只能配置URL的官方客户端，访问日志必须排除查询串。
func NewCallbackHandler(bus *EventBus, o CallbackOptions) (http.Handler, error) {
	if bus == nil || len(o.Token) < 32 {
		return nil, errors.New("回调必须配置事件总线和至少32字符的独立token")
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = 512 << 10
	}
	if o.MaxBodyBytes < 1 || o.MaxBodyBytes > 2<<20 {
		return nil, errors.New("回调请求体上限无效")
	}
	expected := sha256.Sum256([]byte(o.Token))
	if o.MaxInFlight == 0 {
		o.MaxInFlight = 16
	}
	if o.MaxInFlight < 1 || o.MaxInFlight > 128 {
		return nil, errors.New("回调解析并发预算无效")
	}
	parsing := make(chan struct{}, o.MaxInFlight)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		send := func(status int, ok bool) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": ok})
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			send(405, false)
			return
		}
		supplied := r.Header.Get("X-QX-Callback-Token")
		if supplied == "" {
			supplied = r.URL.Query().Get("token")
		}
		actual := sha256.Sum256([]byte(supplied))
		if r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			send(401, false)
			return
		}
		select {
		case parsing <- struct{}{}:
			defer func() { <-parsing }()
		default:
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Connection", "close")
			send(503, false)
			return
		}
		body := http.MaxBytesReader(w, r.Body, o.MaxBodyBytes)
		defer body.Close()
		raw, err := io.ReadAll(body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				send(413, false)
			} else {
				send(400, false)
			}
			return
		}
		event, err := DecodeEvent(raw)
		if err != nil {
			send(400, false)
			return
		}
		if o.WXID != "" && event.WXID != "" && event.WXID != o.WXID {
			send(403, false)
			return
		}
		if _, err = bus.TryPublish(event); err != nil {
			w.Header().Set("Retry-After", "1")
			send(503, false)
			return
		}
		send(200, true)
	}), nil
}
