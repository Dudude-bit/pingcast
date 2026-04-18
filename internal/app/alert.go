package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// deliveryReason extracts the reason from a DeliveryError, or returns "unknown".
func deliveryReason(err error) string {
	var de *domain.DeliveryError
	if errors.As(err, &de) {
		return de.Reason
	}
	return "unknown"
}

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

	var (
		sent      int
		failed    int
		failedIDs []uuid.UUID
		mu        sync.Mutex
	)

	groupCtx, groupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer groupCancel()

	g, gCtx := errgroup.WithContext(groupCtx)
	g.SetLimit(10)

	for _, ch := range channels {
		if !ch.IsEnabled {
			continue
		}
		ch := ch
		g.Go(func() error {
			sender, err := s.registry.CreateSender(ch.Type, ch.ID, ch.Config)
			if err != nil {
				slog.Error("failed to create sender", "channel_id", ch.ID, "type", ch.Type, "error", err)
				if s.metrics != nil {
					s.metrics.RecordAlertSent(gCtx, string(ch.Type), false, "config_error")
				}
				mu.Lock()
				failedIDs = append(failedIDs, ch.ID)
				failed++
				mu.Unlock()
				return nil // don't abort other goroutines
			}

			chCtx, chCancel := context.WithTimeout(gCtx, 10*time.Second)
			defer chCancel()

			if err := sender.Send(chCtx, event); err != nil {
				reason := deliveryReason(err)
				slog.Error("channel delivery failed", "channel_id", ch.ID, "type", ch.Type, "reason", reason, "error", err)
				if s.metrics != nil {
					s.metrics.RecordAlertSent(gCtx, string(ch.Type), false, reason)
				}
				mu.Lock()
				failedIDs = append(failedIDs, ch.ID)
				failed++
				mu.Unlock()
				return nil
			}

			if s.metrics != nil {
				s.metrics.RecordAlertSent(gCtx, string(ch.Type), true, "")
			}
			mu.Lock()
			sent++
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

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

// writeToDLQ persists a partial-failure event to the failed_alerts table
// with retry (3 attempts, 1s/2s/4s backoff). Never returns error — the alert
// is already Acked, and re-sending to successful channels is worse than
// losing the audit record.
func (s *AlertService) writeToDLQ(ctx context.Context, event *domain.AlertEvent, sent, failed int, failedIDs []uuid.UUID) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event for DLQ", "error", err)
		return
	}
	errMsg := fmt.Sprintf("%d/%d channels failed for monitor %s", failed, sent+failed, event.MonitorID)

	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt, delay := range delays {
		if dlqErr := s.failedAlerts.Create(ctx, eventJSON, errMsg, failedIDs); dlqErr != nil {
			slog.Error("DLQ write failed, retrying",
				"attempt", attempt+1,
				"max_attempts", len(delays),
				"error", dlqErr,
			)
			time.Sleep(delay)
			continue
		}
		if s.metrics != nil {
			s.metrics.RecordAlertDeadLettered(ctx)
		}
		return
	}
	slog.Error("DLQ write failed after all retries — partial failure metadata lost",
		"monitor_id", event.MonitorID,
		"sent", sent,
		"failed", failed,
		"failed_channel_ids", failedIDs,
		"event", string(eventJSON),
	)
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

// GetChannelByID returns a single channel owned by userID. Maps a
// foreign-tenant match to ErrForbidden so the API boundary emits 403
// FORBIDDEN_TENANT instead of leaking existence via 404.
func (s *AlertService) GetChannelByID(ctx context.Context, userID, channelID uuid.UUID) (*domain.NotificationChannel, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, domain.ErrNotFound
	}
	if ch.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return ch, nil
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
