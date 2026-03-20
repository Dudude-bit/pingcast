package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.UserRepo = (*UserRepo)(nil)

type UserRepo struct {
	q *gen.Queries
}

func NewUserRepo(q *gen.Queries) *UserRepo {
	return &UserRepo{q: q}
}

func (r *UserRepo) Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error) {
	row, err := r.q.CreateUser(ctx, gen.CreateUserParams{
		Email:        email,
		Slug:         slug,
		PasswordHash: passwordHash,
	})
	if err != nil {
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
