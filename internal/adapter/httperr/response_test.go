package httperr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kirillinakin/pingcast/internal/domain"
)

func TestClassifyHTTPError_Table(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"not-found", domain.ErrNotFound, 404, "resource not found"},
		{"validation", domain.ErrValidation, 422, "validation failed"},
		{"forbidden", domain.ErrForbidden, 403, "access denied"},
		{"conflict", domain.ErrConflict, 409, "conflict"},
		{"user-exists", domain.ErrUserExists, 422, "email already registered"},
		{"wrapped-not-found", fmt.Errorf("wrap: %w", domain.ErrNotFound), 404, "resource not found"},
		{"unclassified", errors.New("boom"), 500, "internal error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStatus, gotMsg := ClassifyHTTPError(c.err)
			if gotStatus != c.wantStatus || gotMsg != c.wantMsg {
				t.Fatalf("got (%d, %q), want (%d, %q)",
					gotStatus, gotMsg, c.wantStatus, c.wantMsg)
			}
		})
	}
}
