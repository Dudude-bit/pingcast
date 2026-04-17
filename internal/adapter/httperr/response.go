package httperr

import (
	"errors"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// ClassifyHTTPError maps a domain error to an HTTP status code and a
// client-safe message. Unclassified errors collapse to 500 / "internal
// error"; the raw error text is the caller's responsibility to log.
//
// ErrUserExists intentionally maps to a generic "registration failed"
// message to prevent user-enumeration via API response body.
func ClassifyHTTPError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return 404, "not found"
	case errors.Is(err, domain.ErrValidation):
		return 400, "invalid request"
	case errors.Is(err, domain.ErrForbidden):
		return 403, "forbidden"
	case errors.Is(err, domain.ErrConflict):
		return 409, "conflict"
	case errors.Is(err, domain.ErrUserExists):
		return 400, "registration failed"
	default:
		return 500, "internal error"
	}
}
