package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// BlogSubscriptionService owns the lifecycle of newsletter subscribers
// to the global PingCast newsletter (separate from per-tenant status-page
// subscribers in SubscriptionService). Same double-opt-in mechanic.
type BlogSubscriptionService struct {
	repo    port.BlogSubscriberRepo
	mailer  port.Mailer
	baseURL string
}

func NewBlogSubscriptionService(repo port.BlogSubscriberRepo, mailer port.Mailer, baseURL string) *BlogSubscriptionService {
	return &BlogSubscriptionService{
		repo:    repo,
		mailer:  mailer,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Subscribe creates a pending subscriber and fires the confirmation
// email. Duplicate email is treated as success silently — same
// rationale as status-page subscribe (don't leak who's subscribed).
// `source` tags where on the site the signup happened. `locale`
// selects the email language ("en" / "ru"); empty / unknown defaults
// to English so old clients keep working.
func (s *BlogSubscriptionService) Subscribe(ctx context.Context, email string, source *string, locale string) error {
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

	var localePtr *string
	if locale != "" {
		localePtr = &locale
	}
	sub, err := s.repo.Create(ctx, email, confirmToken, unsubToken, source, localePtr)
	if err != nil {
		// Duplicate email — treat as success silently.
		slog.Info("blog subscribe: possibly duplicate, treating as success",
			"error", err)
		return nil
	}

	confirmURL := fmt.Sprintf("%s/api/newsletter/confirm?token=%s",
		s.baseURL, sub.ConfirmToken)
	subject, body := blogConfirmEmail(toEmailLocale(locale), confirmURL)
	if mErr := s.mailer.Send(ctx, email, subject, body); mErr != nil {
		slog.Error("send blog confirmation email", "error", mErr)
		// Don't fail the subscribe — row exists, user can retry.
	}
	return nil
}

func (s *BlogSubscriptionService) Confirm(ctx context.Context, confirmToken string) (*domain.BlogSubscriber, error) {
	return s.repo.Confirm(ctx, confirmToken)
}

func (s *BlogSubscriptionService) Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.BlogSubscriber, error) {
	return s.repo.Unsubscribe(ctx, unsubscribeToken)
}

func (s *BlogSubscriptionService) Count(ctx context.Context) (int64, error) {
	return s.repo.CountConfirmed(ctx)
}
