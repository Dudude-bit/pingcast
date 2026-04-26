package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fixedClock is a hand-rolled port.Clock that always returns the same
// instant. Lets us assert exact timestamps Resolve receives.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// newMonitoringServiceWithMocks wires every repo dependency as a mock,
// returning the service plus the mocks the caller needs to set
// expectations on. Hides the 15-arg constructor noise.
func newMonitoringServiceWithMocks(t *testing.T, clock port.Clock) (
	*app.MonitoringService,
	*mocks.MockMonitorRepo,
	*mocks.MockIncidentRepo,
	*mocks.MockIncidentUpdateRepo,
	*mocks.MockMaintenanceWindowRepo,
	*mocks.MockTxManager,
) {
	t.Helper()
	monitors := mocks.NewMockMonitorRepo(t)
	incidents := mocks.NewMockIncidentRepo(t)
	incidentUpdates := mocks.NewMockIncidentUpdateRepo(t)
	maintenance := mocks.NewMockMaintenanceWindowRepo(t)
	txm := mocks.NewMockTxManager(t)

	svc := app.NewMonitoringService(
		monitors,
		mocks.NewMockChannelRepo(t),
		mocks.NewMockCheckResultRepo(t),
		incidents,
		incidentUpdates,
		maintenance,
		mocks.NewMockMonitorGroupRepo(t),
		mocks.NewMockUserRepo(t),
		mocks.NewMockUptimeRepo(t),
		txm,
		mocks.NewMockAlertEventPublisher(t),
		mocks.NewMockMonitorEventPublisher(t),
		mocks.NewMockCheckerRegistry(t),
		mocks.NewMockMetrics(t),
		clock,
	)
	return svc, monitors, incidents, incidentUpdates, maintenance, txm
}

// runFn is the standard TxManager.Do return — execute the function in
// the same context. Without this the inner transaction never runs.
func runFn(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}

// TestChangeIncidentState_Resolved_SetsResolvedAtFromClock locks in the
// promise the public status page makes: when an operator transitions an
// incident to "resolved", the `resolved_at` timestamp on the incident
// reflects the moment they did it. Without this, post-mortem timestamps
// drift and uptime math is wrong.
func TestChangeIncidentState_Resolved_SetsResolvedAtFromClock(t *testing.T) {
	userID := uuid.New()
	monitorID := uuid.New()
	incidentID := int64(101)
	resolvedAt := time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC)

	svc, monitorRepo, incidentRepo, updateRepo, _, txm :=
		newMonitoringServiceWithMocks(t, fixedClock{now: resolvedAt})

	incidentRepo.EXPECT().GetByID(mock.Anything, incidentID).
		Return(&domain.Incident{
			ID:        incidentID,
			MonitorID: monitorID,
			State:     domain.IncidentStateMonitoring,
		}, nil).Once()
	monitorRepo.EXPECT().GetByID(mock.Anything, monitorID).
		Return(&domain.Monitor{ID: monitorID, UserID: userID}, nil).Once()
	txm.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(runFn).Once()

	incidentRepo.EXPECT().
		UpdateState(mock.Anything, incidentID, domain.IncidentStateResolved).
		Return(nil).Once()
	// The critical assertion: Resolve must be called with the clock's
	// time, not zero, not time.Now() captured somewhere else.
	incidentRepo.EXPECT().
		Resolve(mock.Anything, incidentID, resolvedAt).
		Return(nil).Once()
	updateRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(in port.CreateIncidentUpdateInput) bool {
			return in.IncidentID == incidentID &&
				in.State == domain.IncidentStateResolved &&
				in.PostedByUserID == userID
		})).
		Return(&domain.IncidentUpdate{ID: 9, IncidentID: incidentID}, nil).Once()

	out, err := svc.ChangeIncidentState(context.Background(), app.ChangeIncidentStateInput{
		IncidentID: incidentID,
		UserID:     userID,
		NewState:   domain.IncidentStateResolved,
		UpdateBody: "All clear, root cause fixed.",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, incidentID, out.IncidentID)
}

