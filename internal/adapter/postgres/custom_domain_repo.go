package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.CustomDomainRepo = (*CustomDomainRepo)(nil)

type CustomDomainRepo struct {
	q *gen.Queries
}

func NewCustomDomainRepo(q *gen.Queries) *CustomDomainRepo {
	return &CustomDomainRepo{q: q}
}

func (r *CustomDomainRepo) Create(ctx context.Context, userID uuid.UUID, hostname, token string) (*domain.CustomDomain, error) {
	row, err := r.q.CreateCustomDomain(ctx, gen.CreateCustomDomainParams{
		UserID:          userID,
		Hostname:        hostname,
		ValidationToken: token,
	})
	if err != nil {
		return nil, err
	}
	return rowToCustomDomain(row), nil
}

func (r *CustomDomainRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.CustomDomain, error) {
	rows, err := r.q.ListCustomDomainsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CustomDomain, len(rows))
	for i, row := range rows {
		out[i] = *rowToCustomDomain(row)
	}
	return out, nil
}

func (r *CustomDomainRepo) GetByHostname(ctx context.Context, hostname string) (*domain.CustomDomain, error) {
	row, err := r.q.GetCustomDomainByHostname(ctx, hostname)
	if err != nil {
		return nil, err
	}
	return rowToCustomDomain(row), nil
}

func (r *CustomDomainRepo) ListPending(ctx context.Context) ([]domain.CustomDomain, error) {
	rows, err := r.q.ListPendingCustomDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CustomDomain, len(rows))
	for i, row := range rows {
		out[i] = *rowToCustomDomain(row)
	}
	return out, nil
}

func (r *CustomDomainRepo) UpdateStatus(
	ctx context.Context,
	id int64,
	status domain.CustomDomainStatus,
	lastError *string,
	dnsValidatedAt, certIssuedAt *time.Time,
) error {
	return r.q.UpdateCustomDomainStatus(ctx, gen.UpdateCustomDomainStatusParams{
		ID:              id,
		Status:          gen.CustomDomainStatus(status),
		LastError:       lastError,
		DnsValidatedAt:  ptrToPgtypeTimestamptz(dnsValidatedAt),
		CertIssuedAt:    ptrToPgtypeTimestamptz(certIssuedAt),
	})
}

func (r *CustomDomainRepo) Delete(ctx context.Context, id int64, userID uuid.UUID) error {
	return r.q.DeleteCustomDomain(ctx, gen.DeleteCustomDomainParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *CustomDomainRepo) ListActiveHostnames(ctx context.Context) (map[string]uuid.UUID, error) {
	rows, err := r.q.ListActiveCustomDomainHostnames(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]uuid.UUID, len(rows))
	for _, row := range rows {
		out[row.Hostname] = row.UserID
	}
	return out, nil
}

func rowToCustomDomain(row gen.CustomDomain) *domain.CustomDomain {
	return &domain.CustomDomain{
		ID:              row.ID,
		UserID:          row.UserID,
		Hostname:        row.Hostname,
		ValidationToken: row.ValidationToken,
		Status:          domain.CustomDomainStatus(row.Status),
		LastError:       row.LastError,
		DNSValidatedAt:  pgtypeTimestamptzToPtr(row.DnsValidatedAt),
		CertIssuedAt:    pgtypeTimestamptzToPtr(row.CertIssuedAt),
		CreatedAt:       row.CreatedAt,
	}
}
