package observability

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Logger(level, format string, w io.Writer) *slog.Logger {
	l := slog.LevelInfo
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: l}
	if format == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	m := &Metrics{registry: prometheus.NewRegistry(), requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hadalbot_http_requests_total", Help: "HTTP requests by registered route."}, []string{"method", "route", "status"}), duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "hadalbot_http_request_duration_seconds", Help: "HTTP request latency.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})}
	m.registry.MustRegister(m.requests, m.duration, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
func (m *Metrics) Middleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}
			method := r.Method
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
			default:
				method = "OTHER"
			}
			m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
			m.duration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
			// 不记录 URL 查询串、请求体、Authorization 或 Cookie，避免令牌进入日志。
			log.InfoContext(r.Context(), "http request", "request_id", middleware.GetReqID(r.Context()), "method", method, "route", route, "status", status, "duration", time.Since(started))
		})
	}
}
