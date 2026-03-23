package port

import (
	"context"
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
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type CheckResultRepo interface {
	Insert(ctx context.Context, cr *domain.CheckResult) error
	ConsecutiveFailures(ctx context.Context, monitorID uuid.UUID) (int, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type UptimeRepo interface {
	RecordCheck(ctx context.Context, monitorID uuid.UUID, checkedAt time.Time, success bool) error
	GetUptime(ctx context.Context, monitorID uuid.UUID, since time.Time) (float64, error)
	GetUptimeBatch(ctx context.Context, monitorIDs []uuid.UUID, since time.Time) (map[uuid.UUID]float64, error)
}

type IncidentRepo interface {
	Create(ctx context.Context, monitorID uuid.UUID, cause string) (*domain.Incident, error)
	Resolve(ctx context.Context, id int64, resolvedAt time.Time) error
	GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error)
	IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error)
	ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error)
}
