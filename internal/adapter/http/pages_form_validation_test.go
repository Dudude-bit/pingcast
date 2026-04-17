package httpadapter

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestParseIntInRange(t *testing.T) {
	cases := []struct {
		name       string
		formValue  string
		defaultVal int
		r          formRange
		want       int
		wantErr    string
	}{
		{"empty returns default", "", 300, formRange{"interval_seconds", 30, 86400}, 300, ""},
		{"in range", "600", 300, formRange{"interval_seconds", 30, 86400}, 600, ""},
		{"below min", "10", 300, formRange{"interval_seconds", 30, 86400}, 0, "between 30 and 86400"},
		{"above max", "99999", 300, formRange{"interval_seconds", 30, 86400}, 0, "between 30 and 86400"},
		{"not a number", "abc", 300, formRange{"interval_seconds", 30, 86400}, 0, "must be a number"},
		{"alert-after too low", "0", 3, formRange{"alert_after_failures", 1, 10}, 0, "between 1 and 10"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := fiber.New()
			app.Post("/p", func(ctx *fiber.Ctx) error {
				got, err := parseIntInRange(ctx, c.r, c.defaultVal)
				if c.wantErr != "" {
					if err == nil {
						t.Fatalf("want error containing %q, got nil", c.wantErr)
					}
					if !strings.Contains(err.Error(), c.wantErr) {
						t.Fatalf("want error containing %q, got %q", c.wantErr, err.Error())
					}
					return ctx.SendStatus(400)
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != c.want {
					t.Fatalf("want %d, got %d", c.want, got)
				}
				return ctx.SendStatus(200)
			})

			body := url.Values{}
			if c.formValue != "" {
				body.Set(c.r.field, c.formValue)
			}
			req := httptest.NewRequest("POST", "/p", strings.NewReader(body.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			_ = resp.Body.Close()
		})
	}
}
