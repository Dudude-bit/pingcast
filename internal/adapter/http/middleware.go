package httpadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/xcontext"
)

const userCtxKey = "auth.user"

// touchSem bounds concurrent API-key Touch goroutines. On saturation, the
// current request's Touch is skipped with a Warn log — the trade-off is
// preferred over unbounded goroutine fanout under auth load.
var touchSem = make(chan struct{}, 50)

// AuthMiddleware returns a Fiber handler that validates the session cookie
// or API key and stores *domain.User in Locals. Returns 401 JSON on failure.
func AuthMiddleware(auth *app.AuthService, apiKeyRepo port.APIKeyRepo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try API key first (Authorization: Bearer pc_live_...)
		if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			rawKey := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(rawKey, "pc_live_") {
				return authenticateWithAPIKey(c, auth, apiKeyRepo, rawKey)
			}
		}

		// Fall back to session cookie
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return httperr.WriteUnauthorized(c)
		}

		user, err := auth.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return httperr.WriteUnauthorized(c)
		}

		c.Locals(userCtxKey, user)
		return c.Next()
	}
}

// requiredScope determines the API key scope needed for a given HTTP method and path.
func requiredScope(method, path string) string {
	// Determine resource from path: /api/monitors/... → monitors, /api/channels/... → channels, etc.
	resource := "monitors" // default
	if strings.HasPrefix(path, "/api/channels") {
		resource = "channels"
	} else if strings.HasPrefix(path, "/api/incidents") {
		resource = "incidents"
	}

	switch method {
	case fiber.MethodGet, fiber.MethodHead:
		return resource + ":read"
	default:
		return resource + ":write"
	}
}

func authenticateWithAPIKey(c *fiber.Ctx, auth *app.AuthService, apiKeyRepo port.APIKeyRepo, rawKey string) error {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	apiKey, err := apiKeyRepo.GetByHash(c.UserContext(), keyHash)
	if err != nil {
		return httperr.WriteUnauthorized(c)
	}

	if apiKey.IsExpired() {
		return httperr.WriteUnauthorized(c)
	}

	// Check scope enforcement
	scope := requiredScope(c.Method(), c.Path())
	if !apiKey.HasScope(scope) {
		slog.Warn("api key missing required scope", "key_id", apiKey.ID, "required", scope, "scopes", apiKey.Scopes)
		return httperr.WriteForbiddenTenant(c)
	}

	user, err := auth.GetUserByID(c.UserContext(), apiKey.UserID)
	if err != nil {
		return httperr.WriteUnauthorized(c)
	}

	// Touch last_used_at in background with detached context (Issue 4.5).
	// Request context may be cancelled before goroutine executes. Semaphore
	// bounds concurrent goroutines; saturation → skip with a Warn.
	select {
	case touchSem <- struct{}{}:
		touchCtx, touchCancel := xcontext.Detached(c.UserContext(), 5*time.Second, "api-key.touch")
		go func() {
			defer func() {
				<-touchSem
				touchCancel()
			}()
			if err := apiKeyRepo.Touch(touchCtx, apiKey.ID); err != nil {
				slog.Warn("failed to touch api key", "key_id", apiKey.ID, "error", err)
			}
		}()
	default:
		slog.Warn("api-key touch skipped — semaphore full", "key_id", apiKey.ID)
	}

	c.Locals(userCtxKey, user)
	return c.Next()
}

// PageMiddleware returns a Fiber handler that validates the session cookie
// and stores *domain.User in Locals. Redirects to /login on failure.
func PageMiddleware(auth *app.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Redirect("/login")
		}

		user, err := auth.ValidateSession(c.UserContext(), sessionID)
		if err != nil {
			c.ClearCookie("session_id")
			return c.Redirect("/login")
		}

		c.Locals(userCtxKey, user)
		return c.Next()
	}
}

// UserFromCtx extracts the authenticated *domain.User from fiber.Locals.
func UserFromCtx(c *fiber.Ctx) *domain.User {
	user, _ := c.Locals(userCtxKey).(*domain.User)
	return user
}

// requireUser extracts the authenticated user from context. If absent, it
// writes a 401 JSON response and returns nil. Callers should do:
//
//	user := requireUser(c)
//	if user == nil {
//	    return nil // response already written
//	}
func requireUser(c *fiber.Ctx) *domain.User {
	user := UserFromCtx(c)
	if user == nil {
		_ = httperr.WriteUnauthorized(c)
		return nil
	}
	return user
}
