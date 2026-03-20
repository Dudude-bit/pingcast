package checker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func httpMonitor(url string, method string, expectedStatus int, keyword *string) *domain.Monitor {
	cfg := map[string]any{
		"url":             url,
		"method":          method,
		"expected_status": expectedStatus,
	}
	if keyword != nil {
		cfg["keyword"] = *keyword
	}
	data, _ := json.Marshal(cfg)
	return &domain.Monitor{
		Type:        domain.MonitorHTTP,
		CheckConfig: data,
	}
}

func TestHTTPChecker_CheckUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "PingCast/1.0 (uptime monitor; https://pingcast.io)" {
			t.Errorf("unexpected User-Agent: %s", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	c := checker.NewHTTPChecker()
	result := c.Check(context.Background(), httpMonitor(server.URL, "GET", 200, nil))

	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusUp)
	}
	if *result.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", *result.StatusCode)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
}

func TestHTTPChecker_CheckDown_WrongStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	c := checker.NewHTTPChecker()
	result := c.Check(context.Background(), httpMonitor(server.URL, "GET", 200, nil))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
}

func TestHTTPChecker_CheckDown_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	c := checker.NewHTTPCheckerWithTimeout(1)
	result := c.Check(context.Background(), httpMonitor(server.URL, "GET", 200, nil))

	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want %q", result.Status, domain.StatusDown)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for timeout")
	}
}

func TestHTTPChecker_KeywordFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("all systems operational"))
	}))
	defer server.Close()

	keyword := "operational"
	c := checker.NewHTTPChecker()
	result := c.Check(context.Background(), httpMonitor(server.URL, "GET", 200, &keyword))
	if result.Status != domain.StatusUp {
		t.Errorf("status = %q, want up (keyword found)", result.Status)
	}
}

func TestHTTPChecker_KeywordMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("all systems operational"))
	}))
	defer server.Close()

	missing := "notfound"
	c := checker.NewHTTPChecker()
	result := c.Check(context.Background(), httpMonitor(server.URL, "GET", 200, &missing))
	if result.Status != domain.StatusDown {
		t.Errorf("status = %q, want down (keyword missing)", result.Status)
	}
}
