package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type AlertService struct {
	channels port.ChannelRepo
	monitors port.MonitorRepo
	registry port.ChannelRegistry
}

func NewAlertService(channels port.ChannelRepo, monitors port.MonitorRepo, registry port.ChannelRegistry) *AlertService {
	return &AlertService{channels: channels, monitors: monitors, registry: registry}
}

func (s *AlertService) Registry() port.ChannelRegistry {
	return s.registry
}

// Handle delivers an alert to all relevant channels (best-effort).
func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error {
	channels, _ := s.channels.ListForMonitor(ctx, event.MonitorID)

	if len(channels) == 0 {
		channels, _ = s.channels.ListByUserID(ctx, event.UserID)
	}

	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		factory, err := s.registry.Get(ch.Type)
		if err != nil {
			slog.Error("unknown channel type", "type", ch.Type, "channel_id", ch.ID)
			continue
		}
		sender, err := factory.CreateSender(ch.Config)
		if err != nil {
			slog.Error("failed to create sender", "channel_id", ch.ID, "error", err)
			continue
		}
		if err := sender.Send(ctx, event); err != nil {
			slog.Error("channel delivery failed", "channel_id", ch.ID, "type", ch.Type, "error", err)
		}
	}
	return nil
}

// --- Channel CRUD ---

type CreateChannelInput struct {
	Name   string
	Type   domain.ChannelType
	Config json.RawMessage
}

func (s *AlertService) CreateChannel(ctx context.Context, userID uuid.UUID, input CreateChannelInput) (*domain.NotificationChannel, error) {
	if err := s.registry.ValidateConfig(input.Type, input.Config); err != nil {
		return nil, fmt.Errorf("invalid channel config: %w", err)
	}
	ch := &domain.NotificationChannel{
		UserID:    userID,
		Name:      input.Name,
		Type:      input.Type,
		Config:    input.Config,
		IsEnabled: true,
	}
	return s.channels.Create(ctx, ch)
}

func (s *AlertService) UpdateChannel(ctx context.Context, userID, channelID uuid.UUID, name string, config json.RawMessage, isEnabled bool) (*domain.NotificationChannel, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil || ch.UserID != userID {
		return nil, fmt.Errorf("channel not found")
	}
	if config != nil {
		if err := s.registry.ValidateConfig(ch.Type, config); err != nil {
			return nil, fmt.Errorf("invalid channel config: %w", err)
		}
		ch.Config = config
	}
	ch.Name = name
	ch.IsEnabled = isEnabled
	if err := s.channels.Update(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *AlertService) DeleteChannel(ctx context.Context, userID, channelID uuid.UUID) error {
	return s.channels.Delete(ctx, channelID, userID)
}

func (s *AlertService) ListChannels(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error) {
	return s.channels.ListByUserID(ctx, userID)
}

func (s *AlertService) BindChannel(ctx context.Context, userID, monitorID, channelID uuid.UUID) error {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil || mon.UserID != userID {
		return fmt.Errorf("monitor not found")
	}
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil || ch.UserID != userID {
		return fmt.Errorf("channel not found")
	}
	return s.channels.BindToMonitor(ctx, monitorID, channelID)
}

func (s *AlertService) UnbindChannel(ctx context.Context, userID, monitorID, channelID uuid.UUID) error {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil || mon.UserID != userID {
		return fmt.Errorf("monitor not found")
	}
	return s.channels.UnbindFromMonitor(ctx, monitorID, channelID)
}
