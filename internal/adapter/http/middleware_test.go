package httpadapter

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireUser_NilReturns401(t *testing.T) {
	app := fiber.New()
	app.Get("/guarded", func(c *fiber.Ctx) error {
		user := requireUser(c)
		if user == nil {
			return nil
		}
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("got status %d, want 401", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "UNAUTHORIZED") {
		t.Fatalf("body does not contain UNAUTHORIZED: %s", body)
	}
}
