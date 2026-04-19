package port

import "time"

// RateLimiters groups the per-scope rate limiters used across the API.
// Each field is a distinct bucket (different prefix, max, window).
// Wire these in bootstrap.NewApp; middleware reads only the field
// relevant to the route it's guarding.
type RateLimiters struct {
	Register RateLimiter // key: IP            — spec §5: 10/hour
	Login    RateLimiter // key: lowercase email — spec §5: 5/15min
	Status   RateLimiter // key: IP+slug       — spec §5: 60/min
	Write    RateLimiter // key: user ID       — spec §5: 300/min
	Read     RateLimiter // key: user ID       — spec §5: 600/min
}

// RateLimitConfig carries bucket sizes + an optional test-only
// WindowOverride that replaces all windows uniformly. Production
// passes nil; tests pass small windows + small maxes so a
// burst-of-N test completes in seconds.
type RateLimitConfig struct {
	RegisterPerHour int
	LoginPer15Min   int
	StatusPerMin    int
	WritePerMin     int
	ReadPerMin      int

	// WindowOverride, if non-zero, replaces the natural window of
	// every bucket. Use with small Max values in integration tests.
	WindowOverride time.Duration
}

// Defaults returns the spec §5 production numbers.
func RateLimitDefaults() RateLimitConfig {
	return RateLimitConfig{
		RegisterPerHour: 10,
		LoginPer15Min:   5,
		StatusPerMin:    60,
		WritePerMin:     300,
		ReadPerMin:      600,
	}
}
