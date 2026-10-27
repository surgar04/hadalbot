package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/modules"
	"github.com/surgar04/hadalbot/internal/observability"
	"github.com/surgar04/hadalbot/internal/version"
)

type Checker interface {
	Check(context.Context) map[string]string
}
type State struct{ Ready atomic.Bool }

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}, "request_id": middleware.GetReqID(r.Context())})
}

func New(c config.Config, log *slog.Logger, checks Checker, state *State, registry *modules.Registry, extraRoutes ...func(chi.Router)) http.Handler {
	router := chi.NewRouter()
	metrics := observability.NewMetrics()
	router.Use(middleware.RequestID, metrics.Middleware(log))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					log.ErrorContext(r.Context(), "handler panic", "request_id", middleware.GetReqID(r.Context()))
					Error(w, r, 500, "internal_error", "内部服务错误")
				}
			}()
			next.ServeHTTP(w, r)
		})
	})
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]string{"status": "ok"}) })
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !state.Ready.Load() {
			JSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, status := range checks.Check(ctx) {
			if status != "ok" && status != "disabled" {
				JSON(w, 503, map[string]string{"status": "unavailable"})
				return
			}
		}
		JSON(w, 200, map[string]string{"status": "ready"})
	})
	router.Group(func(r chi.Router) {
		r.Use(Auth(c.HTTP.APIKey))
		r.Handle("/metrics", metrics.Handler())
		r.Route("/api/v1", func(api chi.Router) {
			api.Get("/system", func(w http.ResponseWriter, r *http.Request) {
				JSON(w, 200, map[string]any{"name": "HadalBot-Go", "version": version.Version, "commit": version.Commit, "built_at": version.BuiltAt, "stage": "foundation", "modules": registry.IDs()})
			})
			registry.Mount(api)
		})
	})
	for _, mount := range extraRoutes {
		if mount != nil {
			mount(router)
		}
	}
	router.NotFound(func(w http.ResponseWriter, r *http.Request) { Error(w, r, 404, "not_found", "接口不存在") })
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		Error(w, r, 405, "method_not_allowed", "请求方法不支持")
	})
	return http.MaxBytesHandler(router, c.HTTP.MaxBodyBytes)
}

// Auth 同时接受 X-API-Key 和 Bearer，未配置密钥仅允许经过配置校验的本地开发模式。
func Auth(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key != "" {
				provided := r.Header.Get("X-API-Key")
				if provided == "" {
					auth := r.Header.Get("Authorization")
					if len(auth) > 7 && auth[:7] == "Bearer " {
						provided = auth[7:]
					}
				}
				if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
					Error(w, r, 401, "unauthorized", "认证失败")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
