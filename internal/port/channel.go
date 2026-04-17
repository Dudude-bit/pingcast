package port

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

type ChannelSenderFactory interface {
	CreateSender(config json.RawMessage) (AlertSender, error)
	ValidateConfig(config json.RawMessage) error
	ConfigSchema() ConfigSchema
}

type ChannelRegistry interface {
	Get(channelType domain.ChannelType) (ChannelSenderFactory, error)
	CreateSender(channelType domain.ChannelType, channelID uuid.UUID, config json.RawMessage) (AlertSender, error)
	Types() []ChannelTypeInfo
	ValidateConfig(channelType domain.ChannelType, config json.RawMessage) error
}

type ChannelTypeInfo struct {
	Type   domain.ChannelType `json:"type"`
	Label  string             `json:"label"`
	Schema ConfigSchema       `json:"schema"`
}

type ChannelRepo interface {
	Create(ctx context.Context, ch *domain.NotificationChannel) (*domain.NotificationChannel, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.NotificationChannel, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error)
	ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error)
	Update(ctx context.Context, ch *domain.NotificationChannel) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	BindToMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error
	UnbindFromMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error
}
