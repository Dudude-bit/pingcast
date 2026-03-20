package telegram_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
)

func TestSender_NotifyDown(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender := telegram.NewWithURL("test-token", server.URL+"/bot%s/sendMessage")
	alert := sender.ForChat(12345)

	err := alert.NotifyDown(t.Context(), "My API", "https://api.example.com", "connection timeout")
	if err != nil {
		t.Fatalf("NotifyDown: %v", err)
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

func TestSender_NotifyUp(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender := telegram.NewWithURL("test-token", server.URL+"/bot%s/sendMessage")
	alert := sender.ForChat(12345)

	err := alert.NotifyUp(t.Context(), "My API", "https://api.example.com")
	if err != nil {
		t.Fatalf("NotifyUp: %v", err)
	}

	text, ok := receivedBody["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text")
	}
}
