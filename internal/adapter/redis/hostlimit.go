package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// HostLimiter limits concurrent checks per host using Redis INCR/DECR with TTL.
type HostLimiter struct {
	client   *goredis.Client
	maxConc  int64
	ttl      time.Duration
}

// NewHostLimiter creates a Redis-based host limiter.
// maxConcurrency is the max concurrent checks per host (default 3).
// ttl is the auto-release timeout if a worker crashes (should be >= check timeout).
func NewHostLimiter(client *goredis.Client, maxConcurrency int, ttl time.Duration) *HostLimiter {
	return &HostLimiter{
		client:  client,
		maxConc: int64(maxConcurrency),
		ttl:     ttl,
	}
}

func (h *HostLimiter) key(host string) string {
	return fmt.Sprintf("hostlimit:%s", host)
}

// Acquire tries to acquire a slot for the given host.
// Returns true if acquired, false if host is at max concurrency.
func (h *HostLimiter) Acquire(ctx context.Context, host string) (bool, error) {
	k := h.key(host)
	val, err := h.client.Incr(ctx, k).Result()
	if err != nil {
		return false, fmt.Errorf("incr host limiter: %w", err)
	}
	// Set TTL on first acquisition (or refresh)
	h.client.Expire(ctx, k, h.ttl)

	if val > h.maxConc {
		// Over limit — decrement back
		h.client.Decr(ctx, k)
		return false, nil
	}
	return true, nil
}

// Release releases a slot for the given host.
func (h *HostLimiter) Release(ctx context.Context, host string) error {
	k := h.key(host)
	val, err := h.client.Decr(ctx, k).Result()
	if err != nil {
		return fmt.Errorf("decr host limiter: %w", err)
	}
	// Clean up key if no more slots held
	if val <= 0 {
		h.client.Del(ctx, k)
	}
	return nil
}
