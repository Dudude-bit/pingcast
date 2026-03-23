package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Lua script for atomic semaphore acquire: INCR, check limit, DECR if over.
var acquireScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

local current = redis.call('INCR', key)
if current == 1 then
    redis.call('PEXPIRE', key, ttl)
end
if current > limit then
    redis.call('DECR', key)
    return 0
end
return 1
`)

// HostLimiter limits concurrent checks per host using a Redis atomic Lua script.
type HostLimiter struct {
	client  *goredis.Client
	maxConc int64
	ttlMs   int64
}

// NewHostLimiter creates a Redis-based host limiter.
func NewHostLimiter(client *goredis.Client, maxConcurrency int, ttl time.Duration) *HostLimiter {
	return &HostLimiter{
		client:  client,
		maxConc: int64(maxConcurrency),
		ttlMs:   ttl.Milliseconds(),
	}
}

func (h *HostLimiter) key(host string) string {
	return fmt.Sprintf("hostlimit:%s", host)
}

// Acquire atomically tries to acquire a slot for the host.
// Returns true if acquired, false if at max concurrency.
func (h *HostLimiter) Acquire(ctx context.Context, host string) (bool, error) {
	result, err := acquireScript.Run(ctx, h.client, []string{h.key(host)}, h.maxConc, h.ttlMs).Int64()
	if err != nil {
		return false, fmt.Errorf("host limiter acquire: %w", err)
	}
	return result == 1, nil
}

// Release decrements the host counter.
func (h *HostLimiter) Release(ctx context.Context, host string) error {
	k := h.key(host)
	val, err := h.client.Decr(ctx, k).Result()
	if err != nil {
		return fmt.Errorf("host limiter release: %w", err)
	}
	if val <= 0 {
		h.client.Del(ctx, k)
	}
	return nil
}
