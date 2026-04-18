//go:build integration

package harness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Session is an HTTP client bound to a cookie jar + optional Bearer.
// All requests are routed to the in-process Fiber app via app.Test().
type Session struct {
	app     *App
	cookies []*http.Cookie
	bearer  string
}

// NewSession returns an unauthenticated session.
func (a *App) NewSession() *Session {
	return &Session{app: a}
}

// WithBearer returns a new session that sends the given Bearer token on
// every request. Cookies are not shared.
func (a *App) WithBearer(token string) *Session {
	return &Session{app: a, bearer: token}
}

// InjectCookie appends a raw cookie (useful for negative-path tests).
func (s *Session) InjectCookie(name, value string) {
	s.cookies = append(s.cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
}

// Response is a small value type that tests inspect directly.
type Response struct {
	Status  int
	Headers http.Header
	Body    []byte
}

func (r *Response) JSON(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, out); err != nil {
		t.Fatalf("unmarshal body=%s: %v", r.Body, err)
	}
}

// Do issues a JSON request with the session's cookies and bearer.
// A nil body sends no Content-Type.
func (s *Session) Do(t *testing.T, method, path string, body any) *Response {
	t.Helper()

	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, rdr)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	s.attach(req)

	resp, err := s.app.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	s.harvestCookies(resp)

	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return &Response{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    raw,
	}
}

// DoRaw sends an exact byte body with a caller-specified content type.
// Useful for malformed-JSON tests.
func (s *Session) DoRaw(t *testing.T, method, path, contentType string, body []byte) *Response {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	s.attach(req)

	resp, err := s.app.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s raw: %v", method, path, err)
	}

	s.harvestCookies(resp)

	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return &Response{Status: resp.StatusCode, Headers: resp.Header, Body: raw}
}

func (s *Session) GET(t *testing.T, path string) *Response {
	return s.Do(t, fiber.MethodGet, path, nil)
}

func (s *Session) POST(t *testing.T, path string, body any) *Response {
	return s.Do(t, fiber.MethodPost, path, body)
}

func (s *Session) PUT(t *testing.T, path string, body any) *Response {
	return s.Do(t, fiber.MethodPut, path, body)
}

func (s *Session) DELETE(t *testing.T, path string) *Response {
	return s.Do(t, fiber.MethodDelete, path, nil)
}

func (s *Session) attach(req *http.Request) {
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	if s.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+s.bearer)
	}
}

func (s *Session) harvestCookies(resp *http.Response) {
	for _, c := range resp.Cookies() {
		s.upsertCookie(c)
	}
}

func (s *Session) upsertCookie(c *http.Cookie) {
	for i, existing := range s.cookies {
		if existing.Name == c.Name {
			s.cookies[i] = c
			return
		}
	}
	s.cookies = append(s.cookies, c)
}
