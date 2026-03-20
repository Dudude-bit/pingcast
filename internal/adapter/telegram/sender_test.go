package telegram_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestFactory_CreateSenderAndSend(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	factory := telegram.NewFactoryWithURL("test-token", server.URL+"/bot%s/sendMessage")

	config := json.RawMessage(`{"chat_id": 12345}`)
	sender, err := factory.CreateSender(config)
	if err != nil {
		t.Fatalf("CreateSender: %v", err)
	}

	event := &domain.AlertEvent{
		MonitorName:   "My API",
		MonitorTarget: "GET https://api.example.com",
		Event:         domain.AlertDown,
		Cause:         "connection timeout",
	}

	err = sender.Send(t.Context(), event)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	text, ok := receivedBody["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text")
	}

	chatID, ok := receivedBody["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Errorf("chat_id = %v, want 12345", receivedBody["chat_id"])
	}
}

func TestFactory_ValidateConfig(t *testing.T) {
	factory := telegram.NewFactory("token")

	if err := factory.ValidateConfig(json.RawMessage(`{"chat_id": 12345}`)); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}

	if err := factory.ValidateConfig(json.RawMessage(`{"chat_id": 0}`)); err == nil {
		t.Error("expected error for zero chat_id")
	}

	if err := factory.ValidateConfig(json.RawMessage(`{}`)); err == nil {
		t.Error("expected error for missing chat_id")
	}
}
