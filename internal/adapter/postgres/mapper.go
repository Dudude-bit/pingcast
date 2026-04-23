package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

// ---------------------------------------------------------------------------
// pgtype helpers
// ---------------------------------------------------------------------------

func pgtypeTimestamptzToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func timeToPgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ptrToPgtypeTimestamptz is the nullable variant — nil in → invalid out.
func ptrToPgtypeTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func intToPgtypeInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	//nolint:gosec // G115: CheckResult.StatusCode is an HTTP status (0..599), always fits int32
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

// ---------------------------------------------------------------------------
// User mappers
// ---------------------------------------------------------------------------

func userFromCreateRow(r gen.CreateUserRow) *domain.User {
	return &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		CreatedAt: r.CreatedAt,
	}
}

func userFromGetByIDRow(r gen.GetUserByIDRow) *domain.User {
	return &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		CreatedAt: r.CreatedAt,
	}
}

func userFromGetByEmailRow(r gen.GetUserByEmailRow) (*domain.User, string) {
	u := &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		CreatedAt: r.CreatedAt,
	}
	return u, r.PasswordHash
}

func userFromGetBySlugRow(r gen.GetUserBySlugRow) *domain.User {
	return &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		CreatedAt: r.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Monitor mappers
// ---------------------------------------------------------------------------

func toDomainMonitor(
	id, userID uuid.UUID,
	name, typ string,
	checkConfig []byte,
	intervalSeconds, alertAfterFailures int32,
	isPaused, isPublic bool,
	currentStatus string,
	createdAt time.Time,
) domain.Monitor {
	return domain.Monitor{
		ID:                 id,
		UserID:             userID,
		Name:               name,
		Type:               domain.MonitorType(typ),
		CheckConfig:        json.RawMessage(checkConfig),
		IntervalSeconds:    int(intervalSeconds),
		AlertAfterFailures: int(alertAfterFailures),
		IsPaused:           isPaused,
		IsPublic:           isPublic,
		CurrentStatus:      domain.MonitorStatus(currentStatus),
		CreatedAt:          createdAt,
	}
}

func monitorFromCreateRow(r gen.CreateMonitorRow) domain.Monitor {
	return toDomainMonitor(r.ID, r.UserID, r.Name, r.Type, r.CheckConfig,
		r.IntervalSeconds, r.AlertAfterFailures, r.IsPaused, r.IsPublic,
		r.CurrentStatus, r.CreatedAt)
}

func monitorFromGetByIDRow(r gen.GetMonitorByIDRow) domain.Monitor {
	return toDomainMonitor(r.ID, r.UserID, r.Name, r.Type, r.CheckConfig,
		r.IntervalSeconds, r.AlertAfterFailures, r.IsPaused, r.IsPublic,
		r.CurrentStatus, r.CreatedAt)
}

func monitorFromListByUserIDRow(r gen.ListMonitorsByUserIDRow) domain.Monitor {
	return toDomainMonitor(r.ID, r.UserID, r.Name, r.Type, r.CheckConfig,
		r.IntervalSeconds, r.AlertAfterFailures, r.IsPaused, r.IsPublic,
		r.CurrentStatus, r.CreatedAt)
}

func monitorFromListPublicRow(r gen.ListPublicMonitorsByUserSlugRow) domain.Monitor {
	return toDomainMonitor(r.ID, r.UserID, r.Name, r.Type, r.CheckConfig,
		r.IntervalSeconds, r.AlertAfterFailures, r.IsPaused, r.IsPublic,
		r.CurrentStatus, r.CreatedAt)
}

func monitorFromListActiveRow(r gen.ListActiveMonitorsRow) domain.Monitor {
	return toDomainMonitor(r.ID, r.UserID, r.Name, r.Type, r.CheckConfig,
		r.IntervalSeconds, r.AlertAfterFailures, r.IsPaused, r.IsPublic,
		r.CurrentStatus, r.CreatedAt)
}

func monitorToCreateParams(m *domain.Monitor) gen.CreateMonitorParams {
	return gen.CreateMonitorParams{
		UserID: m.UserID,
		Name:   m.Name,
		Type:   string(m.Type),
		CheckConfig: []byte(m.CheckConfig),
		//nolint:gosec // G115: IntervalSeconds bounded 30..86400 by domain.ValidateMonitorInput
		IntervalSeconds: int32(m.IntervalSeconds),
		//nolint:gosec // G115: AlertAfterFailures bounded 1..10 by domain.ValidateMonitorInput
		AlertAfterFailures: int32(m.AlertAfterFailures),
		IsPaused:           m.IsPaused,
		IsPublic:           m.IsPublic,
	}
}

func monitorToUpdateParams(m *domain.Monitor) gen.UpdateMonitorParams {
	return gen.UpdateMonitorParams{
		ID:          m.ID,
		UserID:      m.UserID,
		Name:        m.Name,
		CheckConfig: []byte(m.CheckConfig),
		//nolint:gosec // G115: IntervalSeconds bounded 30..86400 by domain.ValidateMonitorInput
		IntervalSeconds: int32(m.IntervalSeconds),
		//nolint:gosec // G115: AlertAfterFailures bounded 1..10 by domain.ValidateMonitorInput
		AlertAfterFailures: int32(m.AlertAfterFailures),
		IsPaused:           m.IsPaused,
		IsPublic:           m.IsPublic,
	}
}

// ---------------------------------------------------------------------------
// CheckResult mappers
// ---------------------------------------------------------------------------

func checkResultToInsertParams(cr *domain.CheckResult) gen.InsertCheckResultParams {
	return gen.InsertCheckResultParams{
		MonitorID:  cr.MonitorID,
		Status:     string(cr.Status),
		StatusCode: intToPgtypeInt4(cr.StatusCode),
		//nolint:gosec // G115: response time in ms from http.Client.Timeout (≤ 30s), always fits int32
		ResponseTimeMs: int32(cr.ResponseTimeMs),
		ErrorMessage:   cr.ErrorMessage,
		CheckedAt:      cr.CheckedAt,
	}
}

// ---------------------------------------------------------------------------
// Incident mappers
// ---------------------------------------------------------------------------

func incidentFromRow(r gen.Incident) domain.Incident {
	return domain.Incident{
		ID:         r.ID,
		MonitorID:  r.MonitorID,
		StartedAt:  r.StartedAt,
		ResolvedAt: pgtypeTimestamptzToPtr(r.ResolvedAt),
		Cause:      r.Cause,
		State:      domain.IncidentState(r.State),
		IsManual:   r.IsManual,
		Title:      r.Title,
	}
}
