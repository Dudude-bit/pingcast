//go:build integration

package harness

import (
	"context"
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
	JS    jetstream.JetStream

	// Per-test deterministic replacements injected into
	// bootstrap.AppDeps via Clock/Random fields.
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

	if streamsErr := natsadapter.SetupStreams(ctx, js); streamsErr != nil {
		pool.Close()
		_ = rdb.Close()
		_ = nc.Drain()
		t.Fatalf("nats streams: %v", streamsErr)
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
		JS:       js,
		Clock:    clock,
		Rand:     rng,
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
