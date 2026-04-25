package domain

import "time"

// BlogSubscriber is an email opted in to the global PingCast newsletter.
// Distinct from StatusSubscriber, which is per-tenant status-page
// subscriber. Same double-opt-in mechanic.
type BlogSubscriber struct {
	ID               int64
	Email            string
	ConfirmToken     string
	UnsubscribeToken string
	ConfirmedAt      *time.Time
	CreatedAt        time.Time
	// Source tags where the subscriber signed up: "footer", "blog_sidebar",
	// "post_cta:status-pages-reduce-support-tickets", etc. Nullable.
	Source *string
	// Locale is the UI language the visitor used to subscribe. nil →
	// default ("en") at the application layer.
	Locale *string
}

func (s BlogSubscriber) IsConfirmed() bool {
	return s.ConfirmedAt != nil
}
