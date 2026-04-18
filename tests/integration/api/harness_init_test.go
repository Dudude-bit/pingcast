//go:build integration

package api

import (
	"testing"

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
