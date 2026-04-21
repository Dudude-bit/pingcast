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

var _ port.MonitorRepo = (*MonitorRepo)(nil)

type MonitorRepo struct {
	pool   *pgxpool.Pool
	q      *gen.Queries
	cipher port.Cipher
}

func NewMonitorRepo(pool *pgxpool.Pool, q *gen.Queries, cipher port.Cipher) *MonitorRepo {
	return &MonitorRepo{pool: pool, q: q, cipher: cipher}
}

// queries returns sqlc Queries scoped to the active transaction (if any).
func (r *MonitorRepo) queries(ctx context.Context) *gen.Queries {
	return QueriesFromCtx(ctx, r.q, r.pool)
}

func (r *MonitorRepo) encryptConfig(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if len(config) == 0 {
		return config, nil
	}
	encrypted, err := r.cipher.Encrypt(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	return json.Marshal(encrypted)
}

func (r *MonitorRepo) decryptConfig(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
	if len(config) == 0 {
		return config, nil
	}
	var encrypted string
	if err := json.Unmarshal(config, &encrypted); err != nil {
		// Not encrypted — return as-is (plain JSON config)
		return config, nil
	}
	decrypted, err := r.cipher.Decrypt(ctx, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt monitor config: %w", err)
	}
	return json.RawMessage(decrypted), nil
}

func (r *MonitorRepo) Create(ctx context.Context, m *domain.Monitor) (*domain.Monitor, error) {
	encConfig, err := r.encryptConfig(ctx, m.CheckConfig)
	if err != nil {
		return nil, err
	}
	mCopy := *m
	mCopy.CheckConfig = encConfig
	row, err := r.queries(ctx).CreateMonitor(ctx, monitorToCreateParams(&mCopy))
	if err != nil {
		return nil, err
	}
	out := monitorFromCreateRow(row)
	decrypted, err := r.decryptConfig(ctx, out.CheckConfig)
	if err != nil {
		return nil, err
	}
	out.CheckConfig = decrypted
	return &out, nil
}

func (r *MonitorRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Monitor, error) {
	row, err := r.queries(ctx).GetMonitorByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	out := monitorFromGetByIDRow(row)
	decrypted, err := r.decryptConfig(ctx, out.CheckConfig)
	if err != nil {
		return nil, err
	}
	out.CheckConfig = decrypted
	return &out, nil
}

func (r *MonitorRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Monitor, error) {
	rows, err := r.queries(ctx).ListMonitorsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListByUserIDRow(row)
		if out[i].CheckConfig, err = r.decryptConfig(ctx, out[i].CheckConfig); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *MonitorRepo) ListPublicBySlug(ctx context.Context, slug string) ([]domain.Monitor, error) {
	rows, err := r.queries(ctx).ListPublicMonitorsByUserSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListPublicRow(row)
		if out[i].CheckConfig, err = r.decryptConfig(ctx, out[i].CheckConfig); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *MonitorRepo) ListProHTTPForSSLScan(ctx context.Context) ([]port.ProHTTPMonitor, error) {
	rows, err := r.queries(ctx).ListProHTTPMonitors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]port.ProHTTPMonitor, 0, len(rows))
	for _, row := range rows {
		// CheckConfig is cipher-encrypted at rest; decrypt for the
		// URL-extracting scan. Mirror what decryptConfig does in the
		// rest of this file.
		cfg, err := r.decryptConfig(ctx, row.CheckConfig)
		if err != nil {
			return nil, fmt.Errorf("decrypt ssl-scan config for monitor %s: %w", row.ID, err)
		}
		out = append(out, port.ProHTTPMonitor{
			ID:          row.ID,
			UserID:      row.UserID,
			Name:        row.Name,
			CheckConfig: cfg,
		})
	}
	return out, nil
}

func (r *MonitorRepo) ListActive(ctx context.Context) ([]domain.Monitor, error) {
	rows, err := r.queries(ctx).ListActiveMonitors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListActiveRow(row)
		if out[i].CheckConfig, err = r.decryptConfig(ctx, out[i].CheckConfig); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *MonitorRepo) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := r.queries(ctx).CountMonitorsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *MonitorRepo) Update(ctx context.Context, m *domain.Monitor) error {
	encConfig, err := r.encryptConfig(ctx, m.CheckConfig)
	if err != nil {
		return err
	}
	mCopy := *m
	mCopy.CheckConfig = encConfig
	return r.queries(ctx).UpdateMonitor(ctx, monitorToUpdateParams(&mCopy))
}

func (r *MonitorRepo) TogglePause(ctx context.Context, id, userID uuid.UUID) (*domain.Monitor, error) {
	row, err := r.queries(ctx).ToggleMonitorPause(ctx, gen.ToggleMonitorPauseParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	config, err := r.decryptConfig(ctx, row.CheckConfig)
	if err != nil {
		return nil, err
	}
	return &domain.Monitor{
		ID:                 row.ID,
		UserID:             row.UserID,
		Name:               row.Name,
		Type:               domain.MonitorType(row.Type),
		CheckConfig:        config,
		IntervalSeconds:    int(row.IntervalSeconds),
		AlertAfterFailures: int(row.AlertAfterFailures),
		IsPaused:           row.IsPaused,
		IsPublic:           row.IsPublic,
		CurrentStatus:      domain.MonitorStatus(row.CurrentStatus),
		CreatedAt:          row.CreatedAt,
	}, nil
}

func (r *MonitorRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) (domain.MonitorStatus, error) {
	prev, err := r.queries(ctx).UpdateMonitorStatus(ctx, gen.UpdateMonitorStatusParams{
		ID:            id,
		CurrentStatus: string(status),
	})
	if err != nil {
		return "", err
	}
	return domain.MonitorStatus(prev), nil
}

func (r *MonitorRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.queries(ctx).DeleteMonitor(ctx, gen.DeleteMonitorParams{
		ID:     id,
		UserID: userID,
	})
}
