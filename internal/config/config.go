// Package config 管理配置优先级：环境变量 > 显式 YAML 文件 > 默认值。
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment string   `mapstructure:"environment"`
	HTTP        HTTP     `mapstructure:"http"`
	Log         Log      `mapstructure:"log"`
	Database    Database `mapstructure:"database"`
	Redis       Redis    `mapstructure:"redis"`
	Gateway     Gateway  `mapstructure:"gateway"`
	Qianxun     Qianxun  `mapstructure:"qianxun"`
	Mihoyo      Mihoyo   `mapstructure:"mihoyo"`
	Plugins     Plugins  `mapstructure:"plugins"`
}

// Plugins默认关闭。EnabledIDs是显式白名单，目录里出现新文件不会自动获得执行权。
type Plugins struct {
	Enabled      bool                      `mapstructure:"enabled"`
	Directory    string                    `mapstructure:"directory"`
	TypeScript   bool                      `mapstructure:"typescript"`
	Node         string                    `mapstructure:"node"`
	TSHeapMB     int                       `mapstructure:"ts_heap_mb"`
	MaxTSWorkers int                       `mapstructure:"max_ts_workers"`
	MaxPlugins   int                       `mapstructure:"max_plugins"`
	EnabledIDs   []string                  `mapstructure:"enabled_ids"`
	Grants       map[string][]string       `mapstructure:"grants"`
	Config       map[string]map[string]any `mapstructure:"config"`
}

// Gateway 为所有上游调用分配总超时、响应上限和只读重试预算。
type Gateway struct {
	Timeout          time.Duration `mapstructure:"timeout"`
	MaxResponseBytes int64         `mapstructure:"max_response_bytes"`
	MaxRetries       int           `mapstructure:"max_retries"`
}

// Qianxun 配置不会自动登录或启动微信；只有显式Enabled才装配相应资源。
type Qianxun struct {
	Enabled              bool          `mapstructure:"enabled"`
	Mode                 string        `mapstructure:"mode"`
	BaseURL              string        `mapstructure:"base_url"`
	SafeKey              string        `mapstructure:"safe_key"`
	WXID                 string        `mapstructure:"wxid"`
	Concurrency          int           `mapstructure:"concurrency"`
	QueueSize            int           `mapstructure:"queue_size"`
	MinInterval          time.Duration `mapstructure:"min_interval"`
	CallbackEnabled      bool          `mapstructure:"callback_enabled"`
	CallbackToken        string        `mapstructure:"callback_token"`
	CallbackMaxBodyBytes int64         `mapstructure:"callback_max_body_bytes"`
	EventShards          int           `mapstructure:"event_shards"`
	EventQueuePerShard   int           `mapstructure:"event_queue_per_shard"`
	WebSocketEnabled     bool          `mapstructure:"websocket_enabled"`
	WebSocketURL         string        `mapstructure:"websocket_url"`
}

