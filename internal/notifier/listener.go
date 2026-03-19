package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type MonitorEvent struct {
	MonitorID string `json:"monitor_id"`
	Event     string `json:"event"` // "down" or "up"
	Details   string `json:"details"`
}

type Listener struct {
	pool     *pgxpool.Pool
	queries  *gen.Queries
	telegram *TelegramSender
	email    *EmailSender
}

func NewListener(pool *pgxpool.Pool, queries *gen.Queries, tg *TelegramSender, email *EmailSender) *Listener {
	return &Listener{pool: pool, queries: queries, telegram: tg, email: email}
}

func (l *Listener) Start(ctx context.Context) {
	go l.listen(ctx)
}

func (l *Listener) listen(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := l.listenOnce(ctx); err != nil {
			slog.Error("listener error, reconnecting", "error", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (l *Listener) listenOnce(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN monitor_events")
	if err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}

		var event MonitorEvent
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			slog.Error("invalid event payload", "payload", notification.Payload, "error", err)
			continue
		}

		l.handleEvent(ctx, &event)
	}
}

func (l *Listener) handleEvent(ctx context.Context, event *MonitorEvent) {
	monitorID, err := uuid.Parse(event.MonitorID)
	if err != nil {
		slog.Error("invalid monitor_id in event", "monitor_id", event.MonitorID)
		return
	}

	monitor, err := l.queries.GetMonitorByID(ctx, monitorID)
	if err != nil {
		slog.Error("monitor not found", "monitor_id", event.MonitorID, "error", err)
		return
	}

	user, err := l.queries.GetUserByID(ctx, monitor.UserID)
	if err != nil {
		slog.Error("user not found", "user_id", monitor.UserID, "error", err)
		return
	}

	// Telegram
	if user.TgChatID.Valid && l.telegram != nil {
		chatID := user.TgChatID.Int64
		switch event.Event {
		case "down":
			if err := l.telegram.SendDown(chatID, monitor.Name, monitor.Url, event.Details); err != nil {
				slog.Error("telegram send failed", "error", err)
			}
		case "up":
			if err := l.telegram.SendUp(chatID, monitor.Name, monitor.Url); err != nil {
				slog.Error("telegram send failed", "error", err)
			}
		}
	}

	// Email (Pro only)
	if user.Plan == "pro" && l.email != nil {
		switch event.Event {
		case "down":
			if err := l.email.SendDown(user.Email, monitor.Name, monitor.Url, event.Details); err != nil {
				slog.Error("email send failed", "error", err)
			}
		case "up":
			if err := l.email.SendUp(user.Email, monitor.Name, monitor.Url); err != nil {
				slog.Error("email send failed", "error", err)
			}
		}
	}

	slog.Info("alert sent", "monitor_id", event.MonitorID, "event", event.Event)
}
