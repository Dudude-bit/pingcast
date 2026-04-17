package port

import "context"

// Cipher encrypts and decrypts sensitive data with key versioning.
// Implementations handle key management, nonces, and format encoding internally.
type Cipher interface {
	// Encrypt encrypts plaintext and returns base64-encoded ciphertext
	// with a version prefix for key identification.
	Encrypt(ctx context.Context, plaintext []byte) (string, error)

	// Decrypt decrypts base64-encoded ciphertext, selecting the correct
	// key via the version prefix. O(1) key lookup, no brute-force.
	Decrypt(ctx context.Context, encrypted string) ([]byte, error)

	// NeedsReEncryption returns true if the ciphertext was encrypted
	// with a non-primary key and should be re-encrypted.
	NeedsReEncryption(encrypted string) bool

	// ReEncrypt decrypts and re-encrypts with the current primary key.
	// No-op if already encrypted with the primary key.
	ReEncrypt(ctx context.Context, encrypted string) (string, error)
}
