package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.ChannelSenderFactory = (*Factory)(nil)
var _ port.AlertSender = (*sender)(nil)

type TelegramChannelConfig struct {
	ChatID int64 `json:"chat_id"`
}

type Factory struct {
	token  string
	urlFmt string
	client *http.Client
}

func NewFactory(token string) *Factory {
	return &Factory{
		token:  token,
		urlFmt: "https://api.telegram.org/bot%s/sendMessage",
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func NewFactoryWithURL(token, urlFmt string) *Factory {
	return &Factory{
		token:  token,
		urlFmt: urlFmt,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *Factory) CreateSender(config json.RawMessage) (port.AlertSender, error) {
	var cfg TelegramChannelConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid telegram config: %w", err)
	}
	if cfg.ChatID == 0 {
		return nil, fmt.Errorf("chat_id is required")
	}
	return &sender{
		token:  f.token,
		urlFmt: f.urlFmt,
		client: f.client,
		chatID: cfg.ChatID,
	}, nil
}

func (f *Factory) ValidateConfig(raw json.RawMessage) error {
	var cfg TelegramChannelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid telegram config: %w", err)
	}
	if cfg.ChatID == 0 {
		return fmt.Errorf("chat_id required")
	}
	return nil
}

func (f *Factory) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "chat_id", Label: "Chat ID", Type: "number", Required: true, Placeholder: "Get from @PingCastBot /start"},
	}}
}

type sender struct {
	token  string
	urlFmt string
	client *http.Client
	chatID int64
}

func (s *sender) Send(ctx context.Context, event *domain.AlertEvent) error {
	var text string
	switch event.Event {
	case domain.AlertDown:
		text = fmt.Sprintf("🔴 *%s* is DOWN\n\nTarget: `%s`\nCause: %s", event.MonitorName, event.MonitorTarget, event.Cause)
	case domain.AlertUp:
		text = fmt.Sprintf("🟢 *%s* is back UP\n\nTarget: `%s`", event.MonitorName, event.MonitorTarget)
	default:
		return nil
	}

	payload := map[string]any{
		"chat_id":    s.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	apiURL := fmt.Sprintf(s.urlFmt, s.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}
