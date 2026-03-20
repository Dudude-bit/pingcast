package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type TelegramSender struct {
	token  string
	urlFmt string
}

func NewTelegramSender(token string, urlFmt ...string) *TelegramSender {
	u := "https://api.telegram.org/bot%s/sendMessage"
	if len(urlFmt) > 0 {
		u = urlFmt[0]
	}
	return &TelegramSender{token: token, urlFmt: u}
}

func (t *TelegramSender) SendDown(ctx context.Context, chatID int64, monitorName, url, cause string) error {
	text := fmt.Sprintf("🔴 *%s* is DOWN\n\nURL: `%s`\nCause: %s", monitorName, url, cause)
	return t.send(ctx, chatID, text)
}

func (t *TelegramSender) SendUp(ctx context.Context, chatID int64, monitorName, url string) error {
	text := fmt.Sprintf("🟢 *%s* is back UP\n\nURL: `%s`", monitorName, url)
	return t.send(ctx, chatID, text)
}

func (t *TelegramSender) send(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	apiURL := fmt.Sprintf(t.urlFmt, t.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}
