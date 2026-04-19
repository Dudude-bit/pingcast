package domain

import "encoding/json"

// RedactChannelConfig replaces sensitive fields in a channel's config
// with "***" + the last 4 chars of the original value. Public API
// responses use this to prevent secrets from leaving the server once
// they're stored.
//
// Spec §8.9. Production code calls this on every channel read path.
//
// Unknown channel types pass through untouched — a no-op is safer than
// a silent transformation.
func RedactChannelConfig(t ChannelType, raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}

	switch t {
	case ChannelTelegram:
		redactField(m, "bot_token")
	case ChannelWebhook:
		redactField(m, "url")
	case ChannelEmail:
		redactField(m, "smtp_password")
	}

	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

func redactField(m map[string]any, key string) {
	v, ok := m[key].(string)
	if !ok || v == "" {
		return
	}
	m[key] = mask(v)
}

// mask returns "***" followed by the last 4 chars of v, or plain "***"
// if v is too short to preserve any tail.
func mask(v string) string {
	if len(v) <= 4 {
		return "***"
	}
	return "***" + v[len(v)-4:]
}
