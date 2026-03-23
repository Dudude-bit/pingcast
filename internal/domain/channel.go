package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ChannelType string

const (
	ChannelTelegram ChannelType = "telegram"
	ChannelEmail    ChannelType = "email"
	ChannelWebhook  ChannelType = "webhook"
)

var validChannelTypes = []ChannelType{ChannelTelegram, ChannelEmail, ChannelWebhook}

func (t ChannelType) Valid() bool {
	return ValidEnum(t, validChannelTypes)
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

// ParseConfig unmarshals Config into a map. Returns error instead of silently returning nil.
func (ch *NotificationChannel) ParseConfig() (map[string]any, error) {
	if len(ch.Config) == 0 {
		return nil, nil
	}
	var config map[string]any
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return nil, fmt.Errorf("parse config for channel %s: %w", ch.ID, err)
	}
	return config, nil
}
