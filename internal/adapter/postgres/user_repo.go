package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.UserRepo = (*UserRepo)(nil)

type UserRepo struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func NewUserRepo(pool *pgxpool.Pool, q *gen.Queries) *UserRepo {
	return &UserRepo{pool: pool, q: q}
}

// queries returns sqlc Queries scoped to the active transaction (if any),
// so AcquireFounderCapLock + CountActiveFounderSubscriptions + the
// follow-up SetSubscriptionVariant all run in the same tx.
func (r *UserRepo) queries(ctx context.Context) *gen.Queries {
	return QueriesFromCtx(ctx, r.q, r.pool)
}

func (r *UserRepo) Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error) {
	row, err := r.q.CreateUser(ctx, gen.CreateUserParams{
		Email:        email,
		Slug:         slug,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrUserExists
		}
		return nil, err
	}
	return userFromCreateRow(row), nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return userFromGetByIDRow(row), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	u, hash := userFromGetByEmailRow(row)
	return u, hash, nil
}

func (r *UserRepo) GetBySlug(ctx context.Context, slug string) (*domain.User, error) {
	row, err := r.q.GetUserBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return userFromGetBySlugRow(row), nil
}

func (r *UserRepo) UpdatePlan(ctx context.Context, id uuid.UUID, plan domain.Plan) error {
	return r.q.UpdateUserPlan(ctx, gen.UpdateUserPlanParams{
		ID:   id,
		Plan: string(plan),
	})
}

func (r *UserRepo) UpdateLemonSqueezy(ctx context.Context, id uuid.UUID, customerID, subscriptionID string) error {
	return r.q.UpdateUserLemonSqueezy(ctx, gen.UpdateUserLemonSqueezyParams{
		ID:                         id,
		LemonSqueezyCustomerID:     &customerID,
		LemonSqueezySubscriptionID: &subscriptionID,
	})
}

func (r *UserRepo) SetSubscriptionVariant(ctx context.Context, id uuid.UUID, variant string) error {
	v := &variant
	if variant == "" {
		v = nil
	}
	return r.queries(ctx).SetSubscriptionVariant(ctx, gen.SetSubscriptionVariantParams{
		ID:                  id,
		SubscriptionVariant: v,
	})
}

func (r *UserRepo) CountActiveFounderSubscriptions(ctx context.Context) (int64, error) {
	return r.queries(ctx).CountActiveFounderSubscriptions(ctx)
}

func (r *UserRepo) AcquireFounderCapLock(ctx context.Context) error {
	return r.queries(ctx).AcquireFounderCapLock(ctx)
}

func (r *UserRepo) GetBranding(ctx context.Context, id uuid.UUID) (port.Branding, error) {
	row, err := r.q.GetUserBranding(ctx, id)
	if err != nil {
		return port.Branding{}, err
	}
	return port.Branding{
		LogoURL:          row.LogoUrl,
		AccentColor:      row.AccentColor,
		CustomFooterText: row.CustomFooterText,
	}, nil
}

func (r *UserRepo) GetBrandingBySlug(ctx context.Context, slug string) (domain.Plan, port.Branding, error) {
	row, err := r.q.GetUserBrandingBySlug(ctx, slug)
	if err != nil {
		return "", port.Branding{}, err
	}
	return domain.Plan(row.Plan), port.Branding{
		LogoURL:          row.LogoUrl,
		AccentColor:      row.AccentColor,
		CustomFooterText: row.CustomFooterText,
	}, nil
}

func (r *UserRepo) UpdateBranding(ctx context.Context, id uuid.UUID, b port.Branding) error {
	return r.q.UpdateUserBranding(ctx, gen.UpdateUserBrandingParams{
		ID:               id,
		LogoUrl:          b.LogoURL,
		AccentColor:      b.AccentColor,
		CustomFooterText: b.CustomFooterText,
	})
}
