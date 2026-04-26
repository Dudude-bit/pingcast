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
	"github.com/kirillinakin/pingcast/internal/port"
)

// TestFounderCap is a small cap so race-condition / atomic-tagging
// tests can saturate the founder pool with just a couple of webhooks.
// 2 is enough: lock semantics are identical at any cap size.
const TestFounderCap = 2

// TestTelegramBotToken is the path-segment "secret" the harness boots
// with for /webhook/telegram/:token. Tests verify that requests with a
// different token are rejected (the IDOR fix) and requests with this
// token are accepted.
const TestTelegramBotToken = "test-telegram-bot-token-abc123"

// TestFounderVariantID is the LemonSqueezy variant ID the harness
// pretends is the $9 founder tier. Webhook tests send `variant_id`
// matching this constant to assert founder-cap accounting; sending
// any other ID asserts retail accounting.
const TestFounderVariantID = "12345"

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

	// Rate-limit defaults for the harness: generous enough that the
	// existing C1/C2 suites don't ever trip limiter flakes. Individual
	// C4 tests that exercise burst behaviour use their own App with
	// a tighter config via StartAppWithRateLimits.
	defaultRL := &port.RateLimitConfig{
		RegisterPerHour: 1000,
		LoginPer15Min:   1000,
		StatusPerMin:    1000,
		WritePerMin:     1000,
		ReadPerMin:      1000,
	}

	smtp := NewFakeSMTP()
	bootApp, err := bootstrap.NewApp(bootstrap.AppDeps{
		Pool:                         pool,
		Redis:                        rdb,
		NATS:                         nc,
		JS:                           js,
		Cipher:                       cipher,
		LemonSqueezySecret:           "test-ls-secret",
		LemonSqueezyFounderVariantID: TestFounderVariantID,
		TelegramBotToken:             TestTelegramBotToken,
		FounderCap:                   TestFounderCap,
		Clock:                        clock,
		Random:                       rng,
		RateLimits:                   defaultRL,
		Mailer:                       smtp.AsMailer(),
	})
	if err != nil {
		pool.Close()
		_ = rdb.Close()
		_ = nc.Drain()
		t.Fatalf("compose app: %v", err)
	}

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
