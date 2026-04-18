//go:build integration

package api

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHealth_Smoke(t *testing.T) {
	h := harness.New(t)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := h.App.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
}
