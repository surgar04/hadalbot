// Package app 显式装配依赖，统一管理启动、就绪状态和优雅退出。
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/httpserver"
	"github.com/surgar04/hadalbot/internal/integrations"
	"github.com/surgar04/hadalbot/internal/modules"
	"github.com/surgar04/hadalbot/internal/pluginhost"
	"github.com/surgar04/hadalbot/internal/storage"
)

func Run(ctx context.Context, c config.Config, log *slog.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	connections, err := storage.Open(startup, c)
	cancel()
	if err != nil {
		return err
	}
	defer connections.Close()
	registry := modules.NewRegistry()
	upstreams, err := integrations.New(c, log)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), c.HTTP.ShutdownTimeout)
		defer cancel()
		if err := upstreams.Close(cleanup); err != nil {
			log.Warn("上游组件关闭未在预算内完成")
		}
	}()
	if err = registry.Register(upstreams); err != nil {
		return err
	}
	plugins, err := pluginhost.New(c, upstreams, log)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), c.HTTP.ShutdownTimeout)
		defer cancel()
		if err := plugins.Close(cleanup); err != nil {
			log.Warn("插件关闭未在预算内完成")
		}
	}()
	if err = registry.Register(plugins); err != nil {
		return err
	}
	if err = plugins.Start(ctx, upstreams.Events); err != nil {
		return err
	}
	state := &httpserver.State{}
	server := &http.Server{Addr: c.HTTP.Address, Handler: httpserver.New(c, log, connections, state, registry, upstreams.MountPublic), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: c.HTTP.ReadTimeout, WriteTimeout: c.HTTP.WriteTimeout, IdleTimeout: c.HTTP.IdleTimeout, MaxHeaderBytes: 1 << 20}
	listener, err := net.Listen("tcp", c.HTTP.Address)
	if err != nil {
		return fmt.Errorf("监听 HTTP: %w", err)
	}
	state.Ready.Store(true)
	upstreams.Start(ctx)
	log.Info("server started", "address", listener.Addr().String(), "environment", c.Environment, "modules", registry.IDs())
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	select {
	case err = <-finished:
		state.Ready.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		state.Ready.Store(false)
		shutdown, stop := context.WithTimeout(context.Background(), c.HTTP.ShutdownTimeout)
		defer stop()
		if err = server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return fmt.Errorf("关闭 HTTP: %w", err)
		}
		err = <-finished
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		log.Info("server stopped")
		return nil
	}
}
