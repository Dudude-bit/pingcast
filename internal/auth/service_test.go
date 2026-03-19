package auth_test

import (
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/internal/auth"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		slug string
		ok   bool
	}{
		{"myslug", true},
		{"my-slug-123", true},
		{"ab", false},
		{"UPPERCASE", false},
		{"login", false},
		{"admin", false},
		{"a-valid-slug", true},
		{"this-slug-is-way-too-long-for-validation-rules", false},
	}

	for _, tt := range tests {
		err := auth.ValidateSlug(tt.slug)
		if tt.ok && err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", tt.slug)
		}
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !auth.CheckPassword(hash, "mysecretpassword") {
		t.Error("CheckPassword returned false for correct password")
	}
	if auth.CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword returned true for wrong password")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := auth.NewRateLimiter(3, 1*time.Minute)

	if !rl.Allow("test@example.com") {
		t.Error("expected Allow to return true on first check")
	}

	rl.Record("test@example.com")
	rl.Record("test@example.com")
	rl.Record("test@example.com")

	if rl.Allow("test@example.com") {
		t.Error("expected Allow to return false after 3 recorded failures")
	}

	rl.Reset("test@example.com")
	if !rl.Allow("test@example.com") {
		t.Error("expected Allow to return true after Reset")
	}
}
