package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// --- Mocks ---

// mockChannelRepo implements port.ChannelRepo.
type mockChannelRepo struct {
	listForMonitorFn func(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error)
	listByUserIDFn   func(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error)
}

func (m *mockChannelRepo) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error) {
	return m.listForMonitorFn(ctx, monitorID)
}

func (m *mockChannelRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error) {
	if m.listByUserIDFn != nil {
		return m.listByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockChannelRepo) Create(_ context.Context, _ *domain.NotificationChannel) (*domain.NotificationChannel, error) {
	panic("not implemented")
}
func (m *mockChannelRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.NotificationChannel, error) {
	panic("not implemented")
}
func (m *mockChannelRepo) Update(_ context.Context, _ *domain.NotificationChannel) error {
	panic("not implemented")
}
func (m *mockChannelRepo) Delete(_ context.Context, _, _ uuid.UUID) error {
	panic("not implemented")
}
func (m *mockChannelRepo) BindToMonitor(_ context.Context, _, _ uuid.UUID) error {
	panic("not implemented")
}
func (m *mockChannelRepo) UnbindFromMonitor(_ context.Context, _, _ uuid.UUID) error {
	panic("not implemented")
}

// mockChannelRegistry implements port.ChannelRegistry.
type mockChannelRegistry struct {
	createSenderWithRetryFn func(channelType domain.ChannelType, config json.RawMessage) (port.AlertSender, error)
}

func (m *mockChannelRegistry) CreateSenderWithRetry(channelType domain.ChannelType, config json.RawMessage) (port.AlertSender, error) {
	return m.createSenderWithRetryFn(channelType, config)
}

func (m *mockChannelRegistry) Get(_ domain.ChannelType) (port.ChannelSenderFactory, error) {
	panic("not implemented")
}
func (m *mockChannelRegistry) Types() []port.ChannelTypeInfo { panic("not implemented") }
func (m *mockChannelRegistry) ValidateConfig(_ domain.ChannelType, _ json.RawMessage) error {
	panic("not implemented")
}

// mockAlertSender implements port.AlertSender.
type mockAlertSender struct {
	sendFn func(ctx context.Context, event *domain.AlertEvent) error
}

func (m *mockAlertSender) Send(ctx context.Context, event *domain.AlertEvent) error {
	return m.sendFn(ctx, event)
}

// mockFailedAlertRepo implements port.FailedAlertRepo.
type mockFailedAlertRepo struct {
	createFn func(ctx context.Context, event json.RawMessage, errMsg string, failedChannelIDs []uuid.UUID) error
}

func (m *mockFailedAlertRepo) Create(ctx context.Context, event json.RawMessage, errMsg string, failedChannelIDs []uuid.UUID) error {
	if m.createFn != nil {
		return m.createFn(ctx, event, errMsg, failedChannelIDs)
	}
	return nil
}

// mockMetrics implements port.Metrics as no-ops.
type mockMetrics struct{}

func (m *mockMetrics) RecordCheck(context.Context, string, string, time.Duration) {}
func (m *mockMetrics) RecordAlertSent(context.Context, string, bool)                         {}
func (m *mockMetrics) RecordAlertAllFailed(context.Context)                                  {}
func (m *mockMetrics) RecordAlertDeadLettered(context.Context)                               {}
func (m *mockMetrics) MonitorCreated(context.Context)                                        {}
func (m *mockMetrics) MonitorDeleted(context.Context)                                        {}
func (m *mockMetrics) IncidentOpened(context.Context)                                        {}
func (m *mockMetrics) IncidentResolved(context.Context)                                      {}

// --- Helpers ---

func newTestEvent() *domain.AlertEvent {
	return &domain.AlertEvent{
		MonitorID:     uuid.New(),
		UserID:        uuid.New(),
		IncidentID:    1,
		MonitorName:   "test-monitor",
		MonitorTarget: "https://example.com",
		Event:         domain.AlertDown,
		Cause:         "connection timeout",
	}
}

func newChannel(chType domain.ChannelType, enabled bool) domain.NotificationChannel {
	return domain.NotificationChannel{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Name:      "ch-" + string(chType),
		Type:      chType,
		Config:    json.RawMessage(`{}`),
		IsEnabled: enabled,
	}
}

// --- Tests ---

func TestHandle_AllChannelsSucceed(t *testing.T) {
	ch1 := newChannel(domain.ChannelTelegram, true)
	ch2 := newChannel(domain.ChannelEmail, true)

	sendCount := 0
	sender := &mockAlertSender{sendFn: func(_ context.Context, _ *domain.AlertEvent) error {
		sendCount++
		return nil
	}}

	dlqCalled := false
	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return []domain.NotificationChannel{ch1, ch2}, nil
			},
		},
		nil, // monitors not used by Handle
		&mockChannelRegistry{
			createSenderWithRetryFn: func(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
				return sender, nil
			},
		},
		&mockFailedAlertRepo{
			createFn: func(_ context.Context, _ json.RawMessage, _ string, _ []uuid.UUID) error {
				dlqCalled = true
				return nil
			},
		},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sendCount != 2 {
		t.Fatalf("expected 2 sends, got %d", sendCount)
	}
	if dlqCalled {
		t.Fatal("DLQ should not be written when all channels succeed")
	}
}

