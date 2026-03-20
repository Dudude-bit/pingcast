package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kirillinakin/pingcast/internal/port"
)

// compile-time check
var _ port.AlertSender = (*chatAlert)(nil)

// Sender knows how to talk to the Telegram Bot API.
type Sender struct {
	token  string
	urlFmt string
}

// New creates a Sender for the given bot token.
func New(token string) *Sender {
	return &Sender{
		token:  token,
		urlFmt: "https://api.telegram.org/bot%s/sendMessage",
	}
}

// NewWithURL creates a Sender with a custom API URL format (for testing).
func NewWithURL(token, urlFmt string) *Sender {
	return &Sender{token: token, urlFmt: urlFmt}
}

// chatAlert implements port.AlertSender for a specific chat.
type chatAlert struct {
	sender *Sender
	chatID int64
}

// ForChat returns an AlertSender bound to the given chat ID.
func (s *Sender) ForChat(chatID int64) port.AlertSender {
	return &chatAlert{sender: s, chatID: chatID}
}

func (a *chatAlert) NotifyDown(ctx context.Context, name, target, cause string) error {
	text := fmt.Sprintf("🔴 *%s* is DOWN\n\nTarget: `%s`\nCause: %s", name, target, cause)
	return a.sender.send(ctx, a.chatID, text)
}

func (a *chatAlert) NotifyUp(ctx context.Context, name, target string) error {
	text := fmt.Sprintf("🟢 *%s* is back UP\n\nTarget: `%s`", name, target)
	return a.sender.send(ctx, a.chatID, text)
}

func (s *Sender) send(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id":    chatID,
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
