package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

type UserRepo interface {
	Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (user *domain.User, passwordHash string, err error)
	GetBySlug(ctx context.Context, slug string) (*domain.User, error)
	UpdatePlan(ctx context.Context, id uuid.UUID, plan domain.Plan) error
	UpdateLemonSqueezy(ctx context.Context, id uuid.UUID, customerID, subscriptionID string) error
	SetSubscriptionVariant(ctx context.Context, id uuid.UUID, variant string) error
	CountActiveFounderSubscriptions(ctx context.Context) (int64, error)
	GetBranding(ctx context.Context, id uuid.UUID) (Branding, error)
	GetBrandingBySlug(ctx context.Context, slug string) (plan domain.Plan, b Branding, err error)
	UpdateBranding(ctx context.Context, id uuid.UUID, b Branding) error
}

// Branding is the Pro-tier status-page customisation. Free users can
// still read their own values (stored for them in case they upgrade)
// but the status-page renderer ignores them when plan=free.
type Branding struct {
	LogoURL          *string
	AccentColor      *string
	CustomFooterText *string
}

type SessionRepo interface {
	Create(ctx context.Context, sessionID string, userID uuid.UUID, expiresAt time.Time) error
	GetUserID(ctx context.Context, sessionID string) (uuid.UUID, error)
	Touch(ctx context.Context, sessionID string, expiresAt time.Time) error
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type MonitorRepo interface {
	Create(ctx context.Context, m *domain.Monitor) (*domain.Monitor, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Monitor, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Monitor, error)
	ListPublicBySlug(ctx context.Context, slug string) ([]domain.Monitor, error)
	ListActive(ctx context.Context) ([]domain.Monitor, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	Update(ctx context.Context, m *domain.Monitor) error
	// UpdateStatus atomically sets the new status and returns the previous one.
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) (previousStatus domain.MonitorStatus, err error)
	// TogglePause atomically flips is_paused and returns the full updated monitor.
	TogglePause(ctx context.Context, id, userID uuid.UUID) (*domain.Monitor, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	// ListProHTTPForSSLScan returns all Pro-tier HTTP monitors eligible
	// for the daily SSL-expiry probe. Only minimal fields are carried
	// to keep the row narrow for the scan loop.
	ListProHTTPForSSLScan(ctx context.Context) ([]ProHTTPMonitor, error)
}

// ProHTTPMonitor is the narrow row returned by ListProHTTPForSSLScan.
type ProHTTPMonitor struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	CheckConfig json.RawMessage
}

type CheckResultRepo interface {
	Insert(ctx context.Context, cr *domain.CheckResult) error
	ConsecutiveFailures(ctx context.Context, monitorID uuid.UUID) (int, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
	// DeleteByPlan deletes results older than the given cutoffs, where
	// freeCutoff applies to Free-tier users' results and proCutoff to
	// Pro-tier users'. Enables the sprint-2 1-year-retention promise.
	DeleteByPlan(ctx context.Context, freeCutoff, proCutoff time.Time) (int64, error)
	GetResponseTimeChart(ctx context.Context, monitorID uuid.UUID, since time.Time) ([]domain.ChartPoint, error)
}

type UptimeRepo interface {
	RecordCheck(ctx context.Context, monitorID uuid.UUID, checkedAt time.Time, success bool) error
	GetUptime(ctx context.Context, monitorID uuid.UUID, since time.Time) (float64, error)
	GetUptimeBatch(ctx context.Context, monitorIDs []uuid.UUID, since time.Time) (map[uuid.UUID]float64, error)
}

type IncidentRepo interface {
	Create(ctx context.Context, in CreateIncidentInput) (*domain.Incident, error)
	Resolve(ctx context.Context, id int64, resolvedAt time.Time) error
	UpdateState(ctx context.Context, id int64, state domain.IncidentState) error
	GetByID(ctx context.Context, id int64) (*domain.Incident, error)
	GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error)
	IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error)
	ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error)
	ListForExport(ctx context.Context, userID uuid.UUID) ([]IncidentExportRow, error)
}

// IncidentExportRow is a flat row used by the CSV export — the joined
// monitor name is carried here so the handler doesn't need to reach
// back into the monitor repo per incident.
type IncidentExportRow struct {
	ID          int64
	MonitorID   uuid.UUID
	MonitorName string
	StartedAt   time.Time
	ResolvedAt  *time.Time
	Cause       string
	State       domain.IncidentState
	IsManual    bool
	Title       *string
}