func TestHandle_AllChannelsFail(t *testing.T) {
	ch1 := newChannel(domain.ChannelTelegram, true)
	ch2 := newChannel(domain.ChannelWebhook, true)

	sender := &mockAlertSender{sendFn: func(_ context.Context, _ *domain.AlertEvent) error {
		return errors.New("send failed")
	}}

	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return []domain.NotificationChannel{ch1, ch2}, nil
			},
		},
		nil,
		&mockChannelRegistry{
			createSenderWithRetryFn: func(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
				return sender, nil
			},
		},
		&mockFailedAlertRepo{},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err == nil {
		t.Fatal("expected error when all channels fail, got nil")
	}
}

func TestHandle_PartialFailure(t *testing.T) {
	ch1 := newChannel(domain.ChannelTelegram, true)
	ch2 := newChannel(domain.ChannelEmail, true)
	ch3 := newChannel(domain.ChannelWebhook, true)

	failID := ch2.ID // second channel will fail

	callIndex := 0
	senders := []*mockAlertSender{
		{sendFn: func(_ context.Context, _ *domain.AlertEvent) error { return nil }},
		{sendFn: func(_ context.Context, _ *domain.AlertEvent) error { return errors.New("email down") }},
		{sendFn: func(_ context.Context, _ *domain.AlertEvent) error { return nil }},
	}

	var dlqFailedIDs []uuid.UUID
	dlqCalled := false

	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return []domain.NotificationChannel{ch1, ch2, ch3}, nil
			},
		},
		nil,
		&mockChannelRegistry{
			createSenderWithRetryFn: func(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
				s := senders[callIndex]
				callIndex++
				return s, nil
			},
		},
		&mockFailedAlertRepo{
			createFn: func(_ context.Context, _ json.RawMessage, _ string, failedChannelIDs []uuid.UUID) error {
				dlqCalled = true
				dlqFailedIDs = failedChannelIDs
				return nil
			},
		},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err != nil {
		t.Fatalf("partial failure should return nil (Ack), got %v", err)
	}
	if !dlqCalled {
		t.Fatal("expected DLQ write on partial failure")
	}
	if len(dlqFailedIDs) != 1 {
		t.Fatalf("expected 1 failed channel ID in DLQ, got %d", len(dlqFailedIDs))
	}
	if dlqFailedIDs[0] != failID {
		t.Fatalf("expected failed channel ID %s, got %s", failID, dlqFailedIDs[0])
	}
}

func TestHandle_NoChannels(t *testing.T) {
	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return nil, nil
			},
			listByUserIDFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return nil, nil
			},
		},
		nil,
		&mockChannelRegistry{},
		&mockFailedAlertRepo{},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err != nil {
		t.Fatalf("no channels should return nil, got %v", err)
	}
}

func TestHandle_ChannelListError(t *testing.T) {
	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return nil, errors.New("db connection lost")
			},
		},
		nil,
		&mockChannelRegistry{},
		&mockFailedAlertRepo{},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err == nil {
		t.Fatal("expected error when ListForMonitor fails, got nil")
	}
}

func TestHandle_DisabledChannelsSkipped(t *testing.T) {
	chEnabled := newChannel(domain.ChannelTelegram, true)
	chDisabled := newChannel(domain.ChannelEmail, false)

	sendCount := 0
	sender := &mockAlertSender{sendFn: func(_ context.Context, _ *domain.AlertEvent) error {
		sendCount++
		return nil
	}}

	svc := app.NewAlertService(
		&mockChannelRepo{
			listForMonitorFn: func(_ context.Context, _ uuid.UUID) ([]domain.NotificationChannel, error) {
				return []domain.NotificationChannel{chEnabled, chDisabled}, nil
			},
		},
		nil,
		&mockChannelRegistry{
			createSenderWithRetryFn: func(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
				return sender, nil
			},
		},
		&mockFailedAlertRepo{},
		&mockMetrics{},
	)

	err := svc.Handle(context.Background(), newTestEvent())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sendCount != 1 {
		t.Fatalf("expected 1 send (disabled channel skipped), got %d", sendCount)
	}
}
