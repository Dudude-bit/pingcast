//go:build integration

package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// When the channel sender fails on every attempt, the alert service's
// "all channels failed" path returns an error; subsequent NATS delivery
// retries exhaust MaxDeliver and land in the failed_alerts DLQ table.
// The C1 harness-level FakeWebhookSink records each attempted delivery
// as a hit, and FailAll makes every delivery return an error.
func TestAsyncDLQ_SenderAlwaysFails_RecordsInFailedAlerts(t *testing.T) {
	h := harness.New(t)

	// Install a permanent failure on the webhook sink before the stack
	// starts so the first delivery attempt already sees the error.
	h.App.Webhook.FailAll(errors.New("boom"))

	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")

	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "dlq",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	harness.AssertStatus(t, mr, 201)
	var m struct{ ID string }
	mr.JSON(t, &m)

	// Two channels: telegram succeeds (via FakeTelegram), webhook fails
	// (via Webhook.FailAll). That's a partial-failure path — AlertService
	// writes a failed_alerts row synchronously instead of relying on
	// JetStream MaxDeliver (which would take minutes).
	tg := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops-tg",
		"type":   "telegram",
		"config": map[string]any{"bot_token": "12345:ABCDEFG", "chat_id": 777},
	})
	harness.AssertStatus(t, tg, 201)
	var tgCh struct{ ID string }
	tg.JSON(t, &tgCh)

	wh := s.POST(t, "/api/channels", map[string]any{
		"name":   "doom-hook",
		"type":   "webhook",
		"config": map[string]any{"url": "https://example.com/hook"},
	})
	harness.AssertStatus(t, wh, 201)
	var whCh struct{ ID string }
	wh.JSON(t, &whCh)

	for _, chID := range []string{tgCh.ID, whCh.ID} {
		br := s.POST(t, "/api/monitors/"+m.ID+"/channels",
			map[string]any{"channel_id": chID})
		if br.Status != 200 && br.Status != 201 {
			t.Fatalf("bind %s: %d body=%s", chID, br.Status, br.Body)
		}
	}

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 15*time.Second, func() bool {
		var count int
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM failed_alerts`).Scan(&count)
		return count > 0
	}, "failed_alerts row written")
}
