package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type SessionRepo struct {
	client *goredis.Client
	prefix string
}

func NewSessionRepo(client *goredis.Client) *SessionRepo {
	return &SessionRepo{client: client, prefix: "session:"}
}

func (r *SessionRepo) key(sessionID string) string {
	return r.prefix + sessionID
}

func (r *SessionRepo) Create(ctx context.Context, sessionID string, userID uuid.UUID, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session already expired")
	}
	return r.client.Set(ctx, r.key(sessionID), userID.String(), ttl).Err()
}

func (r *SessionRepo) GetUserID(ctx context.Context, sessionID string) (uuid.UUID, error) {
	val, err := r.client.Get(ctx, r.key(sessionID)).Result()
	if err != nil {
		return uuid.Nil, fmt.Errorf("session not found: %w", err)
	}
	return uuid.Parse(val)
}

func (r *SessionRepo) Touch(ctx context.Context, sessionID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session already expired")
	}
	return r.client.Expire(ctx, r.key(sessionID), ttl).Err()
}

func (r *SessionRepo) Delete(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, r.key(sessionID)).Err()
}

// DeleteExpired is a no-op for Redis sessions — Redis TTL handles expiry automatically.
func (r *SessionRepo) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}
