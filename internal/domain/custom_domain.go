package domain

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CustomDomainStatus tracks the state machine of a Pro-tier custom
// domain through DNS validation and cert issuance.
type CustomDomainStatus string

const (
	CustomDomainPending   CustomDomainStatus = "pending"
	CustomDomainValidated CustomDomainStatus = "validated"
	CustomDomainActive    CustomDomainStatus = "active"
	CustomDomainFailed    CustomDomainStatus = "failed"
)

type CustomDomain struct {
	ID              int64
	UserID          uuid.UUID
	Hostname        string
	ValidationToken string
	Status          CustomDomainStatus
	LastError       *string
	DNSValidatedAt  *time.Time
	CertIssuedAt    *time.Time
	CreatedAt       time.Time
}

// hostnameRegex matches any RFC-1123 hostname of 1-253 chars. Intentionally
// permissive — exact DNS validity is enforced at validation-worker time.
var hostnameRegex = regexp.MustCompile(
	`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
)

// ReservedDomains is the set of hostnames a user can't claim. Covers
// loopback + our own apex + RFC reserved names.
var ReservedDomains = map[string]bool{
	"localhost":           true,
	"pingcast.io":         true,
	"www.pingcast.io":     true,
	"api.pingcast.io":     true,
	"status.pingcast.io":  true,
	"pingcast.kirillin.tech": true,
}

// ValidateCustomHostname enforces the rules of what a customer can
// register. Rejects: syntactically invalid names, reserved names, bare
// IP addresses, apex-domain claims on our own apex.
func ValidateCustomHostname(hostname string) error {
	h := strings.ToLower(strings.TrimSpace(hostname))
	if h == "" {
		return fmt.Errorf("hostname is required")
	}
	if len(h) > 253 {
		return fmt.Errorf("hostname too long (max 253 chars)")
	}
	if net.ParseIP(h) != nil {
		return fmt.Errorf("bare IP addresses not allowed")
	}
	if ReservedDomains[h] {
		return fmt.Errorf("hostname %q is reserved", h)
	}
	if strings.HasSuffix(h, ".pingcast.io") || strings.HasSuffix(h, ".pingcast.kirillin.tech") {
		return fmt.Errorf("custom hostnames must not sit under our apex")
	}
	if !hostnameRegex.MatchString(h) {
		return fmt.Errorf("%q is not a valid hostname", h)
	}
	return nil
}
