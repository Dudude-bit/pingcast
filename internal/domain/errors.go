package domain

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("validation error")
	ErrConflict   = errors.New("conflict")
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

// Error constructors
func NewNotFoundError(code, message string) *DomainError {
	return &DomainError{Err: ErrNotFound, Code: code, Message: message}
}

func NewForbiddenError(code, message string) *DomainError {
	return &DomainError{Err: ErrForbidden, Code: code, Message: message}
}

func NewValidationError(code, message string) *DomainError {
	return &DomainError{Err: ErrValidation, Code: code, Message: message}
}

func NewConflictError(code, message string) *DomainError {
	return &DomainError{Err: ErrConflict, Code: code, Message: message}
}
