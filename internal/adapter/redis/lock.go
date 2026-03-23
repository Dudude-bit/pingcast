package redis

import (
	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredis "github.com/redis/go-redis/v9"
)

// NewRedsync creates a redsync instance from a go-redis client.
// Use rs.NewMutex("lock-name") to create distributed locks.
func NewRedsync(client *goredis.Client) *redsync.Redsync {
	pool := redsyncgoredis.NewPool(client)
	return redsync.New(pool)
}
