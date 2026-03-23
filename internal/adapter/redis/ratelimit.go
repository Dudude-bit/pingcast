package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client      *goredis.Client
	maxAttempts int
	window      time.Duration
	prefix      string
}

func NewRateLimiter(client *goredis.Client, prefix string, maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client:      client,
		maxAttempts: maxAttempts,
		window:      window,
		prefix:      prefix,
	}
}

func (rl *RateLimiter) key(id string) string {
	return fmt.Sprintf("ratelimit:%s:%s", rl.prefix, id)
}

// Allow checks if the key is within the rate limit AND records the attempt atomically.
// Returns true if the request is allowed.
func (rl *RateLimiter) Allow(ctx context.Context, id string) (bool, error) {
	k := rl.key(id)
	now := time.Now()
	windowStart := now.Add(-rl.window)

	pipe := rl.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, k, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
	countCmd := pipe.ZCard(ctx, k)
	pipe.ZAdd(ctx, k, goredis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
	pipe.Expire(ctx, k, rl.window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("rate limiter pipeline: %w", err)
	}

	count := countCmd.Val()
	if count >= int64(rl.maxAttempts) {
		// Over limit — remove the attempt we just added
		rl.client.ZRemRangeByRank(ctx, k, -1, -1)
		return false, nil
	}

	return true, nil
}

// Reset clears all attempts for the key (e.g., on successful login).
func (rl *RateLimiter) Reset(ctx context.Context, id string) error {
	return rl.client.Del(ctx, rl.key(id)).Err()
}
