package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kirillinakin/pingcast/internal/database"
)

// SetupTestDB starts a Postgres testcontainer, runs migrations, returns pool + cleanup func.
// Reuse across all integration tests.
func SetupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("pingcast_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("get connection string: %v", err)
	}

	pool, err := database.Connect(ctx, connStr, 5)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("connect to test database: %v", err)
	}

	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("run migrations: %v", err)
	}

	// Disable FORCE ROW LEVEL SECURITY so tests can operate without
	// setting app.current_user_id in every transaction.
	rlsTables := []string{"monitors", "notification_channels", "check_results", "incidents"}
	for _, table := range rlsTables {
		if _, err := pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DISABLE ROW LEVEL SECURITY", table)); err != nil {
			pool.Close()
			pgContainer.Terminate(ctx)
			t.Fatalf("disable RLS on %s: %v", table, err)
		}
	}

	cleanup := func() {
		pool.Close()
		pgContainer.Terminate(ctx)
	}

	return pool, cleanup
}
