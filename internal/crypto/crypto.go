package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encryptor handles AES-256-GCM encryption/decryption with key rotation support.
type Encryptor struct {
	primary cipher.AEAD
	old     cipher.AEAD // for decryption during key rotation, may be nil
}

// NewEncryptor creates an Encryptor from a base64-encoded 32-byte key.
// An optional old key is used for decryption during key rotation.
func NewEncryptor(keyBase64 string, oldKeyBase64 string) (*Encryptor, error) {
	primary, err := newAEAD(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("primary key: %w", err)
	}

	var old cipher.AEAD
	if oldKeyBase64 != "" {
		old, err = newAEAD(oldKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("old key: %w", err)
		}
	}

	return &Encryptor{primary: primary, old: old}, nil
}

func newAEAD(keyBase64 string) (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns base64-encoded ciphertext (nonce prepended).
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.primary.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := e.primary.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext. Tries primary key first,
// falls back to old key for rotation support.
func (e *Encryptor) Decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	plaintext, err := e.decryptWith(e.primary, data)
	if err == nil {
		return plaintext, nil
	}

	if e.old != nil {
		plaintext, oldErr := e.decryptWith(e.old, data)
		if oldErr == nil {
			return plaintext, nil
		}
	}

	return nil, fmt.Errorf("decrypt: %w", err)
}

func (e *Encryptor) decryptWith(aead cipher.AEAD, data []byte) ([]byte, error) {
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, nil)
}
