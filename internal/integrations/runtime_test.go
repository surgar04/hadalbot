package integrations

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/surgar04/hadalbot/internal/httpserver"
	"github.com/surgar04/hadalbot/internal/modules"
	"github.com/surgar04/hadalbot/internal/testutil"
)

type disabledChecks struct{}

func (disabledChecks) Check(context.Context) map[string]string {
	return map[string]string{"postgres": "disabled", "redis": "disabled"}
}
func TestDefaultRuntimeDoesNotStartUpstreams(t *testing.T) {
	cfg := testutil.Config(t)
	r, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(context.Background())
	if r.Gateway != nil || r.Events != nil || r.WebSocket != nil || r.Mihoyo != nil || r.Framework != nil {
		t.Fatal("默认配置不应创建上游资源")
	}
}
func TestCallbackAndDiagnosticsAreWiredWithoutLegacyAPIs(t *testing.T) {
	cfg := testutil.Config(t)
	cfg.HTTP.APIKey = "console-key"
	cfg.Qianxun.Enabled = true
	cfg.Qianxun.BaseURL = "http://127.0.0.1:1"
	cfg.Qianxun.CallbackEnabled = true
	cfg.Qianxun.CallbackToken = strings.Repeat("c", 32)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r, err := New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(context.Background()) })
	registry := modules.NewRegistry()
	if err = registry.Register(r); err != nil {
		t.Fatal(err)
	}
	state := &httpserver.State{}
	state.Ready.Store(true)
	router := httpserver.New(cfg, logger, disabledChecks{}, state, registry, r.MountPublic)
	callback := httptest.NewRequest("POST", "/callbacks/qianxun?token="+cfg.Qianxun.CallbackToken, strings.NewReader(`{"event":10}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, callback)
	if response.Code != 200 {
		t.Fatalf("回调没有接入: %d", response.Code)
	}
	probe := httptest.NewRequest("GET", "/api/v1/upstreams", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, probe)
	if response.Code != 401 {
		t.Fatal("诊断接口必须经过控制台鉴权")
	}
	probe.Header.Set("X-API-Key", cfg.HTTP.APIKey)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, probe)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"qianxun_http_contracts":99`) {
		t.Fatal("诊断内容错误")
	}
	for _, path := range []string{"/api/v1/qx/sendText", "/wxid", "/api/v1/meta/config"} {
		response = httptest.NewRecorder()
		legacy := httptest.NewRequest("GET", path, nil)
		legacy.Header.Set("X-API-Key", cfg.HTTP.APIKey)
		router.ServeHTTP(response, legacy)
		if response.Code != 404 {
			t.Fatalf("旧HTTP路由不应被重新开放: %s status=%d", path, response.Code)
		}
	}
	if err = r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), cfg.Qianxun.CallbackToken) || strings.Contains(logs.String(), cfg.HTTP.APIKey) {
		t.Fatal("访问日志泄露了鉴权材料")
	}
}
