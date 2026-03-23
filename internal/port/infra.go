package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// RateLimiter checks and records rate limits.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string) error
}

// HostLimiter limits concurrent operations per host.
type HostLimiter interface {
	Acquire(ctx context.Context, host string) (bool, error)
	Release(ctx context.Context, host string) error
}

// DistributedMutex provides distributed locking.
type DistributedMutex interface {
	Lock() error
	Unlock() error
	Extend() (bool, error)
}

// APIKeyRepo provides API key lookup for authentication.
type APIKeyRepo interface {
	GetByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	Touch(ctx context.Context, id uuid.UUID) error
}
