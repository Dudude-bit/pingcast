package notifier

// AlertSender delivers alert notifications for a specific monitor event.
type AlertSender interface {
	NotifyDown(monitorName, monitorURL, cause string) error
	NotifyUp(monitorName, monitorURL string) error
}

// TelegramAlert adapts TelegramSender to AlertSender for a specific chat.
type TelegramAlert struct {
	sender *TelegramSender
	chatID int64
}

func NewTelegramAlert(sender *TelegramSender, chatID int64) *TelegramAlert {
	return &TelegramAlert{sender: sender, chatID: chatID}
}

func (a *TelegramAlert) NotifyDown(monitorName, monitorURL, cause string) error {
	return a.sender.SendDown(a.chatID, monitorName, monitorURL, cause)
}

func (a *TelegramAlert) NotifyUp(monitorName, monitorURL string) error {
	return a.sender.SendUp(a.chatID, monitorName, monitorURL)
}

// EmailAlert adapts EmailSender to AlertSender for a specific recipient.
type EmailAlert struct {
	sender *EmailSender
	to     string
}

func NewEmailAlert(sender *EmailSender, to string) *EmailAlert {
	return &EmailAlert{sender: sender, to: to}
}

func (a *EmailAlert) NotifyDown(monitorName, monitorURL, cause string) error {
	return a.sender.SendDown(a.to, monitorName, monitorURL, cause)
}

func (a *EmailAlert) NotifyUp(monitorName, monitorURL string) error {
	return a.sender.SendUp(a.to, monitorName, monitorURL)
}
