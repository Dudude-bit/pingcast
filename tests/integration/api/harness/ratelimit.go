//go:build integration

package harness

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/port"
)

// NewAppWithRateLimits composes an isolated App with the given
// rate-limit config. Used by C4 tests that need tight per-scope
// buckets to exercise burst + Retry-After behaviour.
//
// A fresh Postgres pool / Redis client / NATS conn is spun up against
// the shared containers so the caller can reuse harness.Reset() +
// RegisterAndLogin flows without competing with other tests.
func NewAppWithRateLimits(t *testing.T, cfg *port.RateLimitConfig) *App {
	t.Helper()

	c := GetContainers()
	if c == nil {
		t.Fatal("harness not initialized")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, c.PostgresURL)
	if err != nil {
		t.Fatalf("pg: %v", err)
	}
	rdb, err := redisadapter.Connect(ctx, c.RedisURL)
	if err != nil {
		pool.Close()
		t.Fatalf("redis: %v", err)
	}
	nc, err := natsadapter.Connect(c.NATSURL)
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		t.Fatalf("nats: %v", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		_ = nc.Drain()
		t.Fatalf("jetstream: %v", err)
	}
	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		pool.Close()
		_ = rdb.Close()
		_ = nc.Drain()
		t.Fatalf("streams: %v", err)
	}
	cipher, err := crypto.NewEncryptor(1, map[byte]string{1: testEncryptionKey()})
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	clock := NewFakeClock()
	rng := NewFakeRandom()

	bootApp, err := bootstrap.NewApp(bootstrap.AppDeps{
		Pool:               pool,
		Redis:              rdb,
		NATS:               nc,
		JS:                 js,
		Cipher:             cipher,
		LemonSqueezySecret: "test-ls-secret",
		Clock:              clock,
		Random:             rng,
		RateLimits:         cfg,
	})
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		_ = nc.Drain()
		t.Fatalf("compose app: %v", err)
	}

	smtp := NewFakeSMTP()
	tg := NewFakeTelegram()
	t.Cleanup(tg.Close)
	sink := NewFakeWebhookSink()
	t.Cleanup(sink.Close)

	a := &App{
		App:      bootApp,
		Pool:     pool,
		Redis:    rdb,
		NATS:     nc,
		JS:       js,
		Clock:    clock,
		Rand:     rng,
		SMTP:     smtp,
		Telegram: tg,
		Webhook:  sink,
	}
	t.Cleanup(a.Close)
	a.Reset(t)
	return a
}
