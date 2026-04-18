package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.ChannelRepo = (*ChannelRepo)(nil)

type ChannelRepo struct {
	pool   *pgxpool.Pool
	q      *gen.Queries
	cipher port.Cipher
}

func NewChannelRepo(pool *pgxpool.Pool, q *gen.Queries, cipher port.Cipher) *ChannelRepo {
	return &ChannelRepo{pool: pool, q: q, cipher: cipher}
}

func (r *ChannelRepo) queries(ctx context.Context) *gen.Queries {
	return QueriesFromCtx(ctx, r.q, r.pool)
}

func (r *ChannelRepo) encryptConfig(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if len(config) == 0 {
		return config, nil
	}
	encrypted, err := r.cipher.Encrypt(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("encrypt channel config: %w", err)
	}
	return json.Marshal(encrypted)
}

func (r *ChannelRepo) decryptConfig(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if len(config) == 0 {
		return config, nil
	}
	var encrypted string
	if err := json.Unmarshal(config, &encrypted); err != nil {
		return config, nil // Not encrypted — return as-is
	}
	decrypted, err := r.cipher.Decrypt(ctx, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt channel config: %w", err)
	}
	return json.RawMessage(decrypted), nil
}

func (r *ChannelRepo) Create(ctx context.Context, ch *domain.NotificationChannel) (*domain.NotificationChannel, error) {
	encConfig, err := r.encryptConfig(ctx, ch.Config)
	if err != nil {
		return nil, err
	}
	row, err := r.queries(ctx).CreateChannel(ctx, gen.CreateChannelParams{
		UserID: ch.UserID,
		Name:   ch.Name,
		Type:   string(ch.Type),
		Config: encConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	result := toDomainChannel(row)
	decrypted, err := r.decryptConfig(ctx, result.Config)
	if err != nil {
		return nil, err
	}
	result.Config = decrypted
	return result, nil
}

func (r *ChannelRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.NotificationChannel, error) {
	row, err := r.queries(ctx).GetChannelByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}
	result := toDomainChannel(row)
	decrypted, err := r.decryptConfig(ctx, result.Config)
	if err != nil {
		return nil, err
	}
	result.Config = decrypted
	return result, nil
}

func (r *ChannelRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error) {
	rows, err := r.queries(ctx).ListChannelsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	channels := toDomainChannels(rows)
	for i := range channels {
		decrypted, err := r.decryptConfig(ctx, channels[i].Config)
		if err != nil {
			return nil, err
		}
		channels[i].Config = decrypted
	}
	return channels, nil
}

func (r *ChannelRepo) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error) {
	rows, err := r.queries(ctx).ListChannelsForMonitor(ctx, monitorID)
	if err != nil {
		return nil, fmt.Errorf("list channels for monitor: %w", err)
	}
	channels := toDomainChannels(rows)
	for i := range channels {
		decrypted, err := r.decryptConfig(ctx, channels[i].Config)
		if err != nil {
			return nil, err
		}
		channels[i].Config = decrypted
	}
	return channels, nil
}

func (r *ChannelRepo) Update(ctx context.Context, ch *domain.NotificationChannel) error {
	encConfig, err := r.encryptConfig(ctx, ch.Config)
	if err != nil {
		return err
	}
	return r.queries(ctx).UpdateChannel(ctx, gen.UpdateChannelParams{
		ID:        ch.ID,
		Name:      ch.Name,
		Config:    encConfig,
		IsEnabled: ch.IsEnabled,
		UserID:    ch.UserID,
	})
}

func (r *ChannelRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.queries(ctx).DeleteChannel(ctx, gen.DeleteChannelParams{ID: id, UserID: userID})
}

func (r *ChannelRepo) BindToMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error {
	return r.queries(ctx).BindChannelToMonitor(ctx, gen.BindChannelToMonitorParams{
		MonitorID: monitorID, ChannelID: channelID,
	})
}

func (r *ChannelRepo) UnbindFromMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error {
	return r.queries(ctx).UnbindChannelFromMonitor(ctx, gen.UnbindChannelFromMonitorParams{
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
