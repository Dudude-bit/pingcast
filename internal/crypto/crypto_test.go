package crypto

import (
	"context"
	"encoding/base64"
	"testing"
)

func testKeyB64(t *testing.T, seed byte) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecrypt(t *testing.T) {
	enc, err := NewEncryptor(1, map[byte]string{1: testKeyB64(t, 0)})
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte(`{"url":"https://hooks.slack.com/secret-token"}`)
	ctx := context.Background()

	ciphertext, err := enc.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if ciphertext == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := enc.Decrypt(ctx, ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestKeyRotation(t *testing.T) {
	ctx := context.Background()
	key1 := testKeyB64(t, 0)
	key2 := testKeyB64(t, 100)

	// Encrypt with key version 1
	enc1, err := NewEncryptor(1, map[byte]string{1: key1})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := enc1.Encrypt(ctx, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	// New encryptor: primary=2, but version 1 still available
	enc2, err := NewEncryptor(2, map[byte]string{1: key1, 2: key2})
	if err != nil {
		t.Fatal(err)
	}

	// Should decrypt data encrypted with version 1
	decrypted, err := enc2.Decrypt(ctx, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != "secret" {
		t.Fatalf("got %q, want %q", decrypted, "secret")
	}

	// NeedsReEncryption should be true (encrypted with v1, primary is v2)
	if !enc2.NeedsReEncryption(ciphertext) {
		t.Fatal("expected NeedsReEncryption=true for old key version")
	}

	// New encryption uses version 2
	ct2, err := enc2.Encrypt(ctx, []byte("new-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if enc2.NeedsReEncryption(ct2) {
		t.Fatal("expected NeedsReEncryption=false for current key version")
	}

	dec2, err := enc2.Decrypt(ctx, ct2)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec2) != "new-secret" {
		t.Fatalf("got %q, want %q", dec2, "new-secret")
	}
}

func TestThreeKeyRotation(t *testing.T) {
	ctx := context.Background()
	k1 := testKeyB64(t, 0)
	k2 := testKeyB64(t, 50)
	k3 := testKeyB64(t, 100)

	// Encrypt with each version
	enc1, _ := NewEncryptor(1, map[byte]string{1: k1})
	ct1, _ := enc1.Encrypt(ctx, []byte("v1-data"))

	enc2, _ := NewEncryptor(2, map[byte]string{1: k1, 2: k2})
	ct2, _ := enc2.Encrypt(ctx, []byte("v2-data"))

	// Encryptor with all 3 keys, primary=3
	enc3, _ := NewEncryptor(3, map[byte]string{1: k1, 2: k2, 3: k3})

	// Should decrypt all versions
	for _, tc := range []struct {
		ct   string
		want string
	}{
		{ct1, "v1-data"},
		{ct2, "v2-data"},
	} {
		dec, err := enc3.Decrypt(ctx, tc.ct)
		if err != nil {
			t.Fatalf("decrypt %q: %v", tc.want, err)
		}
		if string(dec) != tc.want {
			t.Fatalf("got %q, want %q", dec, tc.want)
		}
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := NewEncryptor(1, map[byte]string{1: "not-base64!"})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	shortKey := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err = NewEncryptor(1, map[byte]string{1: shortKey})
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestWrongKeyDecrypt(t *testing.T) {
	ctx := context.Background()
	enc1, _ := NewEncryptor(1, map[byte]string{1: testKeyB64(t, 0)})
	enc2, _ := NewEncryptor(2, map[byte]string{2: testKeyB64(t, 100)})

	ct, _ := enc1.Encrypt(ctx, []byte("hello"))
	_, err := enc2.Decrypt(ctx, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestVersionZeroRejected(t *testing.T) {
	_, err := NewEncryptor(0, map[byte]string{0: testKeyB64(t, 0)})
	if err == nil {
		t.Fatal("expected error for version 0")
	}
}

func TestTamperedCiphertext(t *testing.T) {
	ctx := context.Background()
	enc, _ := NewEncryptor(1, map[byte]string{1: testKeyB64(t, 0)})

	ct, _ := enc.Encrypt(ctx, []byte("secret"))
	raw, _ := base64.StdEncoding.DecodeString(ct)

	// Flip a byte in the ciphertext portion
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err := enc.Decrypt(ctx, tampered)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestTamperedVersionByte(t *testing.T) {
	ctx := context.Background()
	enc, _ := NewEncryptor(1, map[byte]string{1: testKeyB64(t, 0), 2: testKeyB64(t, 50)})

	ct, _ := enc.Encrypt(ctx, []byte("secret"))
	raw, _ := base64.StdEncoding.DecodeString(ct)

	// Swap version byte from 1 to 2 — AD mismatch should cause GCM tag failure
	raw[0] = 2
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err := enc.Decrypt(ctx, tampered)
	if err == nil {
		t.Fatal("expected error for tampered version byte (AD mismatch)")
	}
}

func TestTruncatedCiphertext(t *testing.T) {
	ctx := context.Background()
	enc, _ := NewEncryptor(1, map[byte]string{1: testKeyB64(t, 0)})

	for _, size := range []int{0, 1, 12, 13, 28} {
		raw := make([]byte, size)
		short := base64.StdEncoding.EncodeToString(raw)
		_, err := enc.Decrypt(ctx, short)
		if err == nil {
			t.Fatalf("expected error for truncated ciphertext of size %d", size)
		}
	}
}

func TestReEncrypt(t *testing.T) {
	ctx := context.Background()
	k1 := testKeyB64(t, 0)
	k2 := testKeyB64(t, 50)

	enc1, _ := NewEncryptor(1, map[byte]string{1: k1})
	ct1, _ := enc1.Encrypt(ctx, []byte("data"))

	enc2, _ := NewEncryptor(2, map[byte]string{1: k1, 2: k2})

	if !enc2.NeedsReEncryption(ct1) {
		t.Fatal("should need re-encryption")
	}

	ct2, err := enc2.ReEncrypt(ctx, ct1)
	if err != nil {
		t.Fatal(err)
	}

	if enc2.NeedsReEncryption(ct2) {
		t.Fatal("should not need re-encryption after ReEncrypt")
	}

	dec, _ := enc2.Decrypt(ctx, ct2)
	if string(dec) != "data" {
		t.Fatalf("got %q", dec)
	}
}

func TestParseKeysEnv(t *testing.T) {
	keys, err := ParseKeysEnv("1:AAAA,2:BBBB")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[1] != "AAAA" || keys[2] != "BBBB" {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

