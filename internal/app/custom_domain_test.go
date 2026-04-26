package app_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// recordingCertProvisioner is a hand-rolled fake — captures every
// hostname Provision is called with and lets the test inject errors
// for specific hostnames so we can exercise partial-failure paths.
type recordingCertProvisioner struct {
	mu       sync.Mutex
	provided []string
	errs     map[string]error
}

func (r *recordingCertProvisioner) Provision(_ context.Context, hostname string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provided = append(r.provided, hostname)
	if e, ok := r.errs[hostname]; ok {
		return e
	}
	return nil
}

func (r *recordingCertProvisioner) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.provided))
	copy(out, r.provided)
	return out
}

// TestRunRenewalsOnce_ExpiringCertsReprovisioned is the safety net for
// the day-91 outage scenario: every Pro customer's custom domain dies
// silently if the renewal loop forgets a hostname. We verify that
// every hostname returned by ListExpiringHostnames gets a Provision
// call.
func TestRunRenewalsOnce_ExpiringCertsReprovisioned(t *testing.T) {
	domainRepo := mocks.NewMockCustomDomainRepo(t)
	certRepo := mocks.NewMockCustomDomainCertRepo(t)

	expiring := []string{"status.acme.com", "status.contoso.com"}
	certRepo.EXPECT().
		ListExpiringHostnames(mock.Anything, mock.Anything).
		Return(expiring, nil).
		Once()

	provisioner := &recordingCertProvisioner{}
	svc := app.NewCustomDomainService(domainRepo, certRepo, provisioner, "https://example.test")

	svc.RunRenewalsOnce(context.Background())

	assert.ElementsMatch(t, expiring, provisioner.calls(),
		"every expiring hostname must be re-provisioned")
}

// TestRunRenewalsOnce_NoExpiringCerts_DoesNothing prevents the loop
// from making spurious ACME calls (Let's Encrypt rate-limits). Empty
// list from the repo must mean zero Provision calls.
func TestRunRenewalsOnce_NoExpiringCerts_DoesNothing(t *testing.T) {
	domainRepo := mocks.NewMockCustomDomainRepo(t)
	certRepo := mocks.NewMockCustomDomainCertRepo(t)

	certRepo.EXPECT().
		ListExpiringHostnames(mock.Anything, mock.Anything).
		Return([]string{}, nil).
		Once()

	provisioner := &recordingCertProvisioner{}
	svc := app.NewCustomDomainService(domainRepo, certRepo, provisioner, "")

	svc.RunRenewalsOnce(context.Background())

	assert.Empty(t, provisioner.calls(),
		"no expiring certs ⇒ no ACME calls")
}

// TestRunRenewalsOnce_OneFailureDoesNotKillBatch — one customer's
// failed renewal shouldn't block every other customer. A flaky CNAME
// or a transient ACME outage on one hostname must not cause us to
// skip the rest.
func TestRunRenewalsOnce_OneFailureDoesNotKillBatch(t *testing.T) {
	domainRepo := mocks.NewMockCustomDomainRepo(t)
	certRepo := mocks.NewMockCustomDomainCertRepo(t)

	hostnames := []string{"good1.example.com", "broken.example.com", "good2.example.com"}
	certRepo.EXPECT().
		ListExpiringHostnames(mock.Anything, mock.Anything).
		Return(hostnames, nil).
		Once()

	provisioner := &recordingCertProvisioner{
		errs: map[string]error{"broken.example.com": errors.New("simulated ACME failure")},
	}
	svc := app.NewCustomDomainService(domainRepo, certRepo, provisioner, "")

	svc.RunRenewalsOnce(context.Background())

	assert.ElementsMatch(t, hostnames, provisioner.calls(),
		"failure on one hostname must not skip the others")
}

// TestRunRenewalsOnce_NilCertRepo_NoOp — dev/test bootstraps wire
// nil certRepo (the renewal loop is meaningless without persisted
// certs). The service must not panic in that path.
func TestRunRenewalsOnce_NilCertRepo_NoOp(t *testing.T) {
	domainRepo := mocks.NewMockCustomDomainRepo(t)
	provisioner := &recordingCertProvisioner{}

	svc := app.NewCustomDomainService(domainRepo, nil, provisioner, "")

	require.NotPanics(t, func() {
		svc.RunRenewalsOnce(context.Background())
	})
	assert.Empty(t, provisioner.calls())
}

// TestRunValidationOnce_DNSProbeFailureMarksFailedWithReason — when
// the customer's hostname is unreachable (DNS / TLS / wrong response),
// the row must flip to Failed with a reason describing what broke.
// Without the reason, the dashboard shows a stuck "pending" row and
// the user has no diagnostic to act on.
func TestRunValidationOnce_DNSProbeFailureMarksFailedWithReason(t *testing.T) {
	// .invalid is RFC 2606-reserved — guaranteed never to resolve.
	pending := domain.CustomDomain{
		ID:              42,
		UserID:          uuid.New(),
		Hostname:        "this-host-must-not-exist.invalid",
		ValidationToken: "tok-abc-123",
		Status:          domain.CustomDomainPending,
	}

	domainRepo := mocks.NewMockCustomDomainRepo(t)
	domainRepo.EXPECT().
		ListPending(mock.Anything).
		Return([]domain.CustomDomain{pending}, nil).
		Once()

	// Capture the failure-reason argument so we can assert it's
	// human-readable and points at the actual problem.
	var capturedReason *string
	domainRepo.EXPECT().
		UpdateStatus(
			mock.Anything,
			pending.ID,
			domain.CustomDomainFailed,
			mock.AnythingOfType("*string"),
			mock.Anything,
			mock.Anything,
		).
		Run(func(_ context.Context, _ int64, _ domain.CustomDomainStatus, reason *string, _ *time.Time, _ *time.Time) {
			capturedReason = reason
		}).
		Return(nil).
		Once()

	provisioner := &recordingCertProvisioner{}
	svc := app.NewCustomDomainService(domainRepo, nil, provisioner, "")

	svc.RunValidationOnce(context.Background())

	require.NotNil(t, capturedReason, "UpdateStatus must receive a non-nil reason on probe failure")
	require.NotEmpty(t, *capturedReason, "reason string must not be empty")
	assert.Contains(t, strings.ToLower(*capturedReason), "probe",
		"reason should mention the probe so users know what failed")
	assert.Empty(t, provisioner.calls(),
		"a failed-DNS row must never reach the cert-provision step")
}
