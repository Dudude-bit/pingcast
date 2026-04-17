package crypto

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.Cipher = NoOpCipher{}

// NoOpCipher is used when encryption is disabled. It passes data through
// untouched, so repos can depend on port.Cipher unconditionally without
// nil checks or dual constructors.
type NoOpCipher struct{}

func (NoOpCipher) Encrypt(_ context.Context, plaintext []byte) (string, error) {
	return string(plaintext), nil
}

func (NoOpCipher) Decrypt(_ context.Context, encrypted string) ([]byte, error) {
	return []byte(encrypted), nil
}

func (NoOpCipher) NeedsReEncryption(_ string) bool {
	return false
}

func (NoOpCipher) ReEncrypt(_ context.Context, encrypted string) (string, error) {
	return encrypted, nil
}
