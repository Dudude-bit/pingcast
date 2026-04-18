//go:build integration

package harness

import (
	"strings"
	"testing"
)

// ErrorBody is the canonical error envelope defined in spec §1.
type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// AssertStatus fails if r.Status != want.
func AssertStatus(t *testing.T, r *Response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status: want=%d got=%d body=%s", want, r.Status, r.Body)
	}
}

// AssertError asserts a canonical-envelope error response. Pass
// wantCode="" to skip the code assertion.
func AssertError(t *testing.T, r *Response, wantStatus int, wantCode string) *ErrorBody {
	t.Helper()
	AssertStatus(t, r, wantStatus)

	var body ErrorBody
	r.JSON(t, &body)

	if body.Error.Code == "" {
		t.Fatalf("envelope missing error.code: %s", r.Body)
	}
	if body.Error.Message == "" {
		t.Fatalf("envelope missing error.message: %s", r.Body)
	}
	if wantCode != "" && body.Error.Code != wantCode {
		t.Fatalf("error.code: want=%q got=%q (body=%s)", wantCode, body.Error.Code, r.Body)
	}
	return &body
}

// AssertEnvelopeMessageContains is a tolerant fallback when the exact
// code is not worth locking down but a human-readable substring is.
func AssertEnvelopeMessageContains(t *testing.T, r *Response, substr string) {
	t.Helper()
	var body ErrorBody
	r.JSON(t, &body)
	if !strings.Contains(body.Error.Message, substr) {
		t.Fatalf("error.message: want substring %q, got %q", substr, body.Error.Message)
	}
}
