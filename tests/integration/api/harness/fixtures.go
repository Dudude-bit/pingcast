//go:build integration

package harness

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

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
