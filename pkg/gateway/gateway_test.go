package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testGateway(t *testing.T, server *httptest.Server, limit, queue, retries int) *Gateway {
	t.Helper()
	g, e := New(Options{Services: map[string]Limits{"test": {Concurrency: limit, QueueSize: queue, Origins: []string{server.URL}}}, Timeout: 2 * time.Second, MaxResponseBytes: 1024, MaxRetries: retries})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(g.Close)
	return g
}
func request(server *httptest.Server) Request {
	u, _ := url.Parse(server.URL)
	return Request{Service: "test", Operation: "fixture", Method: "GET", URL: u, Idempotent: true}
}
func TestBoundedQueueCancellationAndClose(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	defer close(release)
	g := testGateway(t, server, 1, 1, 0)
	first := make(chan error, 1)
	go func() { _, e := g.Do(context.Background(), request(server)); first <- e }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() { _, e := g.Do(ctx, request(server)); second <- e }()
	deadline := time.Now().Add(time.Second)
	for g.Snapshot().Queued != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if g.Snapshot().Queued != 1 {
		t.Fatal("等待请求未进入固定队列")
	}
	if _, e := g.Do(context.Background(), request(server)); !errors.Is(e, ErrBusy) {
		t.Fatalf("满队列没有背压: %v", e)
	}
	cancel()
	if e := <-second; !errors.Is(e, context.Canceled) {
		t.Fatalf("等待未取消: %v", e)
	}
	g.Close()
	if e := <-first; !errors.Is(e, context.Canceled) {
		t.Fatalf("在途请求未取消: %v", e)
	}
	if _, e := g.Do(context.Background(), request(server)); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestOnlyIdempotentRequestsRetryAndResponseIsBounded(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	g := testGateway(t, server, 2, 2, 1)
	response, e := g.Do(context.Background(), request(server))
	if e != nil || response.Attempts != 2 {
		t.Fatalf("只读重试失败: %v %+v", e, response)
	}
	calls.Store(0)
	r := request(server)
	r.Method = "POST"
	r.Idempotent = false
	_, e = g.Do(context.Background(), r)
	var status *StatusError
	if !errors.As(e, &status) || calls.Load() != 1 {
		t.Fatalf("写请求不应重试: calls=%d err=%v", calls.Load(), e)
	}
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", 2048)) }))
	defer big.Close()
	limited := testGateway(t, big, 1, 0, 0)
	if _, e = limited.Do(context.Background(), request(big)); !errors.Is(e, ErrResponseTooLarge) {
		t.Fatalf("未限制响应大小: %v", e)
	}
}
func TestOriginRedirectAndStreaming(t *testing.T) {
	var destinationCalls atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationCalls.Add(1) }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, destination.URL, http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "streamed")
	}))
	defer source.Close()
	g := testGateway(t, source, 1, 0, 0)
	r := request(source)
	r.URL.Path = "/redirect"
	if _, e := g.Do(context.Background(), r); e == nil || destinationCalls.Load() != 0 {
		t.Fatal("不应跟随重定向")
	}
	r = request(destination)
	if _, e := g.Do(context.Background(), r); !errors.Is(e, ErrOrigin) {
		t.Fatal("来源限制失效")
	}
	var out strings.Builder
	response, e := g.DoTo(context.Background(), request(source), &out)
	if e != nil || out.String() != "streamed" || len(response.Body) != 0 || response.BytesRead != 8 {
		t.Fatalf("流式响应失败: %v", e)
	}
}
