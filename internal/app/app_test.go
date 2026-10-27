package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/surgar04/hadalbot/internal/testutil"
)

func TestServerLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	cfg := testutil.Config(t)
	cfg.HTTP.Address = addr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	client := &http.Client{Timeout: 200 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, e := client.Get("http://" + addr + "/readyz")
		if e == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("服务未就绪")
	}
	cancel()
	select {
	case err = <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("优雅退出超时")
	}
}
