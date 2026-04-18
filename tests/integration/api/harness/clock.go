//go:build integration

package harness

import (
	"sync"
	"time"
)

// FakeClock provides deterministic wall time for integration tests.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock() *FakeClock {
	return &FakeClock{now: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
