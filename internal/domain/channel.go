package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ChannelType string

const (
	ChannelTelegram ChannelType = "telegram"
	ChannelEmail    ChannelType = "email"
	ChannelWebhook  ChannelType = "webhook"
)

func (t ChannelType) Valid() bool {
	switch t {
	case ChannelTelegram, ChannelEmail, ChannelWebhook:
		return true
	}
	return false
}

type NotificationChannel struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Type      ChannelType
	Config    json.RawMessage
	IsEnabled bool
	CreatedAt time.Time
}
