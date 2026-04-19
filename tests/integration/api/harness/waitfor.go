//go:build integration

package harness

import (
	"testing"
	"time"
)

// WaitFor polls predicate every 100ms until it returns true or deadline
// elapses. Fails the test with desc on timeout. Used for async outcomes
// like "incident opened" or "fake got 3 calls".
func WaitFor(t *testing.T, deadline time.Duration, predicate func() bool, desc string) {
	t.Helper()

	if predicate() {
		return
	}

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	for {
		select {
		case <-tick.C:
			if predicate() {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out after %s waiting for: %s", deadline, desc)
		}
	}
}
