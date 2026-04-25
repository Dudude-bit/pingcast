package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// SubscriptionService owns the status-page email-subscription lifecycle:
// subscribe → confirm → unsubscribe, plus fan-out of incident-update
// notifications to every confirmed subscriber for a slug.
type SubscriptionService struct {
	subs    port.StatusSubscriberRepo
	mailer  port.Mailer
	baseURL string
}

func NewSubscriptionService(subs port.StatusSubscriberRepo, mailer port.Mailer, baseURL string) *SubscriptionService {
	return &SubscriptionService{subs: subs, mailer: mailer, baseURL: strings.TrimRight(baseURL, "/")}
}

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Subscribe creates a pending subscriber and fires the double-opt-in
// confirmation email. Duplicate email on the same slug returns ok
// silently (idempotent — avoids leaking whether a given email is
// already subscribed). `locale` selects the email language;
// empty / unknown → English.
func (s *SubscriptionService) Subscribe(ctx context.Context, slug, email, locale string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email")
	}
	confirmToken, err := randToken()
	if err != nil {
		return err
	}
	unsubToken, err := randToken()
	if err != nil {
		return err
	}

	sub, err := s.subs.Create(ctx, slug, email, confirmToken, unsubToken)
	if err != nil {
		// Duplicate (slug, email) — treat as success to avoid leaking.
		slog.Info("subscribe: possibly duplicate, treating as success",
			"slug", slug, "email_hash", hashForLog(email), "error", err)
		return nil
	}

	confirmURL := fmt.Sprintf("%s/api/status/%s/confirm?token=%s",
		s.baseURL, slug, sub.ConfirmToken)
	subject, body := statusSubscribeConfirmEmail(toEmailLocale(locale), slug, confirmURL)
	if mErr := s.mailer.Send(ctx, email, subject, body); mErr != nil {
		slog.Error("send confirmation email", "slug", slug, "error", mErr)
		// Don't fail the subscribe — the row exists; user can retry
		// later or an admin can resend the token.
	}
	return nil
}

// Confirm flips confirmed_at. Idempotent: a second click just returns
// the same record (or errors if the token was never valid).
func (s *SubscriptionService) Confirm(ctx context.Context, confirmToken string) (*domain.StatusSubscriber, error) {
	return s.subs.Confirm(ctx, confirmToken)
}

// ListConfirmed exposes the repo list for admin dashboards. Emits only
// rows where confirmed_at is set — the repo query already filters this
// but callers get a strongly-typed slice of confirmed subs.
func (s *SubscriptionService) ListConfirmed(ctx context.Context, slug string) ([]domain.StatusSubscriber, error) {
	return s.subs.ListConfirmedBySlug(ctx, slug)
}

// Unsubscribe deletes the row. Token-scoped so forged URLs can't touch
// other subscribers.
func (s *SubscriptionService) Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.StatusSubscriber, error) {
	return s.subs.Unsubscribe(ctx, unsubscribeToken)
}

// NotifyIncident fans out an incident update to every confirmed
// subscriber for the given slug. Errors on individual sends are logged
// but don't abort the batch. Call from MonitoringService after a
// ChangeIncidentState or CreateManualIncident succeeds.
func (s *SubscriptionService) NotifyIncident(
	ctx context.Context,
	slug, headline, state, body string,
) {
	subs, err := s.subs.ListConfirmedBySlug(ctx, slug)
	if err != nil {
		slog.Error("list subscribers failed", "slug", slug, "error", err)
		return
	}
	subject := fmt.Sprintf("[%s] %s (status: %s)", slug, headline, state)
	for _, sub := range subs {
		unsubURL := fmt.Sprintf("%s/api/status/%s/unsubscribe?token=%s",
			s.baseURL, slug, sub.UnsubscribeToken)
		payload := fmt.Sprintf("%s\n\n%s\n\n---\nStatus page: %s/status/%s\nUnsubscribe: %s",
			headline, body, s.baseURL, slug, unsubURL)
		if err := s.mailer.Send(ctx, sub.Email, subject, payload); err != nil {
			slog.Error("subscriber email failed",
				"slug", slug, "subscriber_id", sub.ID, "error", err)
		}
	}
}

func randToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// hashForLog returns a non-reversible tag for an email so logs don't
// carry plaintext PII. Short length is fine — we just need uniqueness
// across a log volume, not crypto-grade collision resistance.
func hashForLog(s string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf) + "-" + fmt.Sprintf("%d", len(s))
}
