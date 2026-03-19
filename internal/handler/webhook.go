package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type WebhookHandler struct {
	queries            *gen.Queries
	lemonSqueezySecret string
}

func NewWebhookHandler(queries *gen.Queries, lemonSqueezySecret string) *WebhookHandler {
	return &WebhookHandler{queries: queries, lemonSqueezySecret: lemonSqueezySecret}
}

type lemonSqueezyWebhook struct {
	Meta struct {
		EventName string `json:"event_name"`
	} `json:"meta"`
	Data struct {
		Attributes struct {
			CustomerID     int    `json:"customer_id"`
			Status         string `json:"status"`
			UserEmail      string `json:"user_email"`
		} `json:"attributes"`
		ID string `json:"id"`
	} `json:"data"`
}

func (h *WebhookHandler) HandleLemonSqueezy(c *fiber.Ctx) error {
	body := c.Body()
	sig := c.Get("X-Signature")

	if !h.verifySignature(body, sig) {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	var webhook lemonSqueezyWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	email := webhook.Data.Attributes.UserEmail
	user, err := h.queries.GetUserByEmail(c.UserContext(), email)
	if err != nil {
		slog.Error("webhook: user not found", "email", email)
		return c.SendStatus(fiber.StatusOK)
	}

	switch webhook.Meta.EventName {
	case "subscription_created", "subscription_updated":
		if webhook.Data.Attributes.Status == "active" {
			h.queries.UpdateUserPlan(c.UserContext(), gen.UpdateUserPlanParams{ID: user.ID, Plan: "pro"})
			customerID := fmt.Sprintf("%d", webhook.Data.Attributes.CustomerID)
			h.queries.UpdateUserLemonSqueezy(c.UserContext(), gen.UpdateUserLemonSqueezyParams{
				ID:                         user.ID,
				LemonSqueezyCustomerID:     &customerID,
				LemonSqueezySubscriptionID: &webhook.Data.ID,
			})
			slog.Info("user upgraded to pro", "user_id", user.ID)
		}
	case "subscription_cancelled":
		h.queries.UpdateUserPlan(c.UserContext(), gen.UpdateUserPlanParams{ID: user.ID, Plan: "free"})
		slog.Info("user downgraded to free", "user_id", user.ID)
	case "subscription_payment_failed":
		slog.Warn("payment failed", "user_id", user.ID)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if h.lemonSqueezySecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(h.lemonSqueezySecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *WebhookHandler) HandleTelegramWebhook(c *fiber.Ctx) error {
	var update struct {
		Message struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			Text string `json:"text"`
		} `json:"message"`
	}

	if err := c.BodyParser(&update); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	text := update.Message.Text
	chatID := update.Message.Chat.ID

	if strings.HasPrefix(text, "/start ") {
		userIDStr := strings.TrimPrefix(text, "/start ")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return c.SendStatus(fiber.StatusOK)
		}

		if err := h.queries.UpdateUserTelegramChatID(c.UserContext(), gen.UpdateUserTelegramChatIDParams{
			ID:       userID,
			TgChatID: pgtype.Int8{Int64: chatID, Valid: true},
		}); err != nil {
			slog.Error("failed to link telegram", "user_id", userID, "error", err)
		} else {
			slog.Info("telegram linked", "user_id", userID, "chat_id", chatID)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
