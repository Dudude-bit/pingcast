package checker

import (
	"crypto/tls"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestHTTPChecker_RejectsLegacyTLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(nil)
	// Intentionally legacy to verify the checker rejects it. Not a real-world config.
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11} //nolint:gosec // test fixture
	srv.StartTLS()
	defer srv.Close()

	cfg, _ := json.Marshal(HTTPCheckConfig{URL: srv.URL, Method: MethodGET, ExpectedStatus: 200})
	monitor := &domain.Monitor{
		ID:          uuid.New(),
		Type:        domain.MonitorHTTP,
		CheckConfig: cfg,
	}

	chk := NewHTTPChecker()
	result := chk.Check(t.Context(), monitor)

	if result.Status != domain.StatusDown {
		t.Fatalf("expected StatusDown, got %s", result.Status)
	}
	if result.ErrorMessage == nil {
		t.Fatal("expected ErrorMessage to be populated")
	}
}
