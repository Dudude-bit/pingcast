package postgres

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.BlogSubscriberRepo = (*BlogSubscriberRepo)(nil)

type BlogSubscriberRepo struct {
	q *gen.Queries
}

func NewBlogSubscriberRepo(q *gen.Queries) *BlogSubscriberRepo {
	return &BlogSubscriberRepo{q: q}
}

func (r *BlogSubscriberRepo) Create(ctx context.Context, email, confirmToken, unsubscribeToken string, source *string, locale *string) (*domain.BlogSubscriber, error) {
	row, err := r.q.CreateBlogSubscriber(ctx, gen.CreateBlogSubscriberParams{
		Email:            email,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubscribeToken,
		Source:           source,
		Locale:           locale,
	})
	if err != nil {
		return nil, err
	}
	return rowToBlogSub(row), nil
}

func (r *BlogSubscriberRepo) Confirm(ctx context.Context, confirmToken string) (*domain.BlogSubscriber, error) {
	row, err := r.q.ConfirmBlogSubscriber(ctx, confirmToken)
	if err != nil {
		return nil, err
	}
	return rowToBlogSub(row), nil
}

func (r *BlogSubscriberRepo) Unsubscribe(ctx context.Context, unsubscribeToken string) (*domain.BlogSubscriber, error) {
	row, err := r.q.UnsubscribeBlogSubscriber(ctx, unsubscribeToken)
	if err != nil {
		return nil, err
	}
	return rowToBlogSub(row), nil
}

func (r *BlogSubscriberRepo) ListConfirmed(ctx context.Context) ([]domain.BlogSubscriber, error) {
	rows, err := r.q.ListConfirmedBlogSubscribers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BlogSubscriber, len(rows))
	for i, row := range rows {
		out[i] = *rowToBlogSub(row)
	}
	return out, nil
}

func (r *BlogSubscriberRepo) CountConfirmed(ctx context.Context) (int64, error) {
	return r.q.CountConfirmedBlogSubscribers(ctx)
}

func rowToBlogSub(row gen.BlogSubscriber) *domain.BlogSubscriber {
	return &domain.BlogSubscriber{
		ID:               row.ID,
		Email:            row.Email,
		ConfirmToken:     row.ConfirmToken,
		UnsubscribeToken: row.UnsubscribeToken,
		ConfirmedAt:      pgtypeTimestamptzToPtr(row.ConfirmedAt),
		CreatedAt:        row.CreatedAt,
		Source:           row.Source,
		Locale:           row.Locale,
	}
}
