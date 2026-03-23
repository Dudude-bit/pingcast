package crypto

import (
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func testKey2(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecrypt(t *testing.T) {
	enc, err := NewEncryptor(testKey(t), "")
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte(`{"url":"https://hooks.slack.com/secret-token"}`)

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if ciphertext == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestKeyRotation(t *testing.T) {
	oldKey := testKey(t)
	newKey := testKey2(t)

	// Encrypt with old key
	oldEnc, err := NewEncryptor(oldKey, "")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := oldEnc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	// New encryptor with new primary key + old key for rotation
	newEnc, err := NewEncryptor(newKey, oldKey)
	if err != nil {
		t.Fatal(err)
	}

	// Should decrypt data encrypted with old key
	decrypted, err := newEnc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != "secret" {
		t.Fatalf("got %q, want %q", decrypted, "secret")
	}

	// Should also encrypt/decrypt with new key
	ct2, err := newEnc.Encrypt([]byte("new-secret"))
	if err != nil {
		t.Fatal(err)
	}
	dec2, err := newEnc.Decrypt(ct2)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec2) != "new-secret" {
		t.Fatalf("got %q, want %q", dec2, "new-secret")
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := NewEncryptor("not-base64!", "")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	shortKey := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err = NewEncryptor(shortKey, "")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestWrongKeyDecrypt(t *testing.T) {
	enc1, _ := NewEncryptor(testKey(t), "")
	enc2, _ := NewEncryptor(testKey2(t), "")

	ct, _ := enc1.Encrypt([]byte("hello"))
	_, err := enc2.Decrypt(ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}
