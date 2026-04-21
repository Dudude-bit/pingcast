package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"time"
)

// sslDialTimeout caps a single cert probe. Short because we're checking
// hundreds of monitors in a daily batch; misconfigured hosts shouldn't
// stall the whole scan.
const sslDialTimeout = 10 * time.Second

// CheckSSLExpiry opens a TLS handshake to the host in targetURL and
// returns the leaf cert's NotAfter time. Only the hostname is used —
// port defaults to 443 if the URL doesn't specify one. Pure probe: no
// GET, no body, no redirects.
func CheckSSLExpiry(ctx context.Context, targetURL string) (time.Time, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "https" {
		return time.Time{}, fmt.Errorf("ssl probe only applies to https urls, got %q", u.Scheme)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}

	dialer := &net.Dialer{Timeout: sslDialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), &tls.Config{
		ServerName: host,
		// We want to read the cert even if it's expired/self-signed so
		// the expiring-soon alert can still surface. Verification is
		// the business of the regular HTTP check, not the expiry probe.
		InsecureSkipVerify: true, //nolint:gosec // G402: expiry probe reads the cert regardless of trust chain
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("tls dial %s:%s: %w", host, port, err)
	}
	defer func() { _ = conn.Close() }()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, fmt.Errorf("no peer certificates for %s", host)
	}
	return certs[0].NotAfter, nil
}

// DaysUntilExpiry is a small helper so callers can bucket alerts by the
// 14/7/1 day thresholds without duplicating the truncation logic.
func DaysUntilExpiry(notAfter, now time.Time) int {
	diff := notAfter.Sub(now)
	return int(diff / (24 * time.Hour))
}
