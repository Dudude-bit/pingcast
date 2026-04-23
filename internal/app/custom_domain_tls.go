package app

import "crypto/tls"

// insecureTLSConfig returns a TLS config with verification disabled —
// used by the custom-domain DNS-validation probe, which runs *before*
// the customer's cert is issued. Segregated here so adjacent probe
// changes don't leak G402 findings through the rest of the file.
func insecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // G402: the validation probe reads a token, not user data
}
