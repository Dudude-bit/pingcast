//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncDelivery_Telegram_ReceivesOnIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")

	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "tg-delivery",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	harness.AssertStatus(t, mr, 201)
	var m struct{ ID string }
	mr.JSON(t, &m)

	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops-tg",
		"type":   "telegram",
		"config": map[string]any{"bot_token": "12345:ABCDEFG", "chat_id": 777},
	})
	harness.AssertStatus(t, cr, 201)
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	br := s.POST(t, "/api/monitors/"+m.ID+"/channels",
		map[string]any{"channel_id": ch.ID})
	if br.Status != 200 && br.Status != 201 {
		t.Fatalf("bind: %d body=%s", br.Status, br.Body)
	}

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 15*time.Second, func() bool {
		return len(h.App.Telegram.Calls()) > 0
	}, "telegram received alert")
}

func TestAsyncDelivery_Webhook_ReceivesOnIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")

	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "wh-delivery",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	harness.AssertStatus(t, mr, 201)
	var m struct{ ID string }
	mr.JSON(t, &m)

	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops-wh",
		"type":   "webhook",
		// Validation rejects localhost in the URL — use a cosmetic
		// external host. SendOverrides routes delivery through
		// FakeWebhookSink.AsSender() which ignores the URL.
		"config": map[string]any{"url": "https://example.com/hook"},
	})
	harness.AssertStatus(t, cr, 201)
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	br := s.POST(t, "/api/monitors/"+m.ID+"/channels",
		map[string]any{"channel_id": ch.ID})
	if br.Status != 200 && br.Status != 201 {
		t.Fatalf("bind: %d body=%s", br.Status, br.Body)
	}

	harness.WaitFor(t, 5*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() > 0
	}, "scheduler observed monitor")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 15*time.Second, func() bool {
		return len(h.App.Webhook.Hits()) > 0
	}, "webhook sink received alert")
}
