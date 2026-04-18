//go:build integration

package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// RunMigrations applies all *.sql files under internal/database/migrations
// in filename order against the target Postgres URL. The existing repo
// does not use goose annotations, so we run raw SQL directly.
func RunMigrations(ctx context.Context, pgURL string) error {
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	dir, err := findMigrations()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := conn.Exec(ctx, string(raw)); err != nil {
			return fmt.Errorf("exec %s: %w", name, err)
		}
	}
	return nil
}

// findMigrations locates internal/database/migrations relative to the
// source file, resistant to CWD differences between test invocations.
func findMigrations() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	// thisFile = <repo>/tests/integration/api/harness/migrate.go
	repo := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	return filepath.Join(repo, "internal", "database", "migrations"), nil
}
