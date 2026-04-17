package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.ChannelSenderFactory = (*Factory)(nil)
var _ port.AlertSender = (*sender)(nil)

type WebhookChannelConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type Factory struct {
	client *http.Client
}

func NewFactory() *Factory {
	return &Factory{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *Factory) CreateSender(config json.RawMessage) (port.AlertSender, error) {
	var cfg WebhookChannelConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid webhook config: %w", err)
	}
	return &sender{client: f.client, url: cfg.URL, headers: cfg.Headers}, nil
}

func (f *Factory) ValidateConfig(raw json.RawMessage) error {
	var cfg WebhookChannelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url required")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme")
	}
	if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" {
		return fmt.Errorf("webhook url cannot point to localhost")
	}
	return nil
}

func (f *Factory) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "url", Label: "Webhook URL", Type: "text", Required: true, Placeholder: "https://hooks.slack.com/..."},
		{Name: "headers", Label: "Headers (JSON)", Type: "text", Placeholder: `{"Authorization": "Bearer ..."}`},
	}}
}

type sender struct {
	client  *http.Client
	url     string
	headers map[string]string
}

func (s *sender) Send(ctx context.Context, event *domain.AlertEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return httperr.ClassifyNetError(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return httperr.ClassifyHTTPStatus(resp.StatusCode, fmt.Errorf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}