// TestChangeIncidentState_NonTerminal_DoesNotResolve — moving from
// investigating to monitoring (or any non-resolved state) must NOT
// flip the resolved_at timestamp. If it did, the public timeline would
// briefly show the incident as resolved and uptime math would corrupt.
func TestChangeIncidentState_NonTerminal_DoesNotResolve(t *testing.T) {
	userID := uuid.New()
	monitorID := uuid.New()
	incidentID := int64(202)

	svc, monitorRepo, incidentRepo, updateRepo, _, txm :=
		newMonitoringServiceWithMocks(t, fixedClock{now: time.Now()})

	incidentRepo.EXPECT().GetByID(mock.Anything, incidentID).
		Return(&domain.Incident{
			ID:        incidentID,
			MonitorID: monitorID,
			State:     domain.IncidentStateInvestigating,
		}, nil).Once()
	monitorRepo.EXPECT().GetByID(mock.Anything, monitorID).
		Return(&domain.Monitor{ID: monitorID, UserID: userID}, nil).Once()
	txm.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(runFn).Once()

	incidentRepo.EXPECT().
		UpdateState(mock.Anything, incidentID, domain.IncidentStateMonitoring).
		Return(nil).Once()
	// No Resolve expectation — strict mocks would catch a stray call.
	updateRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&domain.IncidentUpdate{ID: 10, IncidentID: incidentID}, nil).Once()

	_, err := svc.ChangeIncidentState(context.Background(), app.ChangeIncidentStateInput{
		IncidentID: incidentID,
		UserID:     userID,
		NewState:   domain.IncidentStateMonitoring,
		UpdateBody: "Investigating root cause.",
	})
	require.NoError(t, err)
}

// TestScheduleMaintenance_EndsAtBeforeStartsAt_Errors — the cheapest
// possible footgun: an operator typing 14:00 → 13:00 instead of
// 13:00 → 14:00. Without validation we'd silently insert a window that
// can never suppress alerts (because ends_at is already in the past)
// and the operator would think their maintenance is scheduled.
func TestScheduleMaintenance_EndsAtBeforeStartsAt_Errors(t *testing.T) {
	svc, _, _, _, _, _ := newMonitoringServiceWithMocks(t, fixedClock{now: time.Now()})

	starts := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	ends := starts.Add(-1 * time.Hour) // 13:00 — before starts

	_, err := svc.ScheduleMaintenance(context.Background(), app.ScheduleMaintenanceInput{
		MonitorID: uuid.New(),
		UserID:    uuid.New(),
		StartsAt:  starts,
		EndsAt:    ends,
		Reason:    "deploy",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ends_at",
		"error should mention which field is wrong so the form can highlight it")
}

// TestScheduleMaintenance_EndsAtEqualStartsAt_Errors — zero-duration
// windows are nonsensical (same as above failure mode but more subtle).
// "ends_at must be after starts_at" → strict greater-than.
func TestScheduleMaintenance_EndsAtEqualStartsAt_Errors(t *testing.T) {
	svc, _, _, _, _, _ := newMonitoringServiceWithMocks(t, fixedClock{now: time.Now()})

	at := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	_, err := svc.ScheduleMaintenance(context.Background(), app.ScheduleMaintenanceInput{
		MonitorID: uuid.New(),
		UserID:    uuid.New(),
		StartsAt:  at,
		EndsAt:    at,
		Reason:    "deploy",
	})
	require.Error(t, err)
}

// TestChangeIncidentState_OtherUserCannotTouch — bare ownership check.
// If we ever stop comparing monitor.UserID against in.UserID, ANY
// authenticated user could resolve ANY other user's incident. Cheap
// regression to lock in.
func TestChangeIncidentState_OtherUserCannotTouch(t *testing.T) {
	owner := uuid.New()
	attacker := uuid.New()
	monitorID := uuid.New()
	incidentID := int64(303)

	svc, monitorRepo, incidentRepo, _, _, _ :=
		newMonitoringServiceWithMocks(t, fixedClock{now: time.Now()})

	incidentRepo.EXPECT().GetByID(mock.Anything, incidentID).
		Return(&domain.Incident{
			ID:        incidentID,
			MonitorID: monitorID,
			State:     domain.IncidentStateInvestigating,
		}, nil).Once()
	monitorRepo.EXPECT().GetByID(mock.Anything, monitorID).
		Return(&domain.Monitor{ID: monitorID, UserID: owner}, nil).Once()

	_, err := svc.ChangeIncidentState(context.Background(), app.ChangeIncidentStateInput{
		IncidentID: incidentID,
		UserID:     attacker,
		NewState:   domain.IncidentStateResolved,
		UpdateBody: "I am pwning your incident.",
	})
	require.ErrorIs(t, err, domain.ErrForbidden)
}
