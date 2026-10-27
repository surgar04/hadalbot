// Package integrations 在组合根装配上游SDK，生命周期不与任何一个业务插件的私有存储耦合。
package integrations

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/httpserver"
	"github.com/surgar04/hadalbot/pkg/gateway"
	"github.com/surgar04/hadalbot/pkg/mihoyo"
	"github.com/surgar04/hadalbot/pkg/qianxun"
)

// Runtime 向后续业务模块提供明确依赖；Mihoyo不含全局Cookie，必须ForAccount后调用私有接口。
type Runtime struct {
	Gateway   *gateway.Gateway
	Framework *qianxun.FrameworkClient
	Direct    *qianxun.DirectClient
	Mihoyo    *mihoyo.Client
	Events    *qianxun.EventBus
	WebSocket *qianxun.WebSocket
	callback  http.Handler
	log       *slog.Logger
	workers   sync.WaitGroup
}

// New 只装配被显式启用的组件；默认配置不建立连接、不加载账号、也不启动事件worker。
func New(c config.Config, log *slog.Logger) (r *Runtime, err error) {
	if log == nil {
		log = slog.Default()
	}
	if err = c.Validate(); err != nil {
		return nil, err
	}
	r = &Runtime{log: log}
	defer func() {
		if err != nil {
			if r.WebSocket != nil {
				r.WebSocket.Close()
			}
			if r.Events != nil {
				_ = r.Events.Close(context.Background())
			}
			if r.Gateway != nil {
				r.Gateway.Close()
			}
		}
	}()
	services := map[string]gateway.Limits{}
	if c.Qianxun.Enabled {
		u, e := url.Parse(c.Qianxun.BaseURL)
		if e != nil {
			return r, errors.New("千寻来源无法解析")
		}
		services["qianxun"] = gateway.Limits{Concurrency: c.Qianxun.Concurrency, QueueSize: c.Qianxun.QueueSize, MinInterval: c.Qianxun.MinInterval, Origins: []string{u.Scheme + "://" + u.Host}}
	}
	if c.Mihoyo.Enabled {
		services["mihoyo"] = gateway.Limits{Concurrency: c.Mihoyo.Concurrency, QueueSize: c.Mihoyo.QueueSize, MinInterval: c.Mihoyo.MinInterval, Origins: mihoyo.Origins()}
	}
	if len(services) == 0 {
		return r, nil
	}
	r.Gateway, err = gateway.New(gateway.Options{Services: services, Timeout: c.Gateway.Timeout, MaxResponseBytes: c.Gateway.MaxResponseBytes, MaxRetries: c.Gateway.MaxRetries})
	if err != nil {
		return r, err
	}
	if c.Qianxun.Enabled {
		q := c.Qianxun
		if q.CallbackEnabled || q.WebSocketEnabled {
			r.Events, err = qianxun.NewEventBus(qianxun.BusOptions{Shards: q.EventShards, QueuePerShard: q.EventQueuePerShard, OnError: func(id qianxun.EventID, err error) { log.Warn("千寻事件处理失败", "event", id) }})
			if err != nil {
				return r, err
			}
			// 当前仅确认传输层接入并记录安全元数据；后续业务模块在此总线上订阅，不记录消息正文。
			_, err = r.Events.Subscribe(qianxun.EventAny, func(ctx context.Context, event qianxun.Event) error {
				log.DebugContext(ctx, "千寻事件已分发", "event", event.ID, "mode", event.Mode, "known", event.Known)
				return nil
			})
			if err != nil {
				return r, err
			}
		}
		cfg := qianxun.Config{BaseURL: q.BaseURL, SafeKey: q.SafeKey, WXID: q.WXID}
		if q.WebSocketEnabled {
			r.WebSocket, err = qianxun.NewWebSocket(qianxun.WebSocketOptions{URL: q.WebSocketURL, SafeKey: q.SafeKey, Events: r.Events})
			if err != nil {
				return r, err
			}
			cfg.Transport = r.WebSocket
		}
		if q.Mode == "framework" {
			r.Framework, err = qianxun.NewFramework(r.Gateway, cfg)
		} else {
			r.Direct, err = qianxun.NewDirect(r.Gateway, cfg)
		}
		if err != nil {
			return r, err
		}
		if q.CallbackEnabled {
			r.callback, err = qianxun.NewCallbackHandler(r.Events, qianxun.CallbackOptions{Token: q.CallbackToken, WXID: q.WXID, MaxBodyBytes: q.CallbackMaxBodyBytes})
			if err != nil {
				return r, err
			}
		}
	}
	if c.Mihoyo.Enabled {
		r.Mihoyo, err = mihoyo.New(r.Gateway, mihoyo.Options{})
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

// Start 仅在明确配置WebSocket时建立连接；它由应用总context取消，不创建独立后台守护服务。
func (r *Runtime) Start(ctx context.Context) {
	if r.WebSocket != nil {
		r.workers.Add(1)
		go func() {
			defer r.workers.Done()
			if err := r.WebSocket.Run(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
				r.log.Warn("千寻WebSocket接收循环已停止")
			}
		}()
	}
}

// Close 在HTTP停止接收后关闭长连接、排空事件队列，再释放共享HTTP连接。
func (r *Runtime) Close(ctx context.Context) error {
	if r.WebSocket != nil {
		r.WebSocket.Close()
	}
	r.workers.Wait()
	var err error
	if r.Events != nil {
		err = r.Events.Close(ctx)
	}
	if r.Gateway != nil {
		r.Gateway.Close()
	}
	return err
}
func (r *Runtime) ID() string { return "upstreams" }

// Mount 仅提供受现有API key保护的安全目录/资源统计，不暴露任意URL代理或账号凭证。
func (r *Runtime) Mount(router chi.Router) {
	router.Get("/upstreams", func(w http.ResponseWriter, request *http.Request) {
		value := map[string]any{"qianxun_enabled": r.Framework != nil || r.Direct != nil, "mihoyo_enabled": r.Mihoyo != nil, "qianxun_http_contracts": qianxun.OperationCount(), "mihoyo_contracts": mihoyo.EndpointCount()}
		if r.Gateway != nil {
			value["gateway"] = r.Gateway.Snapshot()
		}
		if r.Events != nil {
			value["events"] = r.Events.Snapshot()
		}
		if r.WebSocket != nil {
			value["websocket"] = r.WebSocket.Snapshot()
		}
		httpserver.JSON(w, 200, value)
	})
}

// MountPublic 单独挂载上游回调，使用独立token；不会套用控制台API key或开放其他未鉴权接口。
func (r *Runtime) MountPublic(router chi.Router) {
	if r.callback != nil {
		router.Method(http.MethodPost, "/callbacks/qianxun", r.callback)
	}
}
