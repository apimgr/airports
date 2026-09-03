// Package backup implements AI.md PART 21 "BACKUP & RESTORE" — creating,
// encrypting, verifying, retaining, and restoring server backups.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for backup password key derivation, per AI.md PART 21
// "Key Derivation | Argon2id (password -> encryption key)". These are
// deliberately modest (time=1) since backups can be large and this runs on
// every create/verify/restore; the random 16-byte salt still makes rainbow
// tables infeasible.
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32 // AES-256
	saltSize      = 16
)

// deriveKey runs password through Argon2id with the given salt to produce a
// 32-byte AES-256 key.
func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

// EncryptArchive encrypts plaintext (an in-memory tar.gz archive) with a key
// derived from password via Argon2id, per AI.md PART 21 "How Encryption
// Works". Output format: salt(16) || nonce(gcm.NonceSize()) || ciphertext.
// The salt is stored alongside the ciphertext (not secret) so DecryptArchive
// can re-derive the same key.
func EncryptArchive(password string, plaintext []byte) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("backup: encryption password must not be empty")
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("backup: generate salt: %w", err)
	}

	key := deriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES-GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("backup: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptArchive reverses EncryptArchive. GCM's built-in authentication tag
// makes this safe against tampering and wrong-password attempts without any
// extra constant-time comparison — Open fails closed on any mismatch.
func DecryptArchive(password string, blob []byte) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("backup: decryption password must not be empty")
	}

	// Standard AES-GCM nonce size is always 12 bytes regardless of key size,
	// so this can be checked before the key is derived from the salt below.
	const nonceSize = 12

	if len(blob) < saltSize+nonceSize {
		return nil, fmt.Errorf("backup: encrypted archive too short or corrupted")
	}

	salt := blob[:saltSize]
	nonce := blob[saltSize : saltSize+nonceSize]
	ciphertext := blob[saltSize+nonceSize:]

	key := deriveKey(password, salt)
	dataBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES cipher: %w", err)
	}
	dataGCM, err := cipher.NewGCM(dataBlock)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES-GCM: %w", err)
	}

	plaintext, err := dataGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("backup: decrypt failed (wrong password or corrupted archive): %w", err)
	}
	return plaintext, nil
}
