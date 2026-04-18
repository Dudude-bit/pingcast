//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Paired tests prove per-test truncate is in effect: _PartA writes a
// row, _PartB verifies the table is empty. Go runs tests in the order
// they appear in the file, so B follows A.

func TestTruncate_PartA_InsertsRow(t *testing.T) {
	h := harness.New(t)
	_, err := h.App.Pool.Exec(context.Background(),
		`INSERT INTO users (id, email, slug, password_hash)
		 VALUES (gen_random_uuid(), 'part-a@test.local', 'part-a', 'x')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestTruncate_PartB_StartsEmpty(t *testing.T) {
	h := harness.New(t)
	var count int
	err := h.App.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty users, found %d rows — truncate failed", count)
	}
}
