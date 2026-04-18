//go:build integration

package harness

import "testing"

// Harness is the per-test handle. Fields grow across Tasks 2-7.
type Harness struct {
	t *testing.T
}

func New(t *testing.T) *Harness {
	t.Helper()
	return &Harness{t: t}
}