// CreateIncidentInput unifies the parameters for incident creation so
// auto-detected (worker-side) and manual (API-side) flows share one
// surface. Callers fill State+IsManual+Title as appropriate.
type CreateIncidentInput struct {
	MonitorID uuid.UUID
	Cause     string
	State     domain.IncidentState
	IsManual  bool
	Title     *string
}

type IncidentUpdateRepo interface {
	Create(ctx context.Context, in CreateIncidentUpdateInput) (*domain.IncidentUpdate, error)
	ListByIncidentID(ctx context.Context, incidentID int64) ([]domain.IncidentUpdate, error)
}

type CreateIncidentUpdateInput struct {
	IncidentID     int64
	State          domain.IncidentState
	Body           string
	PostedByUserID uuid.UUID
}

type FailedAlertRepo interface {
	Create(ctx context.Context, event json.RawMessage, errMsg string, failedChannelIDs []uuid.UUID) error
}

type StatusSubscriberRepo interface {
	Create(ctx context.Context, slug, email, confirmToken, unsubscribeToken string, locale *string) (*domain.StatusSubscriber, error)
	Confirm(ctx context.Context, confirmToken string) (*domain.StatusSubscriber, error)
	Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.StatusSubscriber, error)
	ListConfirmedBySlug(ctx context.Context, slug string) ([]domain.StatusSubscriber, error)
}

type BlogSubscriberRepo interface {
	Create(ctx context.Context, email, confirmToken, unsubscribeToken string, source *string, locale *string) (*domain.BlogSubscriber, error)
	Confirm(ctx context.Context, confirmToken string) (*domain.BlogSubscriber, error)
	Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.BlogSubscriber, error)
	ListConfirmed(ctx context.Context) ([]domain.BlogSubscriber, error)
	CountConfirmed(ctx context.Context) (int64, error)
}

// CustomDomainCert is the at-rest TLS material for one custom domain.
// Stored centrally so an external ingress (Traefik file-provider, Caddy
// dynamic config) can pick it up without re-issuing via ACME on every
// restart.
type CustomDomainCert struct {
	CustomDomainID int64
	CertPEM        string
	KeyPEM         string
	ChainPEM       string
	ExpiresAt      time.Time
}

type CustomDomainCertRepo interface {
	Upsert(ctx context.Context, cert CustomDomainCert) error
	GetByDomainID(ctx context.Context, domainID int64) (*CustomDomainCert, error)
	ListExpiringBefore(ctx context.Context, before time.Time) ([]CustomDomainCert, error)
}

type CustomDomainRepo interface {
	Create(ctx context.Context, userID uuid.UUID, hostname, validationToken string) (*domain.CustomDomain, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.CustomDomain, error)
	GetByHostname(ctx context.Context, hostname string) (*domain.CustomDomain, error)
	ListPending(ctx context.Context) ([]domain.CustomDomain, error)
	UpdateStatus(ctx context.Context, id int64, status domain.CustomDomainStatus, lastError *string, dnsValidatedAt, certIssuedAt *time.Time) error
	Delete(ctx context.Context, id int64, userID uuid.UUID) error
	ListActiveHostnames(ctx context.Context) (map[string]uuid.UUID, error)
}

type MonitorGroupRepo interface {
	Create(ctx context.Context, userID uuid.UUID, name string, ordering int) (*domain.MonitorGroup, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.MonitorGroup, error)
	Update(ctx context.Context, id int64, userID uuid.UUID, name string, ordering int) error
	Delete(ctx context.Context, id int64, userID uuid.UUID) error
	AssignMonitor(ctx context.Context, monitorID, userID uuid.UUID, groupID *int64) error
	// ListMemberships returns monitor_id → group_id for the user's
	// monitors that have a group assigned. nil group_id is omitted.
	ListMemberships(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int64, error)
}

type MaintenanceWindowRepo interface {
	Create(ctx context.Context, in CreateMaintenanceWindowInput) (*domain.MaintenanceWindow, error)
	ListByMonitorID(ctx context.Context, monitorID uuid.UUID) ([]domain.MaintenanceWindow, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.MaintenanceWindow, error)
	Delete(ctx context.Context, id int64, userID uuid.UUID) error
	HasActive(ctx context.Context, monitorID uuid.UUID) (bool, error)
}

type CreateMaintenanceWindowInput struct {
	MonitorID uuid.UUID
	StartsAt  time.Time
	EndsAt    time.Time
	Reason    string
}

// PublicStats is the shape returned by the unauthenticated /api/stats/public
// endpoint. Powers the landing-page trust-bar live counter.
type PublicStats struct {
	MonitorsCount     int64
	IncidentsResolved int64
	PublicStatusPages int64
}

type StatsRepo interface {
	GetPublic(ctx context.Context) (PublicStats, error)
}
