// Package storage 提供可选基础设施；构造失败时释放已经建立的连接。
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/surgar04/hadalbot/internal/config"
)

type Connections struct {
	Postgres *pgxpool.Pool
	Redis    *redis.Client
}

func Open(ctx context.Context, c config.Config) (*Connections, error) {
	s := &Connections{}
	fail := func(err error) (*Connections, error) { s.Close(); return nil, err }
	if c.Database.Enabled {
		cfg, err := pgxpool.ParseConfig(c.Database.DSN)
		if err != nil {
			return fail(errors.New("PostgreSQL DSN 无效"))
		}
		cfg.MaxConns = c.Database.MaxConnections
		cfg.MinConns = c.Database.MinConnections
		cfg.MaxConnLifetime = c.Database.MaxConnectionLifetime
		cfg.ConnConfig.ConnectTimeout = 5 * time.Second
		s.Postgres, err = pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return fail(errors.New("创建 PostgreSQL 连接池失败"))
		}
		if err = s.Postgres.Ping(ctx); err != nil {
			return fail(errors.New("PostgreSQL 连接检查失败"))
		}
	}
	if c.Redis.Enabled {
		opts, err := redis.ParseURL(c.Redis.URL)
		if err != nil {
			return fail(errors.New("redis URL 无效"))
		}
		opts.PoolSize = c.Redis.PoolSize
		opts.MaxActiveConns = c.Redis.PoolSize
		opts.PipelinePoolSize = -1
		opts.PoolTimeout = 3 * time.Second
		opts.ContextTimeoutEnabled = true
		opts.DialTimeout = 3 * time.Second
		opts.ReadTimeout = 3 * time.Second
		opts.WriteTimeout = 3 * time.Second
		s.Redis = redis.NewClient(opts)
		if err = s.Redis.Ping(ctx).Err(); err != nil {
			return fail(errors.New("redis 连接检查失败"))
		}
	}
	return s, nil
}
func (s *Connections) Close() {
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.Postgres != nil {
		s.Postgres.Close()
	}
}
func (s *Connections) Check(ctx context.Context) map[string]string {
	out := map[string]string{"postgres": "disabled", "redis": "disabled"}
	if s.Postgres != nil {
		out["postgres"] = "ok"
		if s.Postgres.Ping(ctx) != nil {
			out["postgres"] = "unavailable"
		}
	}
	if s.Redis != nil {
		out["redis"] = "ok"
		if s.Redis.Ping(ctx).Err() != nil {
			out["redis"] = "unavailable"
		}
	}
	return out
}
