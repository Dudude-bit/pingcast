package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// TryLock attempts to acquire a distributed lock using Redis SET NX with TTL.
// Returns true if the lock was acquired, false if another process holds it.
func TryLock(ctx context.Context, client *goredis.Client, key string, ttl time.Duration) (bool, error) {
	result, err := client.SetArgs(ctx, "lock:"+key, "1", goredis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err == goredis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return result == "OK", nil
}

// Unlock releases a distributed lock.
func Unlock(ctx context.Context, client *goredis.Client, key string) error {
	return client.Del(ctx, "lock:"+key).Err()
}
