//go:build integration

package harness

import "testing"

// Harness is the per-test handle. It owns the App and resets state on
// construction.
type Harness struct {
	t   *testing.T
	App *App
}

func New(t *testing.T) *Harness {
	t.Helper()

	h := &Harness{t: t}
	h.App = NewApp(t)
	t.Cleanup(h.App.Close)

	// State reset (TRUNCATE + FLUSHDB) lands in Task 5.
	return h
}
