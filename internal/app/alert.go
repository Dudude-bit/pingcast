package app

import (
	"context"
	"fmt"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type TelegramFactory func(chatID int64) port.AlertSender
type EmailFactory func(to string) port.AlertSender

type AlertService struct {
	telegramFactory TelegramFactory
	emailFactory    EmailFactory
}

func NewAlertService(tg TelegramFactory, email EmailFactory) *AlertService {
	return &AlertService{telegramFactory: tg, emailFactory: email}
}

func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error {
	var senders []port.AlertSender

	if s.telegramFactory != nil && event.TgChatID != nil {
		senders = append(senders, s.telegramFactory(*event.TgChatID))
	}

	if s.emailFactory != nil && event.Plan == domain.PlanPro && event.Email != "" {
		senders = append(senders, s.emailFactory(event.Email))
	}

	for _, sender := range senders {
		var err error
		switch event.Event {
		case domain.AlertDown:
			err = sender.NotifyDown(ctx, event.MonitorName, event.MonitorTarget, event.Cause)
		case domain.AlertUp:
			err = sender.NotifyUp(ctx, event.MonitorName, event.MonitorTarget)
		}
		if err != nil {
			return fmt.Errorf("alert delivery failed: %w", err)
		}
	}

	return nil
}
