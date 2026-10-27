package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/surgar04/hadalbot/internal/app"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/internal/observability"
	"github.com/surgar04/hadalbot/internal/version"
)

func main() { os.Exit(run()) }
func run() int {
	filename := flag.String("config", "", "YAML 配置文件；留空使用环境变量和默认配置")
	check := flag.Bool("check-config", false, "只验证配置，不连接服务或输出敏感配置")
	showVersion := flag.Bool("version", false, "显示构建信息")
	healthcheck := flag.Bool("healthcheck", false, "通过本机 readyz 检查已运行实例，供容器健康检查使用")
	flag.Parse()
	if *showVersion {
		fmt.Printf("HadalBot-Go %s commit=%s built_at=%s\n", version.Version, version.Commit, version.BuiltAt)
		return 0
	}
	cfg, err := config.Load(*filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *healthcheck {
		_, port, err := net.SplitHostPort(cfg.HTTP.Address)
		if err != nil {
			return 1
		}
		client := http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
		resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/readyz")
		if err != nil {
			return 1
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return 1
		}
		return 0
	}
	log := observability.Logger(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if *check {
		log.Info("configuration valid")
		return 0
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err = app.Run(ctx, cfg, log); err != nil {
		log.Error("application stopped", "error", err)
		return 1
	}
	return 0
}
