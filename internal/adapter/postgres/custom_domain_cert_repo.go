package postgres

import (
	"context"
	"time"

	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.CustomDomainCertRepo = (*CustomDomainCertRepo)(nil)

type CustomDomainCertRepo struct {
	q *gen.Queries
}

func NewCustomDomainCertRepo(q *gen.Queries) *CustomDomainCertRepo {
	return &CustomDomainCertRepo{q: q}
}

func (r *CustomDomainCertRepo) Upsert(ctx context.Context, c port.CustomDomainCert) error {
	return r.q.UpsertCustomDomainCert(ctx, gen.UpsertCustomDomainCertParams{
		CustomDomainID: c.CustomDomainID,
		CertPem:        c.CertPEM,
		KeyPem:         c.KeyPEM,
		ChainPem:       c.ChainPEM,
		ExpiresAt:      c.ExpiresAt,
	})
}

func (r *CustomDomainCertRepo) GetByDomainID(ctx context.Context, id int64) (*port.CustomDomainCert, error) {
	row, err := r.q.GetCustomDomainCertByDomainID(ctx, id)
	if err != nil {
		return nil, err
	}
	return rowToCert(row.CustomDomainID, row.CertPem, row.KeyPem, row.ChainPem, row.ExpiresAt), nil
}

func (r *CustomDomainCertRepo) ListExpiringBefore(ctx context.Context, before time.Time) ([]port.CustomDomainCert, error) {
	rows, err := r.q.ListCustomDomainCertsExpiringBefore(ctx, before)
	if err != nil {
		return nil, err
	}
	out := make([]port.CustomDomainCert, len(rows))
	for i, row := range rows {
		out[i] = *rowToCert(row.CustomDomainID, row.CertPem, row.KeyPem, row.ChainPem, row.ExpiresAt)
	}
	return out, nil
}

func rowToCert(domainID int64, certPEM, keyPEM, chainPEM string, expiresAt time.Time) *port.CustomDomainCert {
	return &port.CustomDomainCert{
		CustomDomainID: domainID,
		CertPEM:        certPEM,
		KeyPEM:         keyPEM,
		ChainPEM:       chainPEM,
		ExpiresAt:      expiresAt,
	}
}
