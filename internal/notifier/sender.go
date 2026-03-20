package notifier

import "context"

// AlertSender delivers alert notifications for a specific monitor event.
type AlertSender interface {
	NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error
	NotifyUp(ctx context.Context, monitorName, monitorURL string) error
}

// TelegramAlert adapts TelegramSender to AlertSender for a specific chat.
type TelegramAlert struct {
	sender *TelegramSender
	chatID int64
}

func NewTelegramAlert(sender *TelegramSender, chatID int64) *TelegramAlert {
	return &TelegramAlert{sender: sender, chatID: chatID}
}

func (a *TelegramAlert) NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error {
	return a.sender.SendDown(ctx, a.chatID, monitorName, monitorURL, cause)
}

func (a *TelegramAlert) NotifyUp(ctx context.Context, monitorName, monitorURL string) error {
	return a.sender.SendUp(ctx, a.chatID, monitorName, monitorURL)
}

// EmailAlert adapts EmailSender to AlertSender for a specific recipient.
type EmailAlert struct {
	sender *EmailSender
	to     string
}

func NewEmailAlert(sender *EmailSender, to string) *EmailAlert {
	return &EmailAlert{sender: sender, to: to}
}

func (a *EmailAlert) NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error {
	return a.sender.SendDown(ctx, a.to, monitorName, monitorURL, cause)
}

func (a *EmailAlert) NotifyUp(ctx context.Context, monitorName, monitorURL string) error {
	return a.sender.SendUp(ctx, a.to, monitorName, monitorURL)
}
