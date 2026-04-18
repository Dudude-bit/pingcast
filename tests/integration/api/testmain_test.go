//go:build integration

package api

import (
	"fmt"
	"os"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestMain(m *testing.M) {
	if err := harness.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "harness setup failed: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	harness.Teardown()
	os.Exit(code)
}
