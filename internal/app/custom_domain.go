package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// CertProvisioner is the port the app uses to ask an ACME client (or
// Traefik admin API, or whatever sits in front of the edge) to acquire
// a TLS cert for the given hostname. Intentionally simple: we don't
// expose the ACME flow itself — adapters decide whether to do HTTP-01,
// DNS-01, or delegate to Traefik's own provisioner.
type CertProvisioner interface {
	Provision(ctx context.Context, hostname string) error
}

// NoopCertProvisioner logs "would provision" and returns nil. Used in
// dev + CI; prod swaps in a Traefik-admin or lego-based impl.
type NoopCertProvisioner struct{}

func (NoopCertProvisioner) Provision(ctx context.Context, hostname string) error {
	slog.Info("cert provisioner noop (no real ACME wired)", "hostname", hostname)
	return nil
}

// CustomDomainService owns the Pro-tier custom-domain lifecycle —
// request (pending) → DNS validated → cert issued (active) → routing
// live. Runs a background worker that advances rows through the state
// machine.
type CustomDomainService struct {
	repo     port.CustomDomainRepo
	certRepo port.CustomDomainCertRepo
	acme     CertProvisioner
	baseURL  string

	// hostnameCache is populated from ListActiveHostnames and refreshed
	// whenever a domain flips to active or gets deleted. Read path is
	// the request hot-path, so we snapshot under RWMutex.
	mu            sync.RWMutex
	hostnameCache map[string]uuid.UUID
}

// NewCustomDomainService constructs the service. certRepo may be nil
// in tests + dev where the renewal loop isn't exercised; production
// must pass a real one or RunRenewalsOnce becomes a no-op.
func NewCustomDomainService(repo port.CustomDomainRepo, certRepo port.CustomDomainCertRepo, acme CertProvisioner, baseURL string) *CustomDomainService {
	return &CustomDomainService{
		repo:          repo,
		certRepo:      certRepo,
		acme:          acme,
		baseURL:       strings.TrimRight(baseURL, "/"),
		hostnameCache: map[string]uuid.UUID{},
	}
}

// WellKnownPath is the URL path customers must serve on their hostname
// during validation. Returns the exact string the worker probes for,
// so the dashboard + docs can show it without duplication.
func WellKnownPath(token string) string {
	return "/.well-known/pingcast/" + token
}

// RequestDomain creates a pending row and returns it. Dashboard shows
// the hostname + the .well-known instructions; the worker takes over
// from there.
func (s *CustomDomainService) RequestDomain(ctx context.Context, userID uuid.UUID, hostname string) (*domain.CustomDomain, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if err := domain.ValidateCustomHostname(hostname); err != nil {
		return nil, err
	}
	token, err := randToken()
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, userID, hostname, token)
}

// ListDomains returns every custom domain the user owns, regardless of
// status — the dashboard surfaces failures so users can retry.
func (s *CustomDomainService) ListDomains(ctx context.Context, userID uuid.UUID) ([]domain.CustomDomain, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *CustomDomainService) DeleteDomain(ctx context.Context, id int64, userID uuid.UUID) error {
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}
	s.refreshHostnameCache(ctx)
	return nil
}

// LookupHostname returns the user_id that owns a given custom hostname,
// or the ok=false if the hostname isn't registered/active. Read path
// for the host-header middleware.
func (s *CustomDomainService) LookupHostname(hostname string) (uuid.UUID, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid, ok := s.hostnameCache[strings.ToLower(hostname)]
	return uid, ok
}

// PreloadHostnameCache is called on app startup so the first request
// hitting a custom domain doesn't pay a DB roundtrip. Exported because
// bootstrap wires this explicitly; internal callers go through
// refreshHostnameCache.
func (s *CustomDomainService) PreloadHostnameCache(ctx context.Context) {
	s.refreshHostnameCache(ctx)
}

// refreshHostnameCache reloads the in-process map from Postgres.
// Called on startup + whenever a domain is activated or deleted.
func (s *CustomDomainService) refreshHostnameCache(ctx context.Context) {
	hosts, err := s.repo.ListActiveHostnames(ctx)
	if err != nil {
		slog.Error("refresh hostname cache failed", "error", err)
		return
	}
	s.mu.Lock()
	s.hostnameCache = hosts
	s.mu.Unlock()
}

