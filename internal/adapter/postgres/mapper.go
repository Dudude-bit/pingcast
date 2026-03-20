package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

// ---------------------------------------------------------------------------
// pgtype helpers
// ---------------------------------------------------------------------------

func pgtypeInt8ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func int64ToPgtypeInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func pgtypeTimestamptzToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func timeToPgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgtypeInt4ToPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

func intToPgtypeInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

// ---------------------------------------------------------------------------
// User mappers
// ---------------------------------------------------------------------------

func userFromRow(r gen.User) *domain.User {
	return &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		TgChatID:  pgtypeInt8ToPtr(r.TgChatID),
		CreatedAt: r.CreatedAt,
	}
}

func userFromGetByIDRow(r gen.GetUserByIDRow) *domain.User {
	return &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		TgChatID:  pgtypeInt8ToPtr(r.TgChatID),
		CreatedAt: r.CreatedAt,
	}
}

func userFromGetByEmailRow(r gen.GetUserByEmailRow) (*domain.User, string) {
	u := &domain.User{
		ID:        r.ID,
		Email:     r.Email,
		Slug:      r.Slug,
		Plan:      domain.Plan(r.Plan),
		TgChatID:  pgtypeInt8ToPtr(r.TgChatID),
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
		TgChatID:  pgtypeInt8ToPtr(r.TgChatID),
		CreatedAt: r.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Monitor mappers
// ---------------------------------------------------------------------------

func monitorFromRow(r gen.Monitor) domain.Monitor {
	return domain.Monitor{
		ID:                 r.ID,
		UserID:             r.UserID,
		Name:               r.Name,
		URL:                r.Url,
		Method:             domain.HTTPMethod(r.Method),
		IntervalSeconds:    int(r.IntervalSeconds),
		ExpectedStatus:     int(r.ExpectedStatus),
		Keyword:            r.Keyword,
		AlertAfterFailures: int(r.AlertAfterFailures),
		IsPaused:           r.IsPaused,
		IsPublic:           r.IsPublic,
		CurrentStatus:      domain.MonitorStatus(r.CurrentStatus),
		CreatedAt:          r.CreatedAt,
	}
}

func monitorToCreateParams(m *domain.Monitor) gen.CreateMonitorParams {
	return gen.CreateMonitorParams{
		UserID:             m.UserID,
		Name:               m.Name,
		Url:                m.URL,
		Method:             string(m.Method),
		IntervalSeconds:    int32(m.IntervalSeconds),
		ExpectedStatus:     int32(m.ExpectedStatus),
		Keyword:            m.Keyword,
		AlertAfterFailures: int32(m.AlertAfterFailures),
		IsPaused:           m.IsPaused,
		IsPublic:           m.IsPublic,
	}
}

func monitorToUpdateParams(m *domain.Monitor) gen.UpdateMonitorParams {
	return gen.UpdateMonitorParams{
		ID:                 m.ID,
		UserID:             m.UserID,
		Name:               m.Name,
		Url:                m.URL,
		Method:             string(m.Method),
		IntervalSeconds:    int32(m.IntervalSeconds),
		ExpectedStatus:     int32(m.ExpectedStatus),
		Keyword:            m.Keyword,
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
		MonitorID:      cr.MonitorID,
		Status:         string(cr.Status),
		StatusCode:     intToPgtypeInt4(cr.StatusCode),
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
	}
}
