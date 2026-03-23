package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.MonitorRepo = (*MonitorRepo)(nil)

type MonitorRepo struct {
	q   *gen.Queries
	enc *crypto.Encryptor // nil = no encryption
}

func NewMonitorRepo(q *gen.Queries) *MonitorRepo {
	return &MonitorRepo{q: q}
}

func NewMonitorRepoWithEncryption(q *gen.Queries, enc *crypto.Encryptor) *MonitorRepo {
	return &MonitorRepo{q: q, enc: enc}
}

func (r *MonitorRepo) encryptConfig(config json.RawMessage) (json.RawMessage, error) {
	if r.enc == nil || len(config) == 0 {
		return config, nil
	}
	encrypted, err := r.enc.Encrypt(config)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	// Store as JSON string: "encrypted_base64"
	return json.Marshal(encrypted)
}

func (r *MonitorRepo) decryptConfig(config json.RawMessage) (json.RawMessage, error) {
	if r.enc == nil || len(config) == 0 {
		return config, nil
	}
	// Try to unmarshal as a JSON string (encrypted value)
	var encrypted string
	if err := json.Unmarshal(config, &encrypted); err != nil {
		// Not a string — likely unencrypted JSON object, return as-is
		return config, nil
	}
	decrypted, err := r.enc.Decrypt(encrypted)
	if err != nil {
		// Decryption failed — might be unencrypted data, return as-is
		return config, nil
	}
	return json.RawMessage(decrypted), nil
}

func (r *MonitorRepo) Create(ctx context.Context, m *domain.Monitor) (*domain.Monitor, error) {
	encConfig, err := r.encryptConfig(m.CheckConfig)
	if err != nil {
		return nil, err
	}
	mCopy := *m
	mCopy.CheckConfig = encConfig
	row, err := r.q.CreateMonitor(ctx, monitorToCreateParams(&mCopy))
	if err != nil {
		return nil, err
	}
	out := monitorFromCreateRow(row)
	out.CheckConfig, _ = r.decryptConfig(out.CheckConfig)
	return &out, nil
}

func (r *MonitorRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Monitor, error) {
	row, err := r.q.GetMonitorByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := monitorFromGetByIDRow(row)
	out.CheckConfig, _ = r.decryptConfig(out.CheckConfig)
	return &out, nil
}

func (r *MonitorRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Monitor, error) {
	rows, err := r.q.ListMonitorsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListByUserIDRow(row)
		out[i].CheckConfig, _ = r.decryptConfig(out[i].CheckConfig)
	}
	return out, nil
}

func (r *MonitorRepo) ListPublicBySlug(ctx context.Context, slug string) ([]domain.Monitor, error) {
	rows, err := r.q.ListPublicMonitorsByUserSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListPublicRow(row)
		out[i].CheckConfig, _ = r.decryptConfig(out[i].CheckConfig)
	}
	return out, nil
}

func (r *MonitorRepo) ListActive(ctx context.Context) ([]domain.Monitor, error) {
	rows, err := r.q.ListActiveMonitors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Monitor, len(rows))
	for i, row := range rows {
		out[i] = monitorFromListActiveRow(row)
		out[i].CheckConfig, _ = r.decryptConfig(out[i].CheckConfig)
	}
	return out, nil
}

func (r *MonitorRepo) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := r.q.CountMonitorsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *MonitorRepo) Update(ctx context.Context, m *domain.Monitor) error {
	encConfig, err := r.encryptConfig(m.CheckConfig)
	if err != nil {
		return err
	}
	mCopy := *m
	mCopy.CheckConfig = encConfig
	return r.q.UpdateMonitor(ctx, monitorToUpdateParams(&mCopy))
}

func (r *MonitorRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) error {
	return r.q.UpdateMonitorStatus(ctx, gen.UpdateMonitorStatusParams{
		ID:            id,
		CurrentStatus: string(status),
	})
}

func (r *MonitorRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.q.DeleteMonitor(ctx, gen.DeleteMonitorParams{
		ID:     id,
		UserID: userID,
	})
}
