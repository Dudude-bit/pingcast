package httpadapter

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
}

func NewWebhookHandler(
	auth *app.AuthService,
	alerts *app.AlertService,
	billing *app.BillingService,
	lemonSqueezySecret string,
	founderVariantID string,
) *WebhookHandler {
	return &WebhookHandler{
		auth:               auth,
		alerts:             alerts,
		billing:            billing,
		lemonSqueezySecret: lemonSqueezySecret,
		founderVariantID:   founderVariantID,
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
				slog.Error("failed to upgrade user", "user_id", user.ID, "error", err)
			} else {
				slog.Info("user upgraded to pro", "user_id", user.ID)
			}
			// Mark variant so CountActiveFounderSubscriptions reflects
			// the new seat. 'founder' vs 'retail' decision is based on
			// the variant ID LemonSqueezy sent us against the env we
			// stashed at boot. Errors are logged but don't fail the
			// webhook — the upgrade itself is what the user cares
			// about; variant is just for cap accounting.
			variant := "retail"
			if h.founderVariantID != "" &&
				fmt.Sprintf("%d", webhook.Data.Attributes.VariantID) == h.founderVariantID {
				variant = "founder"
			}
			if err := h.billing.SetSubscriptionVariant(c.UserContext(), user.ID, variant); err != nil {
				slog.Error("failed to set subscription variant",
					"user_id", user.ID, "variant", variant, "error", err)
			}
		}
	case "subscription_cancelled":
		if err := h.auth.DowngradeToFree(c.UserContext(), user.ID); err != nil {
			slog.Error("failed to downgrade user", "user_id", user.ID, "error", err)
		} else {
			slog.Info("user downgraded to free", "user_id", user.ID)
		}
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
