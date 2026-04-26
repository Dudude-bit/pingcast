package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCertExpiry_ValidPEM_ReturnsNotAfter — round-trips a
// self-signed cert with a known NotAfter, asserts the parser returns
// it back. If this regresses, the renewal scheduler will silently
// store a wrong expiry on every issued cert and the renewal-loop
// threshold check (`expires_at < now+30d`) will fire at the wrong
// time — either too early (rate-limit risk with Let's Encrypt) or
// too late (cert expires in prod).
func TestParseCertExpiry_ValidPEM_ReturnsNotAfter(t *testing.T) {
	want := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	pemBytes := mintSelfSignedPEM(t, want)

	got, err := parseCertExpiry(pemBytes)
	require.NoError(t, err)
	// x509 stores NotAfter at second resolution; our minted cert has
	// no sub-second component so equality is fine.
	assert.True(t, got.Equal(want),
		"parsed expiry %v != minted NotAfter %v", got, want)
}

// TestParseCertExpiry_BundledChain_PicksLeaf — Let's Encrypt returns
// the leaf cert followed by intermediates concatenated as one PEM
// blob. The function must read the FIRST block (the leaf) — picking
// the intermediate's NotAfter would tell the renewal loop to wait
// years before renewing.
func TestParseCertExpiry_BundledChain_PicksLeaf(t *testing.T) {
	leafExpiry := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	intermediateExpiry := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	bundle := append(
		mintSelfSignedPEM(t, leafExpiry),
		mintSelfSignedPEM(t, intermediateExpiry)...,
	)

	got, err := parseCertExpiry(bundle)
	require.NoError(t, err)
	assert.True(t, got.Equal(leafExpiry),
		"got %v, want leaf %v (intermediate was %v)", got, leafExpiry, intermediateExpiry)
}

// TestParseCertExpiry_NoPEM_Errors — garbage input must return error,
// not zero-time silently. A zero-time would make the renewal scheduler
// think every cert expired in 0001-01-01 and immediately trigger an
// ACME flood.
func TestParseCertExpiry_NoPEM_Errors(t *testing.T) {
	_, err := parseCertExpiry([]byte("this is not a PEM block"))
	require.Error(t, err)
}

// TestParseCertExpiry_EmptyInput_Errors — same regression class as
// above. Empty bytes from a misbehaving ACME response shouldn't crash
// or silently succeed.
func TestParseCertExpiry_EmptyInput_Errors(t *testing.T) {
	_, err := parseCertExpiry(nil)
	require.Error(t, err)
}

// TestParseCertExpiry_MalformedDER_Errors — valid PEM wrapper but
// junk inside. Should surface as a parsing error, not a panic.
func TestParseCertExpiry_MalformedDER_Errors(t *testing.T) {
	bogus := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("definitely-not-DER"),
	})
	_, err := parseCertExpiry(bogus)
	require.Error(t, err)
}

// mintSelfSignedPEM produces a fresh ECDSA-P256 self-signed cert with
// the given NotAfter, encoded as one PEM block. NotBefore is set far
// enough in the past that the cert is "valid" if anything ever bothers
// checking — we don't, but defensive.
func mintSelfSignedPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notAfter.Add(-365 * 24 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
