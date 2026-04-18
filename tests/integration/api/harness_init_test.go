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
