// Package acme wires go-acme/lego as the real CertProvisioner. The
// adapter lives here so the heavy lego dependency stays out of /internal/app.
//
// Wiring it in production requires three pieces of infra plumbing the
// app itself can't own:
//
//  1. Inbound `:80` reachable on every custom hostname so Let's Encrypt
//     can hit `/.well-known/acme-challenge/<token>` during HTTP-01 challenge.
//     With Traefik this means a router on the public entrypoint that
//     forwards `/.well-known/acme-challenge/*` for the customer's hostname
//     to *this* service's `:5002` (or whatever ACME_HTTP01_PORT is).
//
//  2. After issuance, the cert+key live in `custom_domain_certs`. An
//     external ingress (Traefik file-provider, Caddy dynamic config)
//     must read them and serve TLS for that hostname. Until that
//     plumbing exists, an issued cert is dead-stored — so this adapter
//     stays opt-in via env (`CERT_PROVIDER=lego`) and the default
//     remains NoopCertProvisioner.
//
//  3. A renewal loop (CustomDomainService.RunRenewalsOnce, scheduled
//     daily by the scheduler service) scans `custom_domain_certs` for
//     `expires_at < now+30d` and re-runs Provision. Lego's cert lifetime
//     is 90 days; we renew at day 60 so a single transient ACME outage
//     doesn't expire prod.
package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	apppkg "github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/port"
)

// Compile-time check.
var _ apppkg.CertProvisioner = (*LegoProvisioner)(nil)

// Config tunes the adapter. CADirURL defaults to Let's Encrypt
// production; tests / staging use the staging URL to avoid hitting
// the LE rate limits.
type Config struct {
	// Email registered with the ACME directory. Required by Let's
	// Encrypt for renewal-failure notifications.
	Email string
	// CADirURL is the ACME directory endpoint. Defaults to LE prod
	// (`https://acme-v02.api.letsencrypt.org/directory`). Set the
	// staging URL for tests / dev.
	CADirURL string
	// HTTP01Port is the local port the HTTP-01 challenge server binds.
	// Traefik must forward `/.well-known/acme-challenge/*` to this port
	// for whatever hostname is being validated. Defaults to 5002.
	HTTP01Port string
}

// LegoProvisioner issues real ACME certificates and persists them via
// the CertStore. One in-process ACME account is used for all hostnames
// — appropriate for a single-tenant operator.
type LegoProvisioner struct {
	client *lego.Client
	store  port.CustomDomainCertRepo
	repo   port.CustomDomainRepo
	cfg    Config
}

// New registers an ACME account on first construction (idempotent
// against Let's Encrypt — they return the existing registration if the
// account key is reused). Returns an error if registration fails so
// bootstrap can fall back to NoopCertProvisioner with a clear log line.
func New(cfg Config, store port.CustomDomainCertRepo, repo port.CustomDomainRepo) (*LegoProvisioner, error) {
	if cfg.CADirURL == "" {
		cfg.CADirURL = lego.LEDirectoryProduction
	}
	if cfg.HTTP01Port == "" {
		cfg.HTTP01Port = "5002"
	}
	if cfg.Email == "" {
		return nil, errors.New("acme: email is required for the ACME account")
	}

	// Account private key — a deterministic per-deployment key would be
	// nicer (so the same registration is reused across restarts), but
	// for v1 a fresh in-process key on each boot is fine; LE accepts
	// "new" registrations from re-keyed clients gracefully.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	user := &acmeUser{Email: cfg.Email, key: priv}

	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = cfg.CADirURL
	legoCfg.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return nil, fmt.Errorf("lego.NewClient: %w", err)
	}

	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", cfg.HTTP01Port)); err != nil {
		return nil, fmt.Errorf("set HTTP-01 provider: %w", err)
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("acme account registration: %w", err)
	}
	user.registration = reg

	return &LegoProvisioner{
		client: client,
		store:  store,
		repo:   repo,
		cfg:    cfg,
	}, nil
}

// Provision issues (or re-issues) a cert for the given hostname.
// Persists cert+key+chain in `custom_domain_certs` so the external
// ingress can serve TLS without a fresh ACME round-trip.
func (p *LegoProvisioner) Provision(ctx context.Context, hostname string) error {
	domain, err := p.repo.GetByHostname(ctx, hostname)
	if err != nil {
		return fmt.Errorf("lookup domain %q: %w", hostname, err)
	}

	res, err := p.client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{hostname},
		Bundle:  true,
	})
	if err != nil {
		return fmt.Errorf("obtain cert for %q: %w", hostname, err)
	}

	expires, err := parseCertExpiry(res.Certificate)
	if err != nil {
		return fmt.Errorf("parse cert expiry: %w", err)
	}

	if storeErr := p.store.Upsert(ctx, port.CustomDomainCert{
		CustomDomainID: domain.ID,
		CertPEM:        string(res.Certificate),
		KeyPEM:         string(res.PrivateKey),
		ChainPEM:       string(res.IssuerCertificate),
		ExpiresAt:      expires,
	}); storeErr != nil {
		return fmt.Errorf("store cert for %q: %w", hostname, storeErr)
	}

	slog.Info("acme: issued cert",
		"hostname", hostname,
		"expires_at", expires,
		"chain_bytes", len(res.IssuerCertificate),
	)
	return nil
}

// parseCertExpiry pulls NotAfter out of the leaf cert in a PEM bundle.
// Used by the renewal loop and the issuance success log.
func parseCertExpiry(pemBytes []byte) (time.Time, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return time.Time{}, errors.New("no PEM block found in certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}

// acmeUser implements registration.User for lego. Holds the account
// key and the registration response from LE.
type acmeUser struct {
	Email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }
