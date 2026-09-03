package ssl

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// EncryptDNSCredentials serializes a DNS-01 provider's credential fields
// (arbitrary lego env-var-name -> value pairs) to JSON and encrypts them
// with AES-256-GCM, keyed by server.security.encryption_key, per AI.md
// PART 15 "Provider Credential Storage" (credentials_encrypted field) and
// backend-rules.md's AES-256-GCM at-rest requirement. The key is hashed
// with SHA-256 first so any non-empty operator-supplied string yields a
// valid 32-byte AES-256 key. Returns a base64-encoded "nonce+ciphertext"
// blob suitable for the server.yml dns_credentials field.
func EncryptDNSCredentials(encryptionKey string, creds map[string]string) (string, error) {
	if encryptionKey == "" {
		return "", fmt.Errorf("server.security.encryption_key is not configured")
	}
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("failed to marshal dns credentials: %w", err)
	}

	block, err := newAESCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to init AES-GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptDNSCredentials reverses EncryptDNSCredentials, returning the
// provider's plaintext credential fields for use with BuildDNSProvider.
// Never log or expose the returned map — it contains live secrets.
func DecryptDNSCredentials(encryptionKey, encoded string) (map[string]string, error) {
	if encryptionKey == "" {
		return nil, fmt.Errorf("server.security.encryption_key is not configured")
	}
	if encoded == "" {
		return map[string]string{}, nil
	}

	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode dns credentials: %w", err)
	}

	block, err := newAESCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to init AES-GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(sealed) < nonceSize {
		return nil, fmt.Errorf("dns credentials ciphertext too short")
	}
	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt dns credentials: %w", err)
	}

	creds := map[string]string{}
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dns credentials: %w", err)
	}
	return creds, nil
}

// newAESCipher derives a 32-byte AES-256 key from an arbitrary-length
// operator-supplied key string via SHA-256.
func newAESCipher(encryptionKey string) (cipher.Block, error) {
	sum := sha256.Sum256([]byte(encryptionKey))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("failed to init AES cipher: %w", err)
	}
	return block, nil
}
