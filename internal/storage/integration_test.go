package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/surgar04/hadalbot/internal/testutil"
	"github.com/surgar04/hadalbot/migrations"
)

// 仅接受显式测试环境变量，在独立 schema 下执行真实迁移，不读取旧项目 .env。
func TestIntegrationPostgresAndMigrations(t *testing.T) {
	dsn := os.Getenv("HADALBOT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("需要独立测试 PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("测试数据库不可连接")
	}
	defer admin.Close(context.Background())
	schema := fmt.Sprintf("hadalbot_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, e := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	cfg := testutil.Config(t)
	cfg.Database.Enabled = true
	cfg.Database.DSN = u.String()
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatalf("迁移重复执行失败: %v", err)
	}
	var count int
	if err = s.Postgres.QueryRow(ctx, "SELECT count(*) FROM hadalbot_schema_metadata").Scan(&count); err != nil || count != 1 {
		t.Fatalf("迁移结果异常: count=%d error=%v", count, err)
	}
	if s.Check(ctx)["postgres"] != "ok" {
		t.Fatal("数据库未就绪")
	}
}
func TestIntegrationRedis(t *testing.T) {
	address := os.Getenv("HADALBOT_TEST_REDIS_URL")
	if address == "" {
		t.Skip("需要独立测试 Redis")
	}
	cfg := testutil.Config(t)
	cfg.Redis.Enabled = true
	cfg.Redis.URL = address
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key := fmt.Sprintf("hadalbot-go:test:%d", time.Now().UnixNano())
	if err = s.Redis.Set(ctx, key, "ok", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	defer s.Redis.Del(context.Background(), key)
	if got, e := s.Redis.Get(ctx, key).Result(); e != nil || got != "ok" {
		t.Fatalf("Redis读写失败: %v", e)
	}
}