// Mihoyo 默认不携带任何账号凭证；业务经所属Provider取得授权后创建ForAccount句柄。
type Mihoyo struct {
	Enabled     bool          `mapstructure:"enabled"`
	Concurrency int           `mapstructure:"concurrency"`
	QueueSize   int           `mapstructure:"queue_size"`
	MinInterval time.Duration `mapstructure:"min_interval"`
}
type HTTP struct {
	Address         string        `mapstructure:"address"`
	APIKey          string        `mapstructure:"api_key"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	MaxBodyBytes    int64         `mapstructure:"max_body_bytes"`
}
type Log struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}
type Database struct {
	Enabled               bool          `mapstructure:"enabled"`
	DSN                   string        `mapstructure:"dsn"`
	MaxConnections        int32         `mapstructure:"max_connections"`
	MinConnections        int32         `mapstructure:"min_connections"`
	MaxConnectionLifetime time.Duration `mapstructure:"max_connection_lifetime"`
}
type Redis struct {
	Enabled  bool   `mapstructure:"enabled"`
	URL      string `mapstructure:"url"`
	PoolSize int    `mapstructure:"pool_size"`
}

func Load(filename string) (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("HADALBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	defaults := map[string]any{
		"environment": "development", "http.address": "127.0.0.1:18878", "http.api_key": "",
		"http.read_timeout": "15s", "http.write_timeout": "30s", "http.idle_timeout": "60s", "http.shutdown_timeout": "15s", "http.max_body_bytes": int64(2 << 20),
		"log.level": "info", "log.format": "json", "database.enabled": false, "database.dsn": "",
		"database.max_connections": int32(8), "database.min_connections": int32(0), "database.max_connection_lifetime": "30m",
		"redis.enabled": false, "redis.url": "redis://127.0.0.1:16379/0", "redis.pool_size": 4,
		"gateway.timeout": "30s", "gateway.max_response_bytes": int64(32 << 20), "gateway.max_retries": 1,
		"qianxun.enabled": false, "qianxun.mode": "framework", "qianxun.base_url": "http://127.0.0.1:7777", "qianxun.safe_key": "", "qianxun.wxid": "",
		"qianxun.concurrency": 4, "qianxun.queue_size": 64, "qianxun.min_interval": "20ms",
		"qianxun.callback_enabled": false, "qianxun.callback_token": "", "qianxun.callback_max_body_bytes": int64(512 << 10), "qianxun.event_shards": 4, "qianxun.event_queue_per_shard": 16,
		"qianxun.websocket_enabled": false, "qianxun.websocket_url": "",
		"mihoyo.enabled": false, "mihoyo.concurrency": 8, "mihoyo.queue_size": 128, "mihoyo.min_interval": "100ms",
		"plugins.enabled": false, "plugins.directory": "plugins", "plugins.typescript": false, "plugins.node": "node", "plugins.ts_heap_mb": 128, "plugins.max_plugins": 128,
		"plugins.enabled_ids": []string{}, "plugins.grants": map[string][]string{}, "plugins.config": map[string]map[string]any{}, "plugins.max_ts_workers": 4,
	}
	for k, val := range defaults {
		v.SetDefault(k, val)
		if err := v.BindEnv(k); err != nil {
			return Config{}, err
		}
	}
	if filename != "" {
		v.SetConfigFile(filename)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("读取配置: %w", err)
		}
	}
	var c Config
	if err := v.UnmarshalExact(&c); err != nil {
		return Config{}, errors.New("配置结构或字段类型无效，请检查 YAML 字段名称及环境变量")
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Plugins.MaxPlugins < 1 || c.Plugins.MaxPlugins > 1024 || c.Plugins.TSHeapMB < 64 || c.Plugins.TSHeapMB > 1024 || c.Plugins.MaxTSWorkers < 1 || c.Plugins.MaxTSWorkers > 16 || len(c.Plugins.EnabledIDs) > c.Plugins.MaxPlugins {
		return errors.New("插件系统资源预算无效")
	}
	if c.Plugins.TypeScript && (!c.Plugins.Enabled || c.Plugins.Node == "" || c.Plugins.Directory == "") {
		return errors.New("TS兼容需要显式启用插件系统并配置目录和Node")
	}
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return errors.New("environment 必须是 development、test 或 production")
	}
	host, port, err := net.SplitHostPort(c.HTTP.Address)
	if err != nil || port == "" {
		return errors.New("http.address 必须是 host:port")
	}
	if _, err = net.LookupPort("tcp", port); err != nil {
		return errors.New("http.address 端口无效")
	}
	ip := net.ParseIP(host)
	local := host == "localhost" || (ip != nil && ip.IsLoopback())
	if (c.Environment == "production" || !local) && len(c.HTTP.APIKey) < 32 {
		return errors.New("生产环境或非回环监听必须配置至少32字符的 http.api_key")
	}
	if c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 || c.HTTP.IdleTimeout <= 0 || c.HTTP.ShutdownTimeout <= 0 || c.HTTP.MaxBodyBytes <= 0 {
		return errors.New("HTTP 超时和请求体上限必须大于0")
	}
	if c.Log.Level != "debug" && c.Log.Level != "info" && c.Log.Level != "warn" && c.Log.Level != "error" {
		return errors.New("log.level 无效")
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		return errors.New("log.format 必须是 json 或 text")
	}
	if c.Database.MaxConnections <= 0 || c.Database.MinConnections < 0 || c.Database.MinConnections > c.Database.MaxConnections || c.Database.MaxConnectionLifetime <= 0 {
		return errors.New("数据库连接池配置无效")
	}
	if c.Database.Enabled && c.Database.DSN == "" {
		return errors.New("启用数据库后必须设置 database.dsn")
	}
	if c.Redis.PoolSize <= 0 {
		return errors.New("redis.pool_size 必须大于0")
	}
	if c.Redis.Enabled {
		u, e := url.Parse(c.Redis.URL)
		if e != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Host == "" {
			return errors.New("redis.url 无效")
		}
	}
	if c.Gateway.Timeout <= 0 || c.Gateway.MaxResponseBytes < 1 || c.Gateway.MaxResponseBytes > 64<<20 || c.Gateway.MaxRetries < 0 || c.Gateway.MaxRetries > 3 {
		return errors.New("gateway资源预算无效")
	}
	if c.Qianxun.Enabled {
		q := c.Qianxun
		u, e := url.Parse(q.BaseURL)
		if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("qianxun.base_url无效")
		}
		if q.Mode != "framework" && q.Mode != "direct" {
			return errors.New("qianxun.mode必须为framework或direct")
		}
		if q.Concurrency < 1 || q.Concurrency > 64 || q.QueueSize < 0 || q.QueueSize > 4096 || q.MinInterval < 0 || q.EventShards < 1 || q.EventShards > 64 || q.EventQueuePerShard < 1 || q.EventQueuePerShard > 4096 {
			return errors.New("千寻并发或事件预算无效")
		}
		if q.CallbackEnabled && (len(q.CallbackToken) < 32 || q.CallbackMaxBodyBytes < 1 || q.CallbackMaxBodyBytes > 2<<20) {
			return errors.New("千寻回调需要独立32字符token和有效请求体预算")
		}
		if q.WebSocketEnabled {
			ws, e := url.Parse(q.WebSocketURL)
			if e != nil || ws.Host == "" || ws.User != nil || ws.RawQuery != "" || ws.Fragment != "" || (ws.Scheme != "ws" && ws.Scheme != "wss") {
				return errors.New("启用WebSocket时必须配置明确的ws/wss地址")
			}
		}
	} else if c.Qianxun.CallbackEnabled || c.Qianxun.WebSocketEnabled {
		return errors.New("回调或WebSocket依赖qianxun.enabled")
	}
	if c.Mihoyo.Enabled && (c.Mihoyo.Concurrency < 1 || c.Mihoyo.Concurrency > 64 || c.Mihoyo.QueueSize < 0 || c.Mihoyo.QueueSize > 4096 || c.Mihoyo.MinInterval < 0) {
		return errors.New("米哈游并发预算无效")
	}
	return nil
}
