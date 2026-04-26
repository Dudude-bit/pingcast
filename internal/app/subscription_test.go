package app_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// recordingMailer captures every Send so per-subscriber locale tests
// can read back what each address received.
type recordingMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

type sentMail struct {
	to      string
	subject string
	body    string
}

func (r *recordingMailer) Send(_ context.Context, to, subject, body string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, sentMail{to: to, subject: subject, body: body})
	return nil
}

func (r *recordingMailer) findByEmail(addr string) (sentMail, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.sent {
		if m.to == addr {
			return m, true
		}
	}
	return sentMail{}, false
}

func ptr[T any](v T) *T { return &v }

// TestNotifyIncident_FanoutPerSubscriberLocale — the safety net for
// "Russian subscriber gets English incident notification". Each
// subscriber's Locale field must drive the language of THEIR email,
// not whatever the first subscriber happened to have.
func TestNotifyIncident_FanoutPerSubscriberLocale(t *testing.T) {
	mailer := &recordingMailer{}
	subRepo := mocks.NewMockStatusSubscriberRepo(t)

	subRepo.EXPECT().
		ListConfirmedBySlug(mock.Anything, "acme").
		Return([]domain.StatusSubscriber{
			{ID: 1, Slug: "acme", Email: "ru@x.test", UnsubscribeToken: "tok-ru", Locale: ptr("ru")},
			{ID: 2, Slug: "acme", Email: "en@x.test", UnsubscribeToken: "tok-en", Locale: ptr("en")},
			{ID: 3, Slug: "acme", Email: "nil@x.test", UnsubscribeToken: "tok-nil", Locale: nil},
		}, nil).
		Once()

	svc := app.NewSubscriptionService(subRepo, mailer, "https://pingcast.io")
	svc.NotifyIncident(context.Background(), "acme", "DB unreachable", "investigating", "Looking into it.")

	require.Len(t, mailer.sent, 3, "every confirmed subscriber must receive exactly one email")

	ruMail, ok := mailer.findByEmail("ru@x.test")
	require.True(t, ok)
	assert.Contains(t, ruMail.subject, "статус",
		"RU subscriber subject should be in Russian: %q", ruMail.subject)

	enMail, ok := mailer.findByEmail("en@x.test")
	require.True(t, ok)
	assert.Contains(t, enMail.subject, "status",
		"EN subscriber subject should be in English: %q", enMail.subject)
	assertNoCyrillic(t, enMail.subject, "EN subject must not leak Cyrillic")

	nilMail, ok := mailer.findByEmail("nil@x.test")
	require.True(t, ok)
	assert.Contains(t, nilMail.subject, "status",
		"nil-locale subscriber should default to English: %q", nilMail.subject)
	assertNoCyrillic(t, nilMail.subject, "nil-locale must not default to RU")
}

// TestNotifyIncident_PerSubscriberUnsubscribeToken — every email must
// carry THAT subscriber's own unsubscribe token. If we accidentally
// reuse the first subscriber's token, one click unsubscribes them
// instead of the actual reader.
func TestNotifyIncident_PerSubscriberUnsubscribeToken(t *testing.T) {
	mailer := &recordingMailer{}
	subRepo := mocks.NewMockStatusSubscriberRepo(t)

	subRepo.EXPECT().
		ListConfirmedBySlug(mock.Anything, "acme").
		Return([]domain.StatusSubscriber{
			{ID: 1, Slug: "acme", Email: "alice@x.test", UnsubscribeToken: "tok-ALICE-abc"},
			{ID: 2, Slug: "acme", Email: "bob@x.test", UnsubscribeToken: "tok-BOB-xyz"},
		}, nil).
		Once()

	svc := app.NewSubscriptionService(subRepo, mailer, "https://pingcast.io")
	svc.NotifyIncident(context.Background(), "acme", "Down", "investigating", "Hold on.")

	alice, _ := mailer.findByEmail("alice@x.test")
	bob, _ := mailer.findByEmail("bob@x.test")

	assert.Contains(t, alice.body, "tok-ALICE-abc",
		"Alice's email must carry Alice's unsub token")
	assert.NotContains(t, alice.body, "tok-BOB-xyz",
		"Alice's email must NOT carry Bob's token (one click would unsub the wrong person)")
	assert.Contains(t, bob.body, "tok-BOB-xyz")
}

// assertNoCyrillic fails if the string contains any Cyrillic codepoint.
// Used to catch language leakage in EN-locale assertions.
func assertNoCyrillic(t *testing.T, s, msg string) {
	t.Helper()
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			t.Errorf("%s: contains Cyrillic %q in %q", msg, r, s)
			return
		}
	}
	_ = strings.TrimSpace // keep import alive
}
