//go:build integration

package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---- SMTP ----------------------------------------------------------

type SentEmail struct {
	To      string
	Subject string
	Body    string
}

type FakeSMTP struct {
	mu   sync.Mutex
	sent []SentEmail
}

func NewFakeSMTP() *FakeSMTP { return &FakeSMTP{} }

func (s *FakeSMTP) Send(to, subject, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, SentEmail{To: to, Subject: subject, Body: body})
	return nil
}

func (s *FakeSMTP) Sent() []SentEmail {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SentEmail, len(s.sent))
	copy(out, s.sent)
	return out
}

func (s *FakeSMTP) AssertSent(t *testing.T, to, subjectContains string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.sent {
		if e.To == to && strings.Contains(e.Subject, subjectContains) {
			return
		}
	}
	t.Fatalf("expected email to %q with subject containing %q; got: %v", to, subjectContains, s.sent)
}

// ---- Telegram ------------------------------------------------------

type TelegramCall struct {
	ChatID string
	Text   string
}

type FakeTelegram struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []TelegramCall
}

func NewFakeTelegram() *FakeTelegram {
	f := &FakeTelegram{}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *FakeTelegram) URL() string { return f.server.URL }
func (f *FakeTelegram) Close()      { f.server.Close() }

func (f *FakeTelegram) Calls() []TelegramCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]TelegramCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *FakeTelegram) handle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)

	f.mu.Lock()
	f.calls = append(f.calls, TelegramCall{ChatID: body.ChatID, Text: body.Text})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// ---- Webhook sink --------------------------------------------------

type WebhookHit struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

type FakeWebhookSink struct {
	server *httptest.Server
	mu     sync.Mutex
	hits   []WebhookHit
}

func NewFakeWebhookSink() *FakeWebhookSink {
	s := &FakeWebhookSink{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

func (s *FakeWebhookSink) URL() string { return s.server.URL }
func (s *FakeWebhookSink) Close()      { s.server.Close() }

func (s *FakeWebhookSink) Hits() []WebhookHit {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WebhookHit, len(s.hits))
	copy(out, s.hits)
	return out
}

func (s *FakeWebhookSink) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.hits = append(s.hits, WebhookHit{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
	})
	s.mu.Unlock()
	w.WriteHeader(200)
}
