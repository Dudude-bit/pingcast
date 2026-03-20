package app_test

import (
	"testing"

	"github.com/kirillinakin/pingcast/internal/app"
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
		err := app.ValidateSlug(tt.slug)
		if tt.ok && err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", tt.slug)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := app.ValidatePassword("short"); err == nil {
		t.Error("expected error for short password")
	}
	if err := app.ValidatePassword("longenough"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := app.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !app.CheckPassword(hash, "mysecretpassword") {
		t.Error("CheckPassword returned false for correct password")
	}
	if app.CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword returned true for wrong password")
	}
}
