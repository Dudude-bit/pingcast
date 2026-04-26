//go:build integration

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
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

// FindSent returns the first captured email matching `to`, or nil. Used
// when a test needs to inspect subject/body content (e.g. asserting the
// email rendered in the right locale).
func (s *FakeSMTP) FindSent(to string) *SentEmail {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.sent {
		if s.sent[i].To == to {
			return &s.sent[i]
		}
	}
	return nil
}

// AsMailer adapts FakeSMTP to port.Mailer so the harness can inject it
// into bootstrap.AppDeps.Mailer for tests that assert email content
// (e.g. per-locale subscription confirmations).
func (s *FakeSMTP) AsMailer() port.Mailer { return mailerAdapter{s} }

type mailerAdapter struct{ s *FakeSMTP }

func (m mailerAdapter) Send(_ context.Context, to, subject, body string) error {
	return m.s.Send(to, subject, body)
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
	failErr error
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
	failErr := s.failErr
	s.mu.Unlock()
	if failErr != nil {
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
}

// FailAll makes the webhook sink return 500 on every subsequent delivery
// until cleared with FailAll(nil). Hits are still recorded. Used by
// DLQ scenario tests.
func (s *FakeWebhookSink) FailAll(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failErr = err
}

// ---- AlertSender adapters (port.AlertSender) -----------------------

// AsSender adapts FakeSMTP to port.AlertSender. The alert event is
// rendered to a minimal subject/body so tests can assert on "Sent".
func (s *FakeSMTP) AsSender() port.AlertSender { return smtpSender{s} }

type smtpSender struct{ *FakeSMTP }

func (a smtpSender) Send(_ context.Context, event *domain.AlertEvent) error {
	subject := fmt.Sprintf("[%s] %s", event.Event, event.MonitorName)
	body := fmt.Sprintf("monitor %s is %s (cause: %s)", event.MonitorName, event.Event, event.Cause)
	return a.FakeSMTP.Send("recipient@test.local", subject, body)
}

// AsSender adapts FakeTelegram to port.AlertSender. Records the event
// summary as a TelegramCall.
func (f *FakeTelegram) AsSender() port.AlertSender { return telegramSender{f} }

type telegramSender struct{ *FakeTelegram }

func (a telegramSender) Send(_ context.Context, event *domain.AlertEvent) error {
	text := fmt.Sprintf("[%s] %s — %s", event.Event, event.MonitorName, event.Cause)
	a.FakeTelegram.mu.Lock()
	a.FakeTelegram.calls = append(a.FakeTelegram.calls, TelegramCall{
		ChatID: event.MonitorID.String(),
		Text:   text,
	})
	a.FakeTelegram.mu.Unlock()
	return nil
}

// AsSender adapts FakeWebhookSink to port.AlertSender. Records an
// equivalent HTTP-looking hit and honours FailAll.
func (s *FakeWebhookSink) AsSender() port.AlertSender { return webhookSender{s} }

type webhookSender struct{ *FakeWebhookSink }

func (a webhookSender) Send(_ context.Context, event *domain.AlertEvent) error {
	body := fmt.Sprintf(`{"monitor_id":%q,"event":%q,"cause":%q}`,
		event.MonitorID, event.Event, event.Cause)
	a.FakeWebhookSink.mu.Lock()
	a.FakeWebhookSink.hits = append(a.FakeWebhookSink.hits, WebhookHit{
		Method:  "POST",
		Path:    "/",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(body),
	})
	failErr := a.FakeWebhookSink.failErr
	a.FakeWebhookSink.mu.Unlock()
	return failErr
}
