package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// ConnectOption applies optional configuration to the pgxpool.
type ConnectOption func(*pgxpool.Config)

// WithTracer attaches a pgx.QueryTracer to the connection pool.
func WithTracer(tracer pgx.QueryTracer) ConnectOption {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = tracer
	}
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect creates a connection pool with configurable max connections.
// Pass maxConns=0 to use the default (max(4, numCPU*2)).
// Optional ConnectOption values can be provided to attach tracers, etc.
func Connect(ctx context.Context, databaseURL string, maxConns int32, opts ...ConnectOption) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if maxConns > 0 {
		config.MaxConns = maxConns
	}
	for _, opt := range opts {
		opt(config)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies pending migrations via goose. Files under
// migrations/ use the `-- +goose Up` / `-- +goose Down` convention.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("pgx"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(gooseSlogLogger{})

	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// gooseSlogLogger adapts goose.Logger onto slog so migration logs
// flow through the same pipeline as everything else.
type gooseSlogLogger struct{}

func (gooseSlogLogger) Printf(format string, v ...any) {
	slog.Info(fmt.Sprintf(format, v...))
}
func (gooseSlogLogger) Println(v ...any) { slog.Info(fmt.Sprint(v...)) }
func (gooseSlogLogger) Fatalf(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...))
}
func (gooseSlogLogger) Fatal(v ...any) { slog.Error(fmt.Sprint(v...)) }
