package httpadapter

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// newTestAppWithUser returns a Fiber app that injects the given user
// (or nil) into Locals under userCtxKey before any downstream handler
// runs, so RequirePro can be exercised in isolation from the real auth
// middleware.
func newTestAppWithUser(user *domain.User) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if user != nil {
			c.Locals(userCtxKey, user)
		}
		return c.Next()
	})
	app.Get("/protected", RequirePro(), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestRequirePro_allowsPro(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Plan: domain.PlanPro}
	app := newTestAppWithUser(user)
	resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("got %d (%s), want 200", resp.StatusCode, body)
	}
}

func TestRequirePro_blocksFreeWith402(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Plan: domain.PlanFree}
	app := newTestAppWithUser(user)
	resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusPaymentRequired {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("got %d (%s), want 402", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("PRO_REQUIRED")) {
		t.Fatalf("response body missing PRO_REQUIRED: %s", body)
	}
}

func TestRequirePro_returnsUnauthorizedWithoutUser(t *testing.T) {
	app := newTestAppWithUser(nil)
	resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("got %d, want 401", resp.StatusCode)
	}
}
