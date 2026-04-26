package httpadapter

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/app"
)

type WebhookHandler struct {
	auth               *app.AuthService
	alerts             *app.AlertService
	billing            *app.BillingService
	lemonSqueezySecret string
	// founderVariantID is the LemonSqueezy variant ID for the $9/mo
	// founder tier. When an incoming subscription_created webhook
	// references this variant, we tag the user with subscription_variant=
	// 'founder' so CountActiveFounderSubscriptions reflects the new
	// seat. Unset → every subscription records as 'retail'.
	founderVariantID string
	// telegramBotToken is the secret Telegram puts in the webhook URL
	// path. We compare incoming :token against this — without the
	// check, anyone can call /webhook/telegram/<anything> and inject
	// notification channels into other users' accounts. Empty string
	// disables the Telegram webhook entirely (returns 401).
	telegramBotToken string
}

func NewWebhookHandler(
	auth *app.AuthService,
	alerts *app.AlertService,
	billing *app.BillingService,
	lemonSqueezySecret string,
	founderVariantID string,
	telegramBotToken string,
) *WebhookHandler {
	return &WebhookHandler{
		auth:               auth,
		alerts:             alerts,
		billing:            billing,
		lemonSqueezySecret: lemonSqueezySecret,
		founderVariantID:   founderVariantID,
		telegramBotToken:   telegramBotToken,
	}
}

type lemonSqueezyWebhook struct {
	Meta struct {
		EventName string `json:"event_name"`
	} `json:"meta"`
	Data struct {
		Attributes struct {
			CustomerID int    `json:"customer_id"`
			Status     string `json:"status"`
			UserEmail  string `json:"user_email"`
			VariantID  int    `json:"variant_id"`
		} `json:"attributes"`
		ID string `json:"id"`
	} `json:"data"`
}

func (h *WebhookHandler) HandleLemonSqueezy(c *fiber.Ctx) error {
	if h.lemonSqueezySecret == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "WEBHOOK_DISABLED",
				"message": "webhook processing disabled: secret not configured",
			},
		})
	}

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

	user, err := h.auth.GetUserByEmail(c.UserContext(), email)
	if err != nil {
		slog.Error("webhook: user not found", "email", email)
		return c.SendStatus(fiber.StatusOK)
	}

	switch webhook.Meta.EventName {
	case "subscription_created", "subscription_updated":
		if webhook.Data.Attributes.Status == "active" {
			customerID := fmt.Sprintf("%d", webhook.Data.Attributes.CustomerID)
			if err := h.auth.UpgradeToPro(c.UserContext(), user.ID, customerID, webhook.Data.ID); err != nil {
				// Return 5xx so LemonSqueezy retries. Swallowing the
				// error and returning 200 would leave the customer
				// stuck on Free after paying — LS treats 200 as final.
				slog.Error("failed to upgrade user", "user_id", user.ID, "error", err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			slog.Info("user upgraded to pro", "user_id", user.ID)
			// Variant tagging is atomic + cap-aware: TagFromWebhook
			// downgrades 'founder' to 'retail' if the cap is full
			// (uses a tx-scoped advisory lock to serialize concurrent
			// webhooks). Errors are logged but don't fail the webhook
			// — the user's plan is already 'pro'; tag is for accounting.
			requested := "retail"
			if h.founderVariantID != "" &&
				fmt.Sprintf("%d", webhook.Data.Attributes.VariantID) == h.founderVariantID {
				requested = "founder"
			}
			actual, err := h.billing.TagFromWebhook(c.UserContext(), user.ID, requested)
			if err != nil {
				slog.Error("failed to tag subscription variant",
					"user_id", user.ID, "requested", requested, "error", err)
			} else if actual != requested {
				slog.Info("variant downgraded to retail (founder cap reached)",
					"user_id", user.ID)
			}
		}
	case "subscription_cancelled":
		if err := h.auth.DowngradeToFree(c.UserContext(), user.ID); err != nil {
			// Same reasoning — let LS retry rather than silently leave
			// a cancelled subscriber on Pro.
			slog.Error("failed to downgrade user", "user_id", user.ID, "error", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		slog.Info("user downgraded to free", "user_id", user.ID)
	case "subscription_payment_failed":
		slog.Warn("payment failed", "user_id", user.ID)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.lemonSqueezySecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *WebhookHandler) HandleTelegramWebhook(c *fiber.Ctx) error {
	// IDOR guard: only Telegram knows the bot token. Reject any URL
	// whose :token doesn't match. Constant-time compare so we don't
	// leak token bytes via response timing.
	if h.telegramBotToken == "" {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	pathToken := c.Params("token")
	if subtle.ConstantTimeCompare([]byte(pathToken), []byte(h.telegramBotToken)) != 1 {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

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

	if userIDStr, ok := strings.CutPrefix(text, "/start "); ok {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return c.SendStatus(fiber.StatusOK)
		}

		config, _ := json.Marshal(map[string]any{"chat_id": chatID})
		_, err = h.alerts.CreateChannel(c.UserContext(), userID, app.CreateChannelInput{
			Name:   fmt.Sprintf("Telegram %d", chatID),
			Type:   "telegram",
			Config: config,
		})
		if err != nil {
			slog.Error("failed to create telegram channel", "user_id", userID, "error", err)
		} else {
			slog.Info("telegram channel created", "user_id", userID, "chat_id", chatID)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
