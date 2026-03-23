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
func ValidateMonitorInput(name string, intervalSeconds, alertAfterFailures int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 255 {
		return fmt.Errorf("name must be at most 255 characters")
	}
	if intervalSeconds < 30 {
		return fmt.Errorf("interval must be at least 30 seconds")
	}
	if intervalSeconds > 86400 {
		return fmt.Errorf("interval must be at most 24 hours")
	}
	if alertAfterFailures < 1 {
		return fmt.Errorf("alert_after_failures must be at least 1")
	}
	if alertAfterFailures > 100 {
		return fmt.Errorf("alert_after_failures must be at most 100")
	}
	return nil
}

// ValidateEmail checks that an email address has valid format.
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// ValidateChannelInput validates channel fields.
func ValidateChannelInput(name string, channelType ChannelType) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("channel name is required")
	}
	if len(name) > 255 {
		return fmt.Errorf("channel name must be at most 255 characters")
	}
	if !channelType.Valid() {
		return fmt.Errorf("invalid channel type: %s", channelType)
	}
	return nil
}
