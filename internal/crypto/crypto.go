package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.Cipher = (*Encryptor)(nil)

// Ciphertext format (binary, pre-base64):
//   [1-byte version][12-byte nonce][ciphertext + 16-byte GCM tag]
//
// AEAD associated data: []byte{version} — binds version to ciphertext, prevents downgrade.
// Version 0x00 is reserved (sentinel for unencrypted data detection).

const nonceSize = 12 // AES-GCM standard

// Encryptor handles AES-256-GCM encryption/decryption with key versioning.
// Supports N keys simultaneously for seamless rotation.
type Encryptor struct {
	primary byte
	keys    map[byte]cipher.AEAD
}

// NewEncryptor creates an Encryptor from a map of version -> base64-encoded 32-byte keys.
// primaryVersion specifies which key is used for encryption.
func NewEncryptor(primaryVersion byte, keys map[byte]string) (*Encryptor, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one key required")
	}
	if _, ok := keys[primaryVersion]; !ok {
		return nil, fmt.Errorf("primary version %d not found in keys", primaryVersion)
	}

	aeadKeys := make(map[byte]cipher.AEAD, len(keys))
	for ver, keyBase64 := range keys {
		if ver == 0 {
			return nil, fmt.Errorf("version 0 is reserved")
		}
		aead, err := newAEAD(keyBase64)
		if err != nil {
			return nil, fmt.Errorf("key version %d: %w", ver, err)
		}
		aeadKeys[ver] = aead
	}

	return &Encryptor{primary: primaryVersion, keys: aeadKeys}, nil
}

// ParseKeysEnv parses ENCRYPTION_KEYS format: "1:base64key,2:base64key,3:base64key"
func ParseKeysEnv(s string) (map[byte]string, error) {
	keys := make(map[byte]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colonIdx := strings.IndexByte(part, ':')
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid key format %q, expected version:base64key", part)
		}
		ver, err := strconv.Atoi(part[:colonIdx])
		if err != nil || ver < 1 || ver > 255 {
			return nil, fmt.Errorf("invalid key version %q", part[:colonIdx])
		}
		keys[byte(ver)] = part[colonIdx+1:]
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys found")
	}
	return keys, nil
}

func (e *Encryptor) Encrypt(_ context.Context, plaintext []byte) (string, error) {
	aead := e.keys[e.primary]
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ad := []byte{e.primary}
	ciphertext := aead.Seal(nil, nonce, plaintext, ad)

	// [version][nonce][ciphertext+tag]
	out := make([]byte, 0, 1+nonceSize+len(ciphertext))
	out = append(out, e.primary)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (e *Encryptor) Decrypt(_ context.Context, encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(data) < 1+nonceSize+16 { // version + nonce + min GCM tag
		return nil, fmt.Errorf("ciphertext too short")
	}

	version := data[0]
	aead, ok := e.keys[version]
	if !ok {
		return nil, fmt.Errorf("unknown key version %d", version)
	}

	nonce := data[1 : 1+nonceSize]
	ct := data[1+nonceSize:]
	ad := []byte{version}

	return aead.Open(nil, nonce, ct, ad)
}

func (e *Encryptor) ReEncrypt(ctx context.Context, encoded string) (string, error) {
	if !e.NeedsReEncryption(encoded) {
		return encoded, nil
	}
	plaintext, err := e.Decrypt(ctx, encoded)
	if err != nil {
		return "", err
	}
	return e.Encrypt(ctx, plaintext)
}

func (e *Encryptor) NeedsReEncryption(encoded string) bool {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 1 {
		return true
	}
	return data[0] != e.primary
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
