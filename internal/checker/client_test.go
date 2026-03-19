package checker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/checker"
)

func TestChecker_CheckUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "PingCast/1.0 (uptime monitor; https://pingcast.io)" {
			t.Errorf("unexpected User-Agent: %s", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	c := checker.NewClient()
	result := c.Check(context.Background(), &checker.MonitorInfo{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	})

	if result.Status != "up" {
		t.Errorf("status = %q, want %q", result.Status, "up")
	}
	if *result.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", *result.StatusCode)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time")
	}
}

func TestChecker_CheckDown_WrongStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	c := checker.NewClient()
	result := c.Check(context.Background(), &checker.MonitorInfo{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	})

	if result.Status != "down" {
		t.Errorf("status = %q, want %q", result.Status, "down")
	}
}

func TestChecker_CheckDown_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	c := checker.NewClientWithTimeout(1)
	result := c.Check(context.Background(), &checker.MonitorInfo{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: 200,
	})

	if result.Status != "down" {
		t.Errorf("status = %q, want %q", result.Status, "down")
	}
	if result.ErrorMessage == nil || *result.ErrorMessage == "" {
		t.Error("expected error message for timeout")
	}
}

func TestChecker_KeywordFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("all systems operational"))
	}))
	defer server.Close()

	keyword := "operational"
	c := checker.NewClient()
	result := c.Check(context.Background(), &checker.MonitorInfo{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: 200,
		Keyword:        &keyword,
	})
	if result.Status != "up" {
		t.Errorf("status = %q, want up (keyword found)", result.Status)
	}
}

func TestChecker_KeywordMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("all systems operational"))
	}))
	defer server.Close()

	missing := "notfound"
	c := checker.NewClient()
	result := c.Check(context.Background(), &checker.MonitorInfo{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: 200,
		Keyword:        &missing,
	})
	if result.Status != "down" {
		t.Errorf("status = %q, want down (keyword missing)", result.Status)
	}
}
