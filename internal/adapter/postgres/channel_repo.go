package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.ChannelRepo = (*ChannelRepo)(nil)

type ChannelRepo struct {
	q *gen.Queries
}

func NewChannelRepo(q *gen.Queries) *ChannelRepo {
	return &ChannelRepo{q: q}
}

func (r *ChannelRepo) Create(ctx context.Context, ch *domain.NotificationChannel) (*domain.NotificationChannel, error) {
	row, err := r.q.CreateChannel(ctx, gen.CreateChannelParams{
		UserID: ch.UserID,
		Name:   ch.Name,
		Type:   string(ch.Type),
		Config: ch.Config,
	})
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return toDomainChannel(row), nil
}

func (r *ChannelRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.NotificationChannel, error) {
	row, err := r.q.GetChannelByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return toDomainChannel(row), nil
}

func (r *ChannelRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error) {
	rows, err := r.q.ListChannelsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return toDomainChannels(rows), nil
}

func (r *ChannelRepo) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error) {
	rows, err := r.q.ListChannelsForMonitor(ctx, monitorID)
	if err != nil {
		return nil, fmt.Errorf("list channels for monitor: %w", err)
	}
	return toDomainChannels(rows), nil
}

func (r *ChannelRepo) Update(ctx context.Context, ch *domain.NotificationChannel) error {
	return r.q.UpdateChannel(ctx, gen.UpdateChannelParams{
		ID:        ch.ID,
		Name:      ch.Name,
		Config:    ch.Config,
		IsEnabled: ch.IsEnabled,
		UserID:    ch.UserID,
	})
}

func (r *ChannelRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.q.DeleteChannel(ctx, gen.DeleteChannelParams{ID: id, UserID: userID})
}

func (r *ChannelRepo) BindToMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error {
	return r.q.BindChannelToMonitor(ctx, gen.BindChannelToMonitorParams{
		MonitorID: monitorID, ChannelID: channelID,
	})
}

func (r *ChannelRepo) UnbindFromMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error {
	return r.q.UnbindChannelFromMonitor(ctx, gen.UnbindChannelFromMonitorParams{
		MonitorID: monitorID, ChannelID: channelID,
	})
}

func toDomainChannel(ch gen.NotificationChannel) *domain.NotificationChannel {
	return &domain.NotificationChannel{
		ID:        ch.ID,
		UserID:    ch.UserID,
		Name:      ch.Name,
		Type:      domain.ChannelType(ch.Type),
		Config:    json.RawMessage(ch.Config),
		IsEnabled: ch.IsEnabled,
		CreatedAt: ch.CreatedAt,
	}
}

func toDomainChannels(rows []gen.NotificationChannel) []domain.NotificationChannel {
	result := make([]domain.NotificationChannel, len(rows))
	for i, row := range rows {
		result[i] = *toDomainChannel(row)
	}
	return result
}
