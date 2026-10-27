package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/surgar04/hadalbot/pkg/qianxun"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type probe struct {
	meta    Manifest
	init    func(context.Context, *Context) error
	ready   func(context.Context, *Context) error
	observe func() error
	event   func() (bool, error)
	message func(context.Context, *Context) (bool, error)
	dispose func() error
}

func (p *probe) Manifest() Manifest { return p.meta }
func (p *probe) Init(c context.Context, pc *Context) error {
	if p.init != nil {
		return p.init(c, pc)
	}
	return nil
}
func (p *probe) Ready(c context.Context, pc *Context) error {
	if p.ready != nil {
		return p.ready(c, pc)
	}
	return nil
}
func (p *probe) ObserveMessage(context.Context, Message, *Context) error {
	if p.observe != nil {
		return p.observe()
	}
	return nil
}
func (p *probe) OnEvent(context.Context, qianxun.Event, *Context) (bool, error) {
	if p.event != nil {
		return p.event()
	}
	return false, nil
}
func (p *probe) OnMessage(c context.Context, _ Message, pc *Context) (bool, error) {
	if p.message != nil {
		return p.message(c, pc)
	}
	return false, nil
}
func (p *probe) Dispose(context.Context) error {
	if p.dispose != nil {
		return p.dispose()
	}
	return nil
}
func metadata(id string) Manifest { return Manifest{ID: id, Version: "1.2.3"} }
func manager(t *testing.T, o Options) *Manager {
	t.Helper()
	o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	m, e := New(o)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = m.Close(ctx)
	})
	return m
}
func reg(t *testing.T, m *Manager, p *probe, enabled bool) {
	t.Helper()
	if e := m.Register(p.meta, func() Plugin { return p }, nil, enabled); e != nil {
		t.Fatal(e)
	}
}
func messageEvent(t *testing.T) qianxun.Event {
	t.Helper()
	e, err := qianxun.DecodeEvent([]byte(`{"event":10008,"wxid":"bot","data":{"type":"recvMsg","wxid":"bot","data":{"fromWxid":"group","finalFromWxid":"sender","msgId":"123","msg":"ping","msgType":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestLifecycleDependencyRollbackAndRecreation(t *testing.T) {
	m := manager(t, Options{})
	var order []string
	a := &probe{meta: metadata("a"), init: func(context.Context, *Context) error { order = append(order, "init-a"); return nil }, dispose: func() error { order = append(order, "stop-a"); return nil }}
	b := &probe{meta: metadata("b"), init: func(context.Context, *Context) error {
		order = append(order, "init-b")
		return errors.New("secret-password")
	}, dispose: func() error { order = append(order, "stop-b"); return nil }}
	b.meta.Dependencies = map[string]string{"a": "^1.0.0"}
	b.meta.Priority = 100
	reg(t, m, b, true)
	reg(t, m, a, true)
	if e := m.Start(context.Background()); e == nil || strings.Contains(e.Error(), "secret-password") {
		t.Fatalf("启动应失败且脱敏: %v", e)
	}
	if !reflect.DeepEqual(order, []string{"init-a", "init-b", "stop-b", "stop-a"}) {
		t.Fatal(order)
	}
	n := 0
	restarting := manager(t, Options{})
	meta := metadata("fresh")
	if e := restarting.Register(meta, func() Plugin { n++; return &probe{meta: meta} }, nil, true); e != nil {
		t.Fatal(e)
	}
	if e := restarting.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := restarting.Disable(context.Background(), "fresh"); e != nil {
		t.Fatal(e)
	}
	if e := restarting.Enable(context.Background(), "fresh"); e != nil || n != 2 {
		t.Fatalf("没有重建实例 n=%d err=%v", n, e)
	}
}
func TestGraphAndVersionRules(t *testing.T) {
	for _, cases := range []struct {
		v, r string
		want bool
	}{{"1.2.3", "^1.0.0", true}, {"2.0.0", "^1.0.0", false}, {"0.2.9", "^0.2.1", true}, {"0.3.0", "^0.2.1", false}, {"1.3.0", "~1.2.3", false}, {"2.0.0", ">=1.2.3", true}} {
		if matches(cases.v, cases.r) != cases.want {
			t.Fatal(cases)
		}
	}
	for _, rule := range []string{">=^1.0.0", "1.2", "v1.2.3"} {
		if validRule(rule) {
			t.Fatal(rule)
		}
	}
	for _, kind := range []string{"missing", "cycle", "version"} {
		t.Run(kind, func(t *testing.T) {
			m := manager(t, Options{})
			a := &probe{meta: metadata("a")}
			a.meta.Dependencies = map[string]string{"b": "*"}
			reg(t, m, a, true)
			if kind != "missing" {
				b := &probe{meta: metadata("b")}
				if kind == "cycle" {
					b.meta.Dependencies = map[string]string{"a": "*"}
				} else {
					a2 := m.entries["a"]
					a2.manifest.Dependencies["b"] = "2.0.0"
				}
				reg(t, m, b, true)
			}
			if e := m.Start(context.Background()); e == nil {
				t.Fatal("图错误未拒绝")
			}
		})
	}
}
func TestObservationBeforeInterceptionAndFailureIsolation(t *testing.T) {
	m := manager(t, Options{})
	var trace []string
	a := &probe{meta: metadata("a"), observe: func() error { trace = append(trace, "observe-a"); panic("secret") }, event: func() (bool, error) { trace = append(trace, "event-a"); return true, nil }}
	b := &probe{meta: metadata("b"), observe: func() error { trace = append(trace, "observe-b"); return nil }, event: func() (bool, error) { t.Fatal("不应越过拦截"); return false, nil }}
	reg(t, m, a, true)
	reg(t, m, b, true)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := m.Dispatch(context.Background(), messageEvent(t)); !errors.Is(e, ErrHook) {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(trace, []string{"observe-a", "observe-b", "event-a"}) {
		t.Fatal(trace)
	}
}
func TestBudgetDrainAndCancellation(t *testing.T) {
	m := manager(t, Options{})
	entered := make(chan struct{})
	release := make(chan struct{})
	p := &probe{meta: metadata("block"), message: func(context.Context, *Context) (bool, error) { close(entered); <-release; return true, nil }}
	reg(t, m, p, true)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	event := messageEvent(t)
	done := make(chan error, 1)
	go func() { done <- m.Dispatch(context.Background(), event) }()
	<-entered
	if e := m.Dispatch(context.Background(), event); !errors.Is(e, ErrBusy) {
		t.Fatalf("应明确背压: %v", e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := m.Disable(ctx, "block"); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	if m.Status()[0].State != Failed {
		t.Fatal(m.Status())
	}
	if e := m.Enable(context.Background(), "block"); !errors.Is(e, ErrBusy) {
		t.Fatalf("旧调用仍在执行时不能替换: %v", e)
	}
	close(release)
	<-done
	if e := m.Disable(context.Background(), "block"); e != nil {
		t.Fatal(e)
	}
}
func TestCapabilitiesCopiesAndAccountContext(t *testing.T) {
	var called int
	m := manager(t, Options{Caller: CallFunc(func(ctx context.Context, id, method string, args json.RawMessage) (json.RawMessage, error) {
		if id != "cap" || method != "qx.sendText" || AccountWXID(ctx) != "bot" {
			t.Fatal("身份或账号串号")
		}
		called++
		return json.RawMessage(`{}`), nil
	})})
	meta := metadata("cap")
	meta.Capabilities = []string{"qx.sendText"}
	p := &probe{meta: meta, message: func(ctx context.Context, c *Context) (bool, error) {
		if _, e := c.Call(ctx, "qx.delete", json.RawMessage(`{}`)); !errors.Is(e, ErrDenied) {
			t.Fatal(e)
		}
		_, e := c.Call(ctx, "qx.sendText", json.RawMessage(`{}`))
		return true, e
	}}
	reg(t, m, p, true)
	meta.Capabilities[0] = "qx.delete"
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := m.Dispatch(context.Background(), messageEvent(t)); e != nil || called != 1 {
		t.Fatalf("cap=%d err=%v", called, e)
	}
}
func TestScopeBoundsCleanupAndConcurrentControl(t *testing.T) {
	m := manager(t, Options{})
	var pc *Context
	var mu sync.Mutex
	var cleanups []int
	p := &probe{meta: metadata("scope"), init: func(_ context.Context, c *Context) error {
		pc = c
		for i := 0; i < 2; i++ {
			j := i
			if e := c.Scope.Defer(func(context.Context) error { mu.Lock(); defer mu.Unlock(); cleanups = append(cleanups, j); return nil }); e != nil {
				return e
			}
		}
		return c.Scope.Go(func(ctx context.Context) { <-ctx.Done() })
	}}
	p.meta.Limits.MaxTasks = 1
	reg(t, m, p, true)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := pc.Scope.Go(func(context.Context) {}); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if e := m.Disable(context.Background(), "scope"); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(cleanups, []int{1, 0}) {
		t.Fatal(cleanups)
	}
	if e := pc.Scope.Go(func(context.Context) {}); !errors.Is(e, ErrNotReady) {
		t.Fatal(e)
	}
	if e := m.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := m.Dispatch(context.Background(), messageEvent(t)); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func BenchmarkNativeDispatch(b *testing.B) {
	m, _ := New(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	meta := metadata("bench")
	_ = m.Register(meta, func() Plugin { return &probe{meta: meta} }, nil, true)
	_ = m.Start(context.Background())
	defer func() { _ = m.Close(context.Background()) }()
	event := qianxun.Event{ID: qianxun.InjectSuccess}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := m.Dispatch(context.Background(), event); err != nil {
			b.Fatal(fmt.Sprint(err))
		}
	}
}
