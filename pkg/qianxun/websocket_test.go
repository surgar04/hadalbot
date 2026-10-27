package qianxun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWebSocketCorrelationEventsAndDisconnect(t *testing.T) {
	bus, e := NewEventBus(BusOptions{Shards: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer bus.Close(context.Background())
	events := make(chan Event, 1)
	if _, e = bus.Subscribe(EventAny, func(_ context.Context, e Event) error { events <- e; return nil }); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if e = conn.Write(ctx, websocket.MessageText, []byte(`{"event":10000,"wxid":"","data":{"type":"injectSuccess","data":{"port":"8888","pid":"1"}}}`)); e != nil {
			return
		}
		frames := make([]map[string]json.RawMessage, 0, 2)
		for range 2 {
			_, raw, e := conn.Read(ctx)
			if e != nil {
				return
			}
			var frame map[string]json.RawMessage
			if json.Unmarshal(raw, &frame) != nil {
				return
			}
			frames = append(frames, frame)
		}
		for i := len(frames) - 1; i >= 0; i-- {
			frame := frames[i]
			response, _ := json.Marshal(map[string]any{"code": 200, "msg": "OK", "requestId": frame["requestId"], "result": frame["data"]})
			if e = conn.Write(ctx, websocket.MessageText, response); e != nil {
				return
			}
		}
		// 第三个请求在上游断开后失败，验证客户端不会重连重放发送操作。
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()
	socket, e := NewWebSocket(WebSocketOptions{URL: strings.Replace(server.URL, "http", "ws", 1), SafeKey: "secret", Events: bus, NoReconnect: true, RequestTimeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	defer socket.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- socket.Run(ctx) }()
	select {
	case got := <-events:
		if got.ID != InjectSuccess || got.WXID != "" {
			t.Fatal(got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("未收到WebSocket事件")
	}
	results := make(chan string, 2)
	for _, value := range []string{"one", "two"} {
		go func(value string) {
			body := []byte(`{"type":"sendText","data":{"msg":"` + value + `"}}`)
			raw, e := socket.RoundTrip(context.Background(), body, "bot")
			if e != nil {
				results <- "error"
				return
			}
			var reply struct {
				Result struct {
					Msg string `json:"msg"`
				} `json:"result"`
			}
			_ = json.Unmarshal(raw, &reply)
			if reply.Result.Msg != value {
				results <- "mismatch"
			} else {
				results <- value
			}
		}(value)
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case value := <-results:
			seen[value] = true
		case <-time.After(2 * time.Second):
			t.Fatal("请求未完成")
		}
	}
	if !seen["one"] || !seen["two"] {
		t.Fatalf("响应关联错误: %v", seen)
	}
	if _, e = socket.RoundTrip(context.Background(), []byte(`{"type":"sendText","data":{}}`), "bot"); !errors.Is(e, ErrWSDisconnected) {
		t.Fatalf("断开后错误应明确: %v", e)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("连接循环未退出")
	}
	if socket.Snapshot().Pending != 0 {
		t.Fatal("断开后存在泄漏的pending")
	}
}
