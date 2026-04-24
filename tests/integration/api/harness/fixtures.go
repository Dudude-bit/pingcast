//go:build integration

package harness

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// User is a minimal projection of the registered user returned by
// RegisterAndLoginUser. Tests reach for ID when they need to seed
// data directly via SQL bypassing the HTTP surface.
type User struct {
	ID    uuid.UUID
	Email string
	Slug  string
}

var uniqueCounter uint64

func nextUnique() uint64 { return atomic.AddUint64(&uniqueCounter, 1) }

type registerRequest struct {
	Email    string `json:"email"`
	Slug     string `json:"slug"`
	Password string `json:"password"`
}

// RegisterAndLogin creates a user via /api/auth/register. Empty strings
// fall back to unique auto-generated values. The returned session has
// the authentication cookie set.
func (h *Harness) RegisterAndLogin(t *testing.T, email, password string) *Session {
	t.Helper()

	n := nextUnique()
	if email == "" {
		email = fmt.Sprintf("u-%d@test.local", n)
	}
	if password == "" {
		password = "password123"
	}
	slug := slugFromEmail(email, n)

	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", registerRequest{
		Email:    email,
		Slug:     slug,
		Password: password,
	})
	if resp.Status != 200 && resp.Status != 201 {
		t.Fatalf("register %q: status=%d body=%s", email, resp.Status, resp.Body)
	}
	return s
}

// TwoSessions returns two logged-in sessions bound to different users.
func (h *Harness) TwoSessions(t *testing.T) (*Session, *Session) {
	t.Helper()
	return h.RegisterAndLogin(t, "", ""), h.RegisterAndLogin(t, "", "")
}

// RegisterAndLoginUser is like RegisterAndLogin but also returns the
// created user's ID/email/slug. Useful when a test needs to seed rows
// directly via SQL (e.g. a Pro-only table) for the same user.
func (h *Harness) RegisterAndLoginUser(t *testing.T) (*Session, User) {
	t.Helper()

	n := nextUnique()
	email := fmt.Sprintf("u-%d@test.local", n)
	slug := fmt.Sprintf("u-%d", n)

	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", registerRequest{
		Email:    email,
		Slug:     slug,
		Password: "password123",
	})
	if resp.Status != 200 && resp.Status != 201 {
		t.Fatalf("register %q: status=%d body=%s", email, resp.Status, resp.Body)
	}

	var body struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	resp.JSON(t, &body)
	id, err := uuid.Parse(body.User.ID)
	if err != nil {
		t.Fatalf("parse user.id %q: %v", body.User.ID, err)
	}
	return s, User{ID: id, Email: email, Slug: slug}
}

// PromoteToPro flips the user's plan to 'pro' via direct SQL so the
// test doesn't have to pump a LemonSqueezy webhook. Use this when the
// test is about the Pro feature itself, not the payment flow.
func (h *Harness) PromoteToPro(t *testing.T, userID uuid.UUID) {
	t.Helper()
	_, err := h.App.Pool.Exec(context.Background(),
		`UPDATE users SET plan = 'pro' WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("promote %s to pro: %v", userID, err)
	}
}

// SetSubscriptionVariant writes the LemonSqueezy price-variant label
// (e.g. 'founder' / 'retail') on a user. Used by founder-cap tests to
// seed synthetic founders without touching the webhook.
func (h *Harness) SetSubscriptionVariant(t *testing.T, userID uuid.UUID, variant string) {
	t.Helper()
	_, err := h.App.Pool.Exec(context.Background(),
		`UPDATE users SET plan = 'pro', subscription_variant = $1 WHERE id = $2`,
		variant, userID)
	if err != nil {
		t.Fatalf("set variant %s on %s: %v", variant, userID, err)
	}
}

// slugFromEmail derives a slug from the email local-part, falling back
// to a counter suffix to guarantee uniqueness and pattern compliance
// (^[a-z0-9-]{3,30}$).
func slugFromEmail(email string, counter uint64) string {
	local, _, _ := strings.Cut(email, "@")
	local = strings.ToLower(local)
	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}
	slug := b.String()
	if len(slug) < 3 {
		slug = fmt.Sprintf("u-%d", counter)
	}
	if len(slug) > 20 {
		slug = slug[:20]
	}
	// append counter to ensure uniqueness across multiple users with
	// similar local-parts within a single test run
	return fmt.Sprintf("%s-%d", slug, counter)
}
