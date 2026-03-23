package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kirillinakin/pingcast/internal/domain"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type APIKeyRepo struct {
	q *sqlcgen.Queries
}

func NewAPIKeyRepo(q *sqlcgen.Queries) *APIKeyRepo {
	return &APIKeyRepo{q: q}
}

func (r *APIKeyRepo) Create(ctx context.Context, key *domain.APIKey) (*domain.APIKey, error) {
	var expiresAt pgtype.Timestamptz
	if key.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: *key.ExpiresAt, Valid: true}
	}

	row, err := r.q.CreateAPIKey(ctx, sqlcgen.CreateAPIKeyParams{
		UserID:    key.UserID,
		KeyHash:   key.KeyHash,
		Name:      key.Name,
		Scopes:    key.Scopes,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}
	return apiKeyFromRow(row), nil
}

func (r *APIKeyRepo) GetByHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	row, err := r.q.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return apiKeyFromRow(row), nil
}

func (r *APIKeyRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.APIKey, error) {
	rows, err := r.q.ListAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	keys := make([]domain.APIKey, len(rows))
	for i, row := range rows {
		keys[i] = *apiKeyFromRow(row)
	}
	return keys, nil
}

func (r *APIKeyRepo) Touch(ctx context.Context, id uuid.UUID) error {
	return r.q.TouchAPIKey(ctx, id)
}

func (r *APIKeyRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.q.DeleteAPIKey(ctx, sqlcgen.DeleteAPIKeyParams{ID: id, UserID: userID})
}

func apiKeyFromRow(row sqlcgen.ApiKey) *domain.APIKey {
	key := &domain.APIKey{
		ID:        row.ID,
		UserID:    row.UserID,
		KeyHash:   row.KeyHash,
		Name:      row.Name,
		Scopes:    row.Scopes,
		CreatedAt: row.CreatedAt,
	}
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		key.LastUsedAt = &t
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		key.ExpiresAt = &t
	}
	return key
}

// timePtr converts a time.Time to a *time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}
