package domain

import (
	"fmt"
	"net/mail"
	"slices"
	"strings"
)

// ValidEnum checks if a value is in the allowed list.
func ValidEnum[T comparable](value T, allowed []T) bool {
	return slices.Contains(allowed, value)
}

// ValidateMonitorInput validates monitor fields shared by Create and Update.
// Errors returned wrap ErrValidation so the canonical envelope classifier
// emits 422 VALIDATION_FAILED with the embedded message.
func ValidateMonitorInput(name string, intervalSeconds, alertAfterFailures int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewValidationError("MONITOR_NAME_REQUIRED", "name is required")
	}
	if len(name) > 255 {
		return NewValidationError("MONITOR_NAME_TOO_LONG", "name must be at most 255 characters")
	}
	if intervalSeconds < 30 {
		return NewValidationError("INTERVAL_TOO_SHORT", "interval must be at least 30 seconds")
	}
	if intervalSeconds > 86400 {
		return NewValidationError("INTERVAL_TOO_LONG", "interval must be at most 24 hours")
	}
	if alertAfterFailures < 1 {
		return NewValidationError("ALERT_THRESHOLD_TOO_LOW", "alert_after_failures must be at least 1")
	}
	if alertAfterFailures > 100 {
		return NewValidationError("ALERT_THRESHOLD_TOO_HIGH", "alert_after_failures must be at most 100")
	}
	return nil
}

// ValidateEmail checks that an email address has valid format.
func ValidateEmail(email string) error {
	if email == "" {
		return NewValidationError("EMAIL_REQUIRED", "email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return NewValidationError("EMAIL_INVALID", "invalid email format")
	}
	return nil
}

// ValidateChannelInput validates channel fields.
func ValidateChannelInput(name string, channelType ChannelType) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewValidationError("CHANNEL_NAME_REQUIRED", "channel name is required")
	}
	if len(name) > 255 {
		return NewValidationError("CHANNEL_NAME_TOO_LONG", "channel name must be at most 255 characters")
	}
	if !channelType.Valid() {
		return NewValidationError("INVALID_CHANNEL_TYPE", fmt.Sprintf("invalid channel type: %s", channelType))
	}
	return nil
}
