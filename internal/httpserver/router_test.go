package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/surgar04/hadalbot/internal/modules"
	"github.com/surgar04/hadalbot/internal/testutil"
)

type checker map[string]string

func (c checker) Check(context.Context) map[string]string { return c }

type testModule struct{}

func (testModule) ID() string { return "test" }
func (testModule) Mount(r chi.Router) {
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("secret must not leak") })
	r.Post("/echo", func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			Error(w, r, 413, "too_large", "请求体过大")
			return
		}
		JSON(w, 200, map[string]string{"status": "ok"})
	})
}
func handler(t *testing.T, status checker, ready bool) http.Handler {
	t.Helper()
	c := testutil.Config(t)
	c.HTTP.APIKey = "test-key"
	c.HTTP.MaxBodyBytes = 8
	registry := modules.NewRegistry()
	if err := registry.Register(testModule{}); err != nil {
		t.Fatal(err)
	}
	s := &State{}
	s.Ready.Store(ready)
	return New(c, slog.New(slog.NewTextHandler(io.Discard, nil)), status, s, registry)
}
func TestReadinessAndLiveness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ready  bool
		status checker
		want   int
	}{{"starting", false, checker{}, 503}, {"no optional services", true, checker{"postgres": "disabled"}, 200}, {"dependency failed", true, checker{"postgres": "unavailable"}, 503}} {
		t.Run(tc.name, func(t *testing.T) {
			h := handler(t, tc.status, tc.ready)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			w = httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
		})
	}
}
func TestAuthAndErrors(t *testing.T) {
	h := handler(t, checker{}, true)
	for _, tc := range []struct {
		method, path, key string
		want              int
	}{{"GET", "/api/v1/system", "", 401}, {"GET", "/api/v1/system", "incorrect", 401}, {"GET", "/api/v1/system", "test-key", 200}, {"GET", "/metrics", "", 401}, {"GET", "/metrics", "test-key", 200}, {"GET", "/missing", "", 404}, {"POST", "/healthz", "", 405}, {"GET", "/api/v1/panic", "test-key", 500}} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.key != "" {
			r.Header.Set("Authorization", "Bearer "+tc.key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, w.Code, tc.want)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("缺少 request ID")
		}
		if strings.Contains(w.Body.String(), "secret must not leak") {
			t.Fatal("panic 内容泄漏")
		}
		if tc.path != "/metrics" {
			var body any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestBodyLimit(t *testing.T) {
	h := handler(t, checker{}, true)
	r := httptest.NewRequest("POST", "/api/v1/echo", strings.NewReader("0123456789"))
	r.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatalf("got %d", w.Code)
	}
}
