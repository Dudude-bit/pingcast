package httpadapter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/port"
)

// rateLimitMW returns a Fiber middleware that consults limiter using
// keyFn to compute the bucket key. On rejection it emits the canonical
// 429 envelope with a Retry-After hint. On internal errors it lets
// the request through (fail-open) — it's a rate limit, not an auth
// check, and a dead Redis shouldn't close the API.
func rateLimitMW(limiter port.RateLimiter, keyFn func(*fiber.Ctx) string, retryAfterSecs int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := keyFn(c)
		if key == "" {
			return c.Next()
		}
		allowed, err := limiter.Allow(c.UserContext(), key)
		if err != nil {
			return c.Next() // fail-open
		}
		if !allowed {
			return httperr.WriteRateLimited(c, retryAfterSecs)
		}
		return c.Next()
	}
}

// userKey returns the authenticated user's ID, or "" to skip limiting
// when the request is unauthenticated (auth middleware runs first and
// would have 401'd anything that shouldn't be here).
func userKey(c *fiber.Ctx) string {
	user := UserFromCtx(c)
	if user == nil {
		return ""
	}
	return user.ID.String()
}

// ipSlugKey returns a compound key for the public status page limiter:
// same IP × same slug counts as one bucket; different slug = different
// bucket so one scraped slug doesn't starve others.
func ipSlugKey(c *fiber.Ctx) string {
	return c.IP() + "|" + c.Params("slug")
}

// ipKey buckets purely on IP. For endpoints without a per-path
// sub-identity (newsletter subscribe, global public lookups).
func ipKey(c *fiber.Ctx) string {
	return c.IP()
}
