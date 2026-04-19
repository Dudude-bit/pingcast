package httperr

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// Envelope is the canonical error response shape (spec §1).
type Envelope struct {
	Error Inner `json:"error"`
}

type Inner struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write emits a canonical error envelope derived from err. 5xx
// responses also log the raw error (with request path) so debuggers
// aren't left in the dark.
func Write(c *fiber.Ctx, err error) error {
	status, code, msg := classify(err)
	if status >= 500 {
		slog.Error("internal error", "path", c.Path(), "error", err)
	}
	return c.Status(status).JSON(Envelope{Error: Inner{Code: code, Message: msg}})
}

// WriteMalformedJSON is a shortcut for 400 MALFORMED_JSON.
func WriteMalformedJSON(c *fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(Envelope{
		Error: Inner{Code: "MALFORMED_JSON", Message: "request body is not valid JSON"},
	})
}

// WriteMalformedParam is a shortcut for 400 MALFORMED_PARAM (typically
// UUID parse failures on path params).
func WriteMalformedParam(c *fiber.Ctx, paramName string) error {
	msg := "malformed path or query parameter"
	if paramName != "" {
		msg = "malformed parameter: " + paramName
	}
	return c.Status(fiber.StatusBadRequest).JSON(Envelope{
		Error: Inner{Code: "MALFORMED_PARAM", Message: msg},
	})
}

// WriteUnauthorized is a shortcut for 401 UNAUTHORIZED.
func WriteUnauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Envelope{
		Error: Inner{Code: "UNAUTHORIZED", Message: "authentication required"},
	})
}

// WriteForbiddenTenant is a shortcut for 403 FORBIDDEN_TENANT.
func WriteForbiddenTenant(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(Envelope{
		Error: Inner{Code: "FORBIDDEN_TENANT", Message: "access denied"},
	})
}

// WriteNotFound is a shortcut for 404 NOT_FOUND. Pass what="" for a
// generic message.
func WriteNotFound(c *fiber.Ctx, what string) error {
	msg := "resource not found"
	if what != "" {
		msg = what + " not found"
	}
	return c.Status(fiber.StatusNotFound).JSON(Envelope{
		Error: Inner{Code: "NOT_FOUND", Message: msg},
	})
}

// WriteConflict is a shortcut for 409 CONFLICT.
func WriteConflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(Envelope{
		Error: Inner{Code: "CONFLICT", Message: message},
	})
}

// WriteValidation is a shortcut for 422 VALIDATION_FAILED.
func WriteValidation(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(Envelope{
		Error: Inner{Code: "VALIDATION_FAILED", Message: message},
	})
}

// WriteRateLimited is a shortcut for 429 RATE_LIMITED with Retry-After.
func WriteRateLimited(c *fiber.Ctx, retryAfterSeconds int) error {
	c.Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	return c.Status(fiber.StatusTooManyRequests).JSON(Envelope{
		Error: Inner{Code: "RATE_LIMITED", Message: "too many requests"},
	})
}

// classify maps an arbitrary error to (status, code, safe message).
// Domain errors with their own Code/Message beat the sentinel fallback.
func classify(err error) (int, string, string) {
	var domErr *domain.DomainError
	if errors.As(err, &domErr) {
		status, code := domainStatusAndCode(domErr)
		msg := domErr.Message
		if msg == "" {
			msg = code
		}
		return status, code, msg
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return 404, "NOT_FOUND", "resource not found"
	case errors.Is(err, domain.ErrForbidden):
		return 403, "FORBIDDEN_TENANT", "access denied"
	case errors.Is(err, domain.ErrValidation):
		return 422, "VALIDATION_FAILED", "validation failed"
	case errors.Is(err, domain.ErrConflict):
		return 409, "CONFLICT", "conflict"
	case errors.Is(err, domain.ErrUserExists):
		// Spec §4: duplicate email during registration is a business
		// validation failure (422), with a generic message to avoid
		// leaking which emails are registered.
		return 422, "VALIDATION_FAILED", "email already registered"
	case errors.Is(err, domain.ErrIncidentExists):
		return 409, "CONFLICT", "active incident already exists"
	}

	var fe *fiber.Error
	if errors.As(err, &fe) {
		switch fe.Code {
		case fiber.StatusUnauthorized:
			return 401, "UNAUTHORIZED", "authentication required"
		case fiber.StatusNotFound:
			return 404, "NOT_FOUND", "resource not found"
		case fiber.StatusForbidden:
			return 403, "FORBIDDEN_TENANT", "access denied"
		case fiber.StatusBadRequest:
			return 400, "MALFORMED_JSON", "request body is not valid JSON"
		}
	}

	return 500, "INTERNAL_ERROR", "internal error"
}

// domainStatusAndCode maps a DomainError to the spec §1 top-level code.
// The DomainError's own Code field is intentionally ignored at this
// boundary: clients see a stable canonical code (VALIDATION_FAILED,
// NOT_FOUND, …) and the specific reason travels in the message. Internal
// sub-codes remain available to server-side callers via the DomainError.
func domainStatusAndCode(e *domain.DomainError) (int, string) {
	switch {
	case errors.Is(e, domain.ErrNotFound):
		return 404, "NOT_FOUND"
	case errors.Is(e, domain.ErrForbidden):
		return 403, "FORBIDDEN_TENANT"
	case errors.Is(e, domain.ErrValidation):
		return 422, "VALIDATION_FAILED"
	case errors.Is(e, domain.ErrConflict):
		return 409, "CONFLICT"
	}
	return 500, "INTERNAL_ERROR"
}

