package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/config"
	"time"
)

func Open(ctx context.Context, c config.Config) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}
	conf.ConnConfig.Host, conf.ConnConfig.Port = c.DBHost, c.DBPort
	conf.ConnConfig.User, conf.ConnConfig.Password, conf.ConnConfig.Database = c.DBUser, c.DBPassword, c.DBName
	conf.ConnConfig.RuntimeParams["search_path"] = c.DBSchema
	conf.ConnConfig.RuntimeParams["application_name"] = "kaderisasi-admin-go"
	conf.MaxConns, conf.MinConns = 10, 2
	conf.ConnConfig.ConnectTimeout = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, err
	}
	var schema string
	if err = pool.QueryRow(ctx, "select current_schema()").Scan(&schema); err != nil || schema != c.DBSchema {
		pool.Close()
		return nil, fmt.Errorf("database schema unavailable: %s", c.DBSchema)
	}
	return pool, nil
}
