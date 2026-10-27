package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/surgar04/hadalbot/internal/config"
	"github.com/surgar04/hadalbot/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	file := flag.String("config", "", "YAML 配置文件")
	command := flag.String("command", "status", "status 或 up；默认只查询迁移状态")
	flag.Parse()
	if *command != "status" && *command != "up" {
		return errors.New("command 只支持 status 或 up")
	}
	cfg, err := config.Load(*file)
	if err != nil {
		return err
	}
	if !cfg.Database.Enabled || cfg.Database.DSN == "" {
		return errors.New("迁移需要显式启用独立的 Go 数据库")
	}
	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return errors.New("数据库配置无效")
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		return errors.New("迁移数据库连接失败")
	}
	goose.SetBaseFS(migrations.Files)
	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	if *command == "up" {
		return goose.UpContext(ctx, db, ".")
	}
	return goose.StatusContext(ctx, db, ".")
}
