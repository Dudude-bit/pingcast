package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrValidation     = errors.New("validation error")
	ErrConflict       = errors.New("conflict")
	ErrIncidentExists = errors.New("active incident already exists for this monitor")
	ErrUserExists     = errors.New("user already exists")
)

// DomainError wraps a sentinel error with a human-readable message and machine-readable code.
type DomainError struct {
	Err     error
	Code    string
	Message string
}

func (e *DomainError) Error() string {
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// DeliveryError represents a failed alert channel delivery with classification.
type DeliveryError struct {
	StatusCode int    // HTTP status from remote service (0 if not HTTP)
	Reason     string // "timeout", "auth_error", "rate_limited", "network_error", "unknown"
	Err        error
}

func (e *DeliveryError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("delivery failed (status %d, reason %s): %v", e.StatusCode, e.Reason, e.Err)
	}
	return fmt.Sprintf("delivery failed (reason %s): %v", e.Reason, e.Err)
}

func (e *DeliveryError) Unwrap() error { return e.Err }

func NewDeliveryError(reason string, statusCode int, err error) *DeliveryError {
	return &DeliveryError{Reason: reason, StatusCode: statusCode, Err: err}
}

// NewValidationError builds the most common kind — auth.go uses it for
// slug/password rejections. NewNotFoundError stays around because the
// HTTP error-envelope handler test needs a way to materialise a
// DomainError; production code reaches for the sentinels via errors.Is.
func NewValidationError(code, message string) *DomainError {
	return &DomainError{Err: ErrValidation, Code: code, Message: message}
}

func NewNotFoundError(code, message string) *DomainError {
	return &DomainError{Err: ErrNotFound, Code: code, Message: message}
}
