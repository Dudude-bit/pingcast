//go:build integration

package harness

import (
	"os"
	"testing"
)

// TestMain is the package-wide setup/teardown entry point for the
// integration harness. Task 2 replaces the body with container boot.
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
