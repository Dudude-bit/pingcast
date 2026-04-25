package postgres

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.StatusSubscriberRepo = (*StatusSubscriberRepo)(nil)

type StatusSubscriberRepo struct {
	q *gen.Queries
}

func NewStatusSubscriberRepo(q *gen.Queries) *StatusSubscriberRepo {
	return &StatusSubscriberRepo{q: q}
}

func (r *StatusSubscriberRepo) Create(ctx context.Context, slug, email, confirmToken, unsubscribeToken string, locale *string) (*domain.StatusSubscriber, error) {
	row, err := r.q.CreateStatusSubscriber(ctx, gen.CreateStatusSubscriberParams{
		Slug:             slug,
		Email:            email,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
		Locale:           locale,
	})
	if err != nil {
		return nil, err
	}
	return rowToSub(row), nil
}

func (r *StatusSubscriberRepo) Confirm(ctx context.Context, confirmToken string) (*domain.StatusSubscriber, error) {
	row, err := r.q.ConfirmStatusSubscriber(ctx, confirmToken)
	if err != nil {
		return nil, err
	}
	return rowToSub(row), nil
}

func (r *StatusSubscriberRepo) Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.StatusSubscriber, error) {
	row, err := r.q.UnsubscribeStatusSubscriber(ctx, unsubscribeToken)
	if err != nil {
		return nil, err
	}
	return rowToSub(row), nil
}

func (r *StatusSubscriberRepo) ListConfirmedBySlug(ctx context.Context, slug string) ([]domain.StatusSubscriber, error) {
	rows, err := r.q.ListConfirmedSubscribersBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StatusSubscriber, len(rows))
	for i, row := range rows {
		out[i] = *rowToSub(row)
	}
	return out, nil
}

func rowToSub(row gen.StatusSubscriber) *domain.StatusSubscriber {
	return &domain.StatusSubscriber{
		ID:               row.ID,
		Slug:             row.Slug,
		Email:            row.Email,
		ConfirmToken:     row.ConfirmToken,
		UnsubscribeToken: row.UnsubscribeToken,
		ConfirmedAt:      pgtypeTimestamptzToPtr(row.ConfirmedAt),
		CreatedAt:        row.CreatedAt,
		Locale:           row.Locale,
	}
}
