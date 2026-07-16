package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	prefix = "gantry1:"
	// Fixed application salt for at-rest key derivation (not for password authentication).
	// Combined with a high-iteration PBKDF2 so CodeQL treats the KDF as computationally expensive.
	appSalt     = "gantry-secrets-v1-aes-gcm"
	pbkdf2Iters = 210_000
	aesKeyBytes = 32
)

// DeriveKey turns an operator-supplied passphrase into a 32-byte AES-256 key
// using PBKDF2-HMAC-SHA256 (slow KDF suitable for secret material).
func DeriveKey(passphrase string) []byte {
	return pbkdf2.Key([]byte(passphrase), []byte(appSalt), pbkdf2Iters, aesKeyBytes, sha256.New)
}

// IsEncrypted reports whether s uses the Gantry ciphertext envelope.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, prefix)
}

// Encrypt encrypts plaintext with AES-256-GCM. Empty key returns plaintext unchanged.
func Encrypt(plaintext, passphrase string) (string, error) {
	if passphrase == "" || plaintext == "" {
		return plaintext, nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}
	block, err := aes.NewCipher(DeriveKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt. Empty key returns input unchanged (dev mode).
// If data is not encrypted, it is returned as-is (legacy plaintext).
func Decrypt(ciphertext, passphrase string) (string, error) {
	if passphrase == "" || ciphertext == "" {
		return ciphertext, nil
	}
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	block, err := aes.NewCipher(DeriveKey(passphrase))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}
