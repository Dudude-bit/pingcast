package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

var _ port.SessionRepo = (*SessionRepo)(nil)

type SessionRepo struct {
	q *gen.Queries
}

func NewSessionRepo(q *gen.Queries) *SessionRepo {
	return &SessionRepo{q: q}
}

func (r *SessionRepo) Create(ctx context.Context, sessionID string, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.q.CreateSession(ctx, gen.CreateSessionParams{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	})
	return err
}

func (r *SessionRepo) GetUserID(ctx context.Context, sessionID string) (uuid.UUID, error) {
	s, err := r.q.GetSessionByID(ctx, sessionID)
	if err != nil {
		return uuid.Nil, err
	}
	return s.UserID, nil
}

func (r *SessionRepo) Touch(ctx context.Context, sessionID string, expiresAt time.Time) error {
	return r.q.TouchSession(ctx, gen.TouchSessionParams{
		ID:        sessionID,
		ExpiresAt: expiresAt,
	})
}

func (r *SessionRepo) Delete(ctx context.Context, sessionID string) error {
	return r.q.DeleteSession(ctx, sessionID)
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return r.q.DeleteExpiredSessions(ctx)
}
