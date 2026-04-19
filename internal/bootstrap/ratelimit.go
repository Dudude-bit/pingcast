package bootstrap

import (
	"time"

	goredis "github.com/redis/go-redis/v9"

	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/port"
)

// buildRateLimiters constructs the per-scope RateLimiters carrier
// using production defaults, overlayed with any non-zero fields from
// cfg. A non-zero cfg.WindowOverride replaces every scope's natural
// window — used only by integration tests running burst scenarios in
// seconds.
func buildRateLimiters(rdb *goredis.Client, cfg *port.RateLimitConfig) *port.RateLimiters {
	defaults := port.RateLimitDefaults()
	registerMax := defaults.RegisterPerHour
	loginMax := defaults.LoginPer15Min
	statusMax := defaults.StatusPerMin
	writeMax := defaults.WritePerMin
	readMax := defaults.ReadPerMin

	registerWin := 1 * time.Hour
	loginWin := 15 * time.Minute
	statusWin := 1 * time.Minute
	writeWin := 1 * time.Minute
	readWin := 1 * time.Minute

	if cfg != nil {
		if cfg.RegisterPerHour > 0 {
			registerMax = cfg.RegisterPerHour
		}
		if cfg.LoginPer15Min > 0 {
			loginMax = cfg.LoginPer15Min
		}
		if cfg.StatusPerMin > 0 {
			statusMax = cfg.StatusPerMin
		}
		if cfg.WritePerMin > 0 {
			writeMax = cfg.WritePerMin
		}
		if cfg.ReadPerMin > 0 {
			readMax = cfg.ReadPerMin
		}
		if cfg.WindowOverride > 0 {
			registerWin = cfg.WindowOverride
			loginWin = cfg.WindowOverride
			statusWin = cfg.WindowOverride
			writeWin = cfg.WindowOverride
			readWin = cfg.WindowOverride
		}
	}

	return &port.RateLimiters{
		Register: redisadapter.NewRateLimiter(rdb, "rl:register", registerMax, registerWin),
		Login:    redisadapter.NewRateLimiter(rdb, "rl:login", loginMax, loginWin),
		Status:   redisadapter.NewRateLimiter(rdb, "rl:status", statusMax, statusWin),
		Write:    redisadapter.NewRateLimiter(rdb, "rl:write", writeMax, writeWin),
		Read:     redisadapter.NewRateLimiter(rdb, "rl:read", readMax, readWin),
	}
}
