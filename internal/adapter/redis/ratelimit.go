package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/kirillinakin/pingcast/internal/port"
	goredis "github.com/redis/go-redis/v9"
)

// Compile-time interface check.
var _ port.RateLimiter = (*RateLimiter)(nil)

// RateLimiter uses go-redis/redis_rate for distributed sliding window rate limiting.
type RateLimiter struct {
	limiter *redis_rate.Limiter
	limit   redis_rate.Limit
	prefix  string
}

// NewRateLimiter creates a Redis-based rate limiter.
// redis_rate.Limit{Rate, Burst, Period} means "up to Rate requests per Period,
// bursting up to Burst". For our "5 attempts per 15 minutes" usage we want
// Rate=maxAttempts and Period=window — NOT per-second rate.
func NewRateLimiter(client *goredis.Client, prefix string, maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limiter: redis_rate.NewLimiter(client),
		limit:   redis_rate.Limit{Rate: maxAttempts, Burst: maxAttempts, Period: window},
		prefix:  prefix,
	}
}

func (rl *RateLimiter) key(id string) string {
	return fmt.Sprintf("ratelimit:%s:%s", rl.prefix, id)
}

// Allow checks if the key is within the rate limit and records the attempt.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(ctx context.Context, id string) (bool, error) {
	res, err := rl.limiter.Allow(ctx, rl.key(id), rl.limit)
	if err != nil {
		return false, fmt.Errorf("rate limiter: %w", err)
	}
	return res.Allowed > 0, nil
}

// Reset clears all attempts for the key (e.g., on successful login).
func (rl *RateLimiter) Reset(ctx context.Context, id string) error {
	return rl.limiter.Reset(ctx, rl.key(id))
}
