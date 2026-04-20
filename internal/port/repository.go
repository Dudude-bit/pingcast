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
}

type CheckResultRepo interface {
	Insert(ctx context.Context, cr *domain.CheckResult) error
	ConsecutiveFailures(ctx context.Context, monitorID uuid.UUID) (int, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
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

type FailedAlertRepo interface {
	Create(ctx context.Context, event json.RawMessage, errMsg string, failedChannelIDs []uuid.UUID) error
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
