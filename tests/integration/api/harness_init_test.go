//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHarness_Init(t *testing.T) {
	h := harness.New(t)
	if h == nil {
		t.Fatal("harness.New returned nil")
	}
}

func TestHarness_ContainersBooted(t *testing.T) {
	c := harness.GetContainers()
	if c == nil {
		t.Fatal("containers not initialized")
	}
	if c.PostgresURL == "" {
		t.Error("postgres url empty")
	}
	if c.RedisURL == "" {
		t.Error("redis url empty")
	}
	if c.NATSURL == "" {
		t.Error("nats url empty")
	}
}

func TestHarness_SchemaMigrated(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, harness.GetContainers().PostgresURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count == 0 {
		t.Fatal("migrations did not create any tables")
	}
	t.Logf("public schema has %d tables", count)
}
