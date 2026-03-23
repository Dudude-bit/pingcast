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
	channels     port.ChannelRepo
	monitors     port.MonitorRepo
	registry     port.ChannelRegistry
	failedAlerts port.FailedAlertRepo
	metrics      port.Metrics
}

func NewAlertService(channels port.ChannelRepo, monitors port.MonitorRepo, registry port.ChannelRegistry, failedAlerts port.FailedAlertRepo, metrics port.Metrics) *AlertService {
	return &AlertService{channels: channels, monitors: monitors, registry: registry, failedAlerts: failedAlerts, metrics: metrics}
}

func (s *AlertService) Registry() port.ChannelRegistry {
	return s.registry
}

// Handle delivers an alert to all relevant channels with per-channel error tracking.
//
// Error semantics:
//   - Channel list query fails → return error → NATS Nak → retry
//   - ALL channels fail → return error → NATS Nak → retry entire event
//   - SOME channels fail → return nil (Ack) → failed channels logged for DLQ (P4.4)
//   - All succeed → return nil (Ack)
func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error {
	channels, err := s.channels.ListForMonitor(ctx, event.MonitorID)
	if err != nil {
		return fmt.Errorf("list channels for monitor %s: %w", event.MonitorID, err)
	}

	if len(channels) == 0 {
		channels, err = s.channels.ListByUserID(ctx, event.UserID)
		if err != nil {
			return fmt.Errorf("list channels for user %s: %w", event.UserID, err)
		}
	}

	if len(channels) == 0 {
		slog.Warn("no channels configured for alert", "monitor_id", event.MonitorID, "user_id", event.UserID)
		return nil
	}

	var sent, failed int
	var failedIDs []uuid.UUID
	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		sender, err := s.registry.CreateSenderWithRetry(ch.Type, ch.Config)
		if err != nil {
			slog.Error("failed to create sender", "channel_id", ch.ID, "type", ch.Type, "error", err)
			if s.metrics != nil {
				s.metrics.RecordAlertSent(ctx, string(ch.Type), false)
			}
			failedIDs = append(failedIDs, ch.ID)
			failed++
			continue
		}
		if err := sender.Send(ctx, event); err != nil {
			slog.Error("channel delivery failed", "channel_id", ch.ID, "type", ch.Type, "error", err)
			if s.metrics != nil {
				s.metrics.RecordAlertSent(ctx, string(ch.Type), false)
			}
			failedIDs = append(failedIDs, ch.ID)
			failed++
			continue
		}
		if s.metrics != nil {
			s.metrics.RecordAlertSent(ctx, string(ch.Type), true)
		}
		sent++
	}

	// All channels failed → return error so NATS retries the entire event
	if sent == 0 && failed > 0 {
		if s.metrics != nil {
			s.metrics.RecordAlertAllFailed(ctx)
		}
		return fmt.Errorf("all %d channels failed for monitor %s", failed, event.MonitorID)
	}

	// Partial failure → Ack (avoid re-sending to successful channels), write to DLQ
	if failed > 0 {
		slog.Error("partial alert delivery failure",
			"monitor_id", event.MonitorID,
			"sent", sent,
			"failed", failed,
		)
		s.writeToDLQ(ctx, event, sent, failed, failedIDs)
	}

	return nil
}

// writeToDLQ persists a partial-failure event to the failed_alerts table.
// Errors are logged but never propagated — the alert is already Acked.
func (s *AlertService) writeToDLQ(ctx context.Context, event *domain.AlertEvent, sent, failed int, failedIDs []uuid.UUID) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for DLQ", "error", err)
		return
	}
	errMsg := fmt.Sprintf("%d/%d channels failed for monitor %s", failed, sent+failed, event.MonitorID)
	if dlqErr := s.failedAlerts.Create(ctx, eventJSON, errMsg, failedIDs); dlqErr != nil {
		slog.Error("failed to write to DLQ", "error", dlqErr)
		return
	}
	if s.metrics != nil {
		s.metrics.RecordAlertDeadLettered(ctx)
	}
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
