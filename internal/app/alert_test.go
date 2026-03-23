package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return([]domain.NotificationChannel{ch1, ch2}, nil).
		Once()

	sender := mocks.NewMockAlertSender(t)
	sender.EXPECT().
		Send(mock.Anything, event).
		Return(nil).
		Times(2)

	registry := mocks.NewMockChannelRegistry(t)
	registry.EXPECT().
		CreateSenderWithRetry(mock.Anything, mock.Anything).
		Return(sender, nil).
		Times(2)

	failedAlerts := mocks.NewMockFailedAlertRepo(t)

	metrics := mocks.NewMockMetrics(t)
	metrics.EXPECT().
		RecordAlertSent(mock.Anything, mock.Anything, true).
		Return().
		Times(2)

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.NoError(t, err)
}

func TestHandle_AllChannelsFail(t *testing.T) {
	ch1 := newChannel(domain.ChannelTelegram, true)
	ch2 := newChannel(domain.ChannelWebhook, true)
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return([]domain.NotificationChannel{ch1, ch2}, nil).
		Once()

	sender := mocks.NewMockAlertSender(t)
	sender.EXPECT().
		Send(mock.Anything, event).
		Return(errors.New("send failed")).
		Times(2)

	registry := mocks.NewMockChannelRegistry(t)
	registry.EXPECT().
		CreateSenderWithRetry(mock.Anything, mock.Anything).
		Return(sender, nil).
		Times(2)

	failedAlerts := mocks.NewMockFailedAlertRepo(t)

	metrics := mocks.NewMockMetrics(t)
	metrics.EXPECT().
		RecordAlertSent(mock.Anything, mock.Anything, false).
		Return().
		Times(2)
	metrics.EXPECT().
		RecordAlertAllFailed(mock.Anything).
		Return().
		Once()

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 2 channels failed")
}

func TestHandle_PartialFailure(t *testing.T) {
	ch1 := newChannel(domain.ChannelTelegram, true)
	ch2 := newChannel(domain.ChannelEmail, true)
	ch3 := newChannel(domain.ChannelWebhook, true)
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return([]domain.NotificationChannel{ch1, ch2, ch3}, nil).
		Once()

	// Three separate senders: first succeeds, second fails, third succeeds.
	sender1 := mocks.NewMockAlertSender(t)
	sender1.EXPECT().Send(mock.Anything, event).Return(nil).Once()

	sender2 := mocks.NewMockAlertSender(t)
	sender2.EXPECT().Send(mock.Anything, event).Return(errors.New("email down")).Once()

	sender3 := mocks.NewMockAlertSender(t)
	sender3.EXPECT().Send(mock.Anything, event).Return(nil).Once()

	callIndex := 0
	senders := []port.AlertSender{sender1, sender2, sender3}

	registry := mocks.NewMockChannelRegistry(t)
	registry.EXPECT().
		CreateSenderWithRetry(mock.Anything, mock.Anything).
		RunAndReturn(func(_ domain.ChannelType, _ json.RawMessage) (port.AlertSender, error) {
			s := senders[callIndex]
			callIndex++
			return s, nil
		}).
		Times(3)

	failedAlerts := mocks.NewMockFailedAlertRepo(t)
	failedAlerts.EXPECT().
		Create(mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(ids []uuid.UUID) bool {
			return len(ids) == 1 && ids[0] == ch2.ID
		})).
		Return(nil).
		Once()

	metrics := mocks.NewMockMetrics(t)
	metrics.EXPECT().
		RecordAlertSent(mock.Anything, mock.Anything, true).
		Return().
		Times(2)
	metrics.EXPECT().
		RecordAlertSent(mock.Anything, mock.Anything, false).
		Return().
		Once()
	metrics.EXPECT().
		RecordAlertDeadLettered(mock.Anything).
		Return().
		Once()

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.NoError(t, err, "partial failure should return nil (Ack)")
}

func TestHandle_NoChannels(t *testing.T) {
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return(nil, nil).
		Once()
	channelRepo.EXPECT().
		ListByUserID(mock.Anything, event.UserID).
		Return(nil, nil).
		Once()

	registry := mocks.NewMockChannelRegistry(t)
	failedAlerts := mocks.NewMockFailedAlertRepo(t)
	metrics := mocks.NewMockMetrics(t)

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.NoError(t, err)
}

func TestHandle_ChannelListError(t *testing.T) {
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return(nil, errors.New("db connection lost")).
		Once()

	registry := mocks.NewMockChannelRegistry(t)
	failedAlerts := mocks.NewMockFailedAlertRepo(t)
	metrics := mocks.NewMockMetrics(t)

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list channels for monitor")
}

func TestHandle_DisabledChannelsSkipped(t *testing.T) {
	chEnabled := newChannel(domain.ChannelTelegram, true)
	chDisabled := newChannel(domain.ChannelEmail, false)
	event := newTestEvent()

	channelRepo := mocks.NewMockChannelRepo(t)
	channelRepo.EXPECT().
		ListForMonitor(mock.Anything, event.MonitorID).
		Return([]domain.NotificationChannel{chEnabled, chDisabled}, nil).
		Once()

	sender := mocks.NewMockAlertSender(t)
	sender.EXPECT().
		Send(mock.Anything, event).
		Return(nil).
		Once()

	registry := mocks.NewMockChannelRegistry(t)
	registry.EXPECT().
		CreateSenderWithRetry(mock.Anything, mock.Anything).
		Return(sender, nil).
		Once()

	failedAlerts := mocks.NewMockFailedAlertRepo(t)

	metrics := mocks.NewMockMetrics(t)
	metrics.EXPECT().
		RecordAlertSent(mock.Anything, mock.Anything, true).
		Return().
		Once()

	svc := app.NewAlertService(channelRepo, nil, registry, failedAlerts, metrics)

	err := svc.Handle(context.Background(), event)
	require.NoError(t, err)
}
