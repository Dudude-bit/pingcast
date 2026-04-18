//go:build integration

package harness

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/crypto"
)

// App is the test-scoped wrapper over bootstrap.App. It also holds
// infrastructure handles so tests can inspect state and harness
// helpers can reset between tests.
type App struct {
	*bootstrap.App

	Pool  *pgxpool.Pool
	Redis *goredis.Client
	NATS  *nats.Conn

	// Per-test deterministic replacements. Services don't yet accept
	// these — threading is introduced in Tasks 10-12. Until then the
	// fakes are wired at the harness level only.
	Clock *FakeClock
	Rand  *FakeRandom

	SMTP     *FakeSMTP
	Telegram *FakeTelegram
	Webhook  *FakeWebhookSink
}

// NewApp composes a fresh API app wired to the shared test containers.
// The caller is responsible for App.Close() via t.Cleanup.
func NewApp(t *testing.T) *App {
	t.Helper()

	c := GetContainers()
	if c == nil {
		t.Fatal("harness not initialized (TestMain did not run?)")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, c.PostgresURL)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
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
		t.Fatalf("nats streams: %v", err)
	}

	cipher, err := crypto.NewEncryptor(1, map[byte]string{1: testEncryptionKey()})
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	bootApp, err := bootstrap.NewApp(bootstrap.AppDeps{
		Pool:               pool,
		Redis:              rdb,
		NATS:               nc,
		JS:                 js,
		Cipher:             cipher,
		LemonSqueezySecret: "test-ls-secret",
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

	return &App{
		App:      bootApp,
		Pool:     pool,
		Redis:    rdb,
		NATS:     nc,
		Clock:    NewFakeClock(),
		Rand:     NewFakeRandom(),
		SMTP:     smtp,
		Telegram: tg,
		Webhook:  sink,
	}
}

func (a *App) Close() {
	if a == nil {
		return
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.NATS != nil {
		_ = a.NATS.Drain()
	}
}

// testEncryptionKey is a fixed 32-byte base64 key for deterministic tests.
func testEncryptionKey() string {
	return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
}

var _ = fmt.Sprintf
