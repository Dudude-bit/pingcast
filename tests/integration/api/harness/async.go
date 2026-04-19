//go:build integration

package harness

import (
	"context"
	"sync"
	"testing"

	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// AsyncStack wires scheduler + worker + notifier in-process against
// the shared test containers. Tests drive fan-out synchronously via
// Scheduler.Leader.DispatchAll.
type AsyncStack struct {
	Scheduler *bootstrap.Scheduler
	Worker    *bootstrap.Worker
	Notifier  *bootstrap.Notifier

	stopOnce sync.Once
	cancel   context.CancelFunc
}

// StartAsyncStack composes scheduler + worker + notifier and starts
// their subscriptions. SendOverrides wire the harness fakes so alert
// delivery can be inspected via a.Telegram/SMTP/Webhook.
func (a *App) StartAsyncStack(t *testing.T) *AsyncStack {
	t.Helper()

	senders := map[domain.ChannelType]port.AlertSender{
		domain.ChannelTelegram: a.Telegram.AsSender(),
		domain.ChannelEmail:    a.SMTP.AsSender(),
		domain.ChannelWebhook:  a.Webhook.AsSender(),
	}

	sched, err := bootstrap.NewScheduler(bootstrap.SchedulerDeps{
		Pool:               a.Pool,
		Redis:              a.Redis,
		JS:                 a.JS,
		Cipher:             a.App.Cipher,
		RetentionDays:      30,
		SkipLeaderElection: true,
	})
	if err != nil {
		t.Fatalf("scheduler: %v", err)
	}

	wrk, err := bootstrap.NewWorker(bootstrap.WorkerDeps{
		Pool:               a.Pool,
		JS:                 a.JS,
		Cipher:             a.App.Cipher,
		DefaultTimeoutSecs: 10,
		Clock:              a.Clock,
	})
	if err != nil {
		t.Fatalf("worker: %v", err)
	}

	not, err := bootstrap.NewNotifier(bootstrap.NotifierDeps{
		Pool:          a.Pool,
		NATS:          a.NATS,
		JS:            a.JS,
		Cipher:        a.App.Cipher,
		SendOverrides: senders,
	})
	if err != nil {
		t.Fatalf("notifier: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stack := &AsyncStack{
		Scheduler: sched,
		Worker:    wrk,
		Notifier:  not,
		cancel:    cancel,
	}
	// Scheduler.Start launches the monitor-change subscriber + the
	// leader loop. With SkipLeaderElection=true the leader loop
	// acquires a no-op mutex and is a harmless background ticker;
	// tests override the ticker by calling DispatchAll directly.
	sched.Start(ctx)

	t.Cleanup(stack.Stop)
	return stack
}

// Stop drains all three services. Safe to call multiple times.
func (s *AsyncStack) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		shutdownCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.Scheduler.Stop(shutdownCtx)
		s.Worker.Stop()
		s.Notifier.Stop()
	})
}
