package bootstrap

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/port"
)

// InitCipher creates a port.Cipher from encryption config.
// Lives in bootstrap (composition root) — not in config — to keep config
// free of adapter dependencies (hex-arch).
// Returns NoOpCipher when ENCRYPTION_KEYS is empty (encryption disabled).
func InitCipher(cfg config.EncryptionConfig) (port.Cipher, error) {
	if cfg.EncryptionKeys == "" {
		slog.Info("encryption disabled — no ENCRYPTION_KEYS configured")
		return crypto.NoOpCipher{}, nil
	}

	keys, err := crypto.ParseKeysEnv(cfg.EncryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("parse ENCRYPTION_KEYS: %w", err)
	}

	v, err := strconv.Atoi(cfg.EncryptionPrimaryVersion)
	if err != nil || v < 1 || v > 255 {
		return nil, fmt.Errorf("invalid ENCRYPTION_PRIMARY_VERSION: %q", cfg.EncryptionPrimaryVersion)
	}
	primaryVersion := byte(v)

	enc, err := crypto.NewEncryptor(primaryVersion, keys)
	if err != nil {
		return nil, fmt.Errorf("initialize encryption: %w", err)
	}
	slog.Info("encryption enabled", "primary_version", primaryVersion, "total_keys", len(keys))
	return enc, nil
}
