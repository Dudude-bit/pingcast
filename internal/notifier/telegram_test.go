package notifier_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/notifier"
)

func TestTelegramSender_SendDown(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg := notifier.NewTelegramSender("test-token", server.URL+"/bot%s/sendMessage")

	err := tg.SendDown(t.Context(), 12345, "My API", "https://api.example.com", "connection timeout")
	if err != nil {
		t.Fatalf("SendDown: %v", err)
	}

	text, ok := receivedBody["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text in request body")
	}

	chatID, ok := receivedBody["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Errorf("chat_id = %v, want 12345", receivedBody["chat_id"])
	}
}

func TestTelegramSender_SendUp(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg := notifier.NewTelegramSender("test-token", server.URL+"/bot%s/sendMessage")

	err := tg.SendUp(t.Context(), 12345, "My API", "https://api.example.com")
	if err != nil {
		t.Fatalf("SendUp: %v", err)
	}

	text, ok := receivedBody["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text in request body")
	}
}