// RunValidationOnce iterates every non-active row and advances it
// through DNS probe + cert provisioning. Called on a ticker by the
// scheduler, or directly by tests.
func (s *CustomDomainService) RunValidationOnce(ctx context.Context) {
	rows, err := s.repo.ListPending(ctx)
	if err != nil {
		slog.Error("list pending custom domains failed", "error", err)
		return
	}
	activatedAny := false
	for _, d := range rows {
		switch d.Status {
		case domain.CustomDomainPending, domain.CustomDomainFailed:
			s.validateDNS(ctx, d)
		case domain.CustomDomainValidated:
			if s.provisionCert(ctx, d) {
				activatedAny = true
			}
		}
	}
	if activatedAny {
		s.refreshHostnameCache(ctx)
	}
}

// validateDNS probes https://<hostname>/.well-known/pingcast/<token>
// over TLS-verify-skipped. If the body matches the token exactly, we
// flip status → validated and move on.
func (s *CustomDomainService) validateDNS(ctx context.Context, d domain.CustomDomain) {
	url := "https://" + d.Hostname + WellKnownPath(d.ValidationToken)
	// Short timeout — a misbehaving hostname shouldn't stall the batch.
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		s.markFailed(ctx, d.ID, fmt.Sprintf("build probe request: %v", err))
		return
	}
	// InsecureSkipVerify: the customer's cert may not be issued yet
	// (Let's Encrypt issuance triggers *after* we validate). We're
	// probing for application-level ownership proof, not TLS trust.
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: insecureTLSConfig(),
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		s.markFailed(ctx, d.ID, fmt.Sprintf("probe %s: %v", url, err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		s.markFailed(ctx, d.ID, fmt.Sprintf("probe %s: HTTP %d (want 200)", url, resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		s.markFailed(ctx, d.ID, fmt.Sprintf("read probe body: %v", err))
		return
	}
	if strings.TrimSpace(string(body)) != d.ValidationToken {
		s.markFailed(ctx, d.ID, "probe body did not match validation token")
		return
	}

	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, d.ID, domain.CustomDomainValidated, nil, &now, nil); err != nil {
		slog.Error("mark custom-domain validated", "id", d.ID, "error", err)
	}
}

// RunRenewalsOnce scans for certs expiring within the next 30 days and
// re-runs Provision on each. Lego cert lifetime is 90 days; renewing at
// day 60 leaves a 30-day buffer so a transient ACME outage doesn't run
// us off a cliff. Idempotent — re-issuing a still-valid cert is fine.
// No-op if certRepo wasn't wired (tests / dev).
func (s *CustomDomainService) RunRenewalsOnce(ctx context.Context) {
	if s.certRepo == nil {
		return
	}
	threshold := time.Now().Add(30 * 24 * time.Hour)
	hostnames, err := s.certRepo.ListExpiringHostnames(ctx, threshold)
	if err != nil {
		slog.Error("renewal: list expiring hostnames failed", "error", err)
		return
	}
	if len(hostnames) == 0 {
		return
	}
	slog.Info("renewal: starting batch", "count", len(hostnames), "threshold", threshold)
	for _, hostname := range hostnames {
		if err := s.acme.Provision(ctx, hostname); err != nil {
			// One renewal failure shouldn't kill the batch — next domain
			// may still succeed, and we'll retry the failure on the next
			// tick.
			slog.Error("renewal: provision failed", "hostname", hostname, "error", err)
			continue
		}
		slog.Info("renewal: re-issued", "hostname", hostname)
	}
}

// provisionCert calls the CertProvisioner + flips to active on success.
// Returns true when the row reached active so the caller can refresh
// the hostname cache.
func (s *CustomDomainService) provisionCert(ctx context.Context, d domain.CustomDomain) bool {
	if err := s.acme.Provision(ctx, d.Hostname); err != nil {
		s.markFailed(ctx, d.ID, fmt.Sprintf("acme: %v", err))
		return false
	}
	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, d.ID, domain.CustomDomainActive, nil, nil, &now); err != nil {
		slog.Error("mark custom-domain active", "id", d.ID, "error", err)
		return false
	}
	return true
}

func (s *CustomDomainService) markFailed(ctx context.Context, id int64, reason string) {
	if err := s.repo.UpdateStatus(ctx, id, domain.CustomDomainFailed, &reason, nil, nil); err != nil {
		slog.Error("mark custom-domain failed", "id", id, "error", err)
	}
}

// (randToken is declared in subscription.go and shared across the app
// package — both features need short hex tokens for email links /
// validation probes; no reason to duplicate the primitive.)
