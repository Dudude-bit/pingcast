package webhook_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestFactory_CreateSenderAndSend(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Custom") != "test-value" {
			t.Errorf("X-Custom = %s, want test-value", r.Header.Get("X-Custom"))
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
	}))
	defer server.Close()

	config := json.RawMessage(`{"url":"` + server.URL + `","headers":{"X-Custom":"test-value"}}`)

	factory := webhook.NewFactory()
	sender, err := factory.CreateSender(config)
	if err != nil {
		t.Fatalf("CreateSender: %v", err)
	}

	event := &domain.AlertEvent{
		MonitorName:   "My API",
		MonitorTarget: "GET https://api.example.com",
		Event:         domain.AlertDown,
		Cause:         "timeout",
	}

	if err := sender.Send(t.Context(), event); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if receivedBody["monitor_name"] != "My API" {
		t.Errorf("monitor_name = %v, want My API", receivedBody["monitor_name"])
	}
}

func TestFactory_ValidateConfig(t *testing.T) {
	factory := webhook.NewFactory()

	if err := factory.ValidateConfig(json.RawMessage(`{"url":"https://hooks.slack.com/test"}`)); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}

	if err := factory.ValidateConfig(json.RawMessage(`{"url":""}`)); err == nil {
		t.Error("expected error for empty url")
	}

	if err := factory.ValidateConfig(json.RawMessage(`{"url":"ftp://bad"}`)); err == nil {
		t.Error("expected error for non-http scheme")
	}

	if err := factory.ValidateConfig(json.RawMessage(`{"url":"http://localhost:8080"}`)); err == nil {
		t.Error("expected error for localhost")
	}
}
