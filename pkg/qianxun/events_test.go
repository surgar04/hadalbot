package qianxun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOfficialEventExamplesAndKnownErratum(t *testing.T) {
	fence := regexp.MustCompile("(?s)```json\\s*(.*?)```")
	valid, invalid := 0, 0
	for _, raw := range loadContracts().Events {
		var p struct {
			Group    string `json:"group"`
			Name     string `json:"name"`
			Resource struct {
				Data struct {
					Content string `json:"content"`
				} `json:"data"`
			} `json:"resource"`
		}
		if json.Unmarshal(raw, &p) != nil {
			t.Fatal("事件契约损坏")
		}
		for _, m := range fence.FindAllStringSubmatch(p.Resource.Data.Content, -1) {
			body := []byte(m[1])
			if !json.Valid(body) {
				known := (strings.Contains(p.Name, "授权到期") && strings.Contains(m[1], ",,")) || (strings.Contains(p.Name, "群成员变动") && regexp.MustCompile(`,\s*}`).Match(body))
				if !known {
					t.Fatalf("未记录的官方示例错误: %s", p.Name)
				}
				if _, e := DecodeEvent(body); e == nil {
					t.Fatal("原始错误JSON不应被生产解码器接受")
				}
				invalid++
				// 只在测试中为已记录的官方排版错误建立派生有效样本；原始快照不修改。
				body = bytes.ReplaceAll(body, []byte(",,"), []byte(","))
				body = regexp.MustCompile(`,\s*}`).ReplaceAll(body, []byte("}"))
			}
			var envelope map[string]json.RawMessage
			_ = json.Unmarshal(body, &envelope)
			if _, event := envelope["event"]; !event {
				var typ string
				_ = json.Unmarshal(envelope["type"], &typ)
				if typ == "sendText" {
					continue
				}
			}
			e, err := DecodeEvent(body)
			if err != nil {
				t.Fatalf("%s: %v", p.Name, err)
			}
			if !e.Known {
				t.Fatalf("官方事件未识别: %s", p.Name)
			}
			if string(e.Raw) != string(body) {
				t.Fatal("事件原始字段没有完整保留")
			}
			valid++
		}
	}
	if valid < 22 || invalid != 3 {
		t.Fatalf("事件示例覆盖异常 valid=%d known_errata=%d", valid, invalid)
	}
}
func TestEmptyInjectUnknownAndLargeNumericID(t *testing.T) {
	raw := []byte(`{"event":10000,"wxid":"","data":{"type":"injectSuccess","data":{"port":"8888","pid":123},"wxid":""}}`)
	if e, err := DecodeEvent(raw); err != nil || e.WXID != "" {
		t.Fatalf("注入前空账号应合法: %v", err)
	}
	e, err := DecodeEvent([]byte(`{"event":123456,"wxid":"bot","data":{"future_field":true}}`))
	if err != nil || e.Known || !strings.Contains(string(e.Payload), "future_field") {
		t.Fatal("未知事件应完整保留")
	}
	e, err = DecodeEvent([]byte(`{"type":"recvMsg","wxid":"bot","data":{"fromType":2,"msgSource":"0","fromWxid":"group","msgId":184467440737095516151,"msg":""}}`))
	if err != nil {
		t.Fatal(err)
	}
	m, err := PayloadAs[MessageData](e)
	if err != nil || m.MsgID != "184467440737095516151" || e.ID != GroupMessage {
		t.Fatal("长ID精度或模式归一化错误")
	}
	if _, err = DecodeEvent([]byte(`{"event":10008,"wxid":"outer","data":{"type":"recvMsg","wxid":"inner","data":{}}}`)); err == nil {
		t.Fatal("账号不一致必须拒绝")
	}
}
func eventForTest(id string) Event {
	return Event{ID: GroupMessage, WXID: "bot", Payload: json.RawMessage(`{"fromWxid":"group","msgId":"` + id + `"}`)}
}
func TestEventOrderingDedupAndBackpressure(t *testing.T) {
	b, err := NewEventBus(BusOptions{Shards: 1, QueuePerShard: 1, DedupCapacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var mu sync.Mutex
	var received []string
	_, err = b.Subscribe(EventAny, func(ctx context.Context, e Event) error {
		if len(received) == 0 {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		m, _ := PayloadAs[MessageData](e)
		mu.Lock()
		received = append(received, string(m.MsgID))
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.TryPublish(eventForTest("1")); err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err = b.TryPublish(eventForTest("2")); err != nil {
		t.Fatal(err)
	}
	if dup, e := b.TryPublish(eventForTest("2")); e != nil || !dup {
		t.Fatal("同一消息应去重")
	}
	if _, err = b.TryPublish(eventForTest("3")); !errors.Is(err, ErrEventBusy) {
		t.Fatal("满队列应拒绝")
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = b.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if strings.Join(received, ",") != "1,2" {
		t.Fatalf("会话顺序错误: %v", received)
	}
	if b.Snapshot().Duplicates != 1 || b.Snapshot().Rejected != 1 {
		t.Fatal(b.Snapshot())
	}
}
func TestCallbackAuthorizationBodyLimitAndNoSubscribers(t *testing.T) {
	b, err := NewEventBus(BusOptions{Shards: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close(context.Background())
	token := strings.Repeat("k", 32)
	h, err := NewCallbackHandler(b, CallbackOptions{Token: token, MaxBodyBytes: 128})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"event":10}`
	check := func(key, body string) int {
		r := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
		r.Header.Set("X-QX-Callback-Token", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if check("invalid", raw) != 401 || check(token, strings.Repeat("x", 129)) != 413 || check(token, raw) != 503 {
		t.Fatal("回调鉴权/上限/无订阅背压异常")
	}
	unsubscribe, err := b.Subscribe(EventAny, func(context.Context, Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if check(token, raw) != 200 {
		t.Fatal("合法回调应确认接收")
	}
	unsubscribe()
	// 已取消的槽位可重复使用，不因热注册/卸载逐渐耗尽MaxHandlers预算。
	small, err := NewEventBus(BusOptions{Shards: 1, MaxHandlers: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer small.Close(context.Background())
	stop, err := small.Subscribe(EventAny, func(context.Context, Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if _, err = small.Subscribe(EventAny, func(context.Context, Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
func BenchmarkDecodeMessage(b *testing.B) {
	raw := []byte(`{"event":10008,"wxid":"bot","data":{"type":"recvMsg","data":{"fromType":2,"msgSource":0,"fromWxid":"group","msgId":"1001","msg":"hello"}}}`)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := DecodeEvent(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEventByteBudget(t *testing.T) {
	b, err := NewEventBus(BusOptions{Shards: 1, QueuePerShard: 8, MaxBufferedBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	if _, err = b.Subscribe(EventAny, func(context.Context, Event) error { close(entered); <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	event, err := DecodeEvent([]byte(`{"event":10,"padding":"` + strings.Repeat("x", 600) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.TryPublish(event); err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err = b.TryPublish(event); !errors.Is(err, ErrEventBusy) {
		t.Error("字节预算应独立于队列数量预算生效")
	}
	close(release)
	if err = b.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.Snapshot().BufferedBytes != 0 {
		t.Fatal("事件释放后预算未归还")
	}
}
