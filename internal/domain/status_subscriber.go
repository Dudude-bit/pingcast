package domain

import "time"

// StatusSubscriber is an email address that opted in to updates on a
// public status page. Double-opt-in: created with confirmed_at=NULL,
// flipped when the user clicks the confirmation link. Every outbound
// email carries unsubscribe_token in the link — one click, gone.
type StatusSubscriber struct {
	ID               int64
	Slug             string
	Email            string
	ConfirmToken     string
	UnsubscribeToken string
	ConfirmedAt      *time.Time
	CreatedAt        time.Time
	// Locale is the UI language the visitor used to subscribe. Drives
	// the language of outbound incident notification emails. nil →
	// default ("en") at the application layer.
	Locale *string
}

func (s StatusSubscriber) IsConfirmed() bool {
	return s.ConfirmedAt != nil
}
