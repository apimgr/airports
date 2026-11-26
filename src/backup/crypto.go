// Package backup implements AI.md PART 21 "BACKUP & RESTORE" — creating,
// encrypting, verifying, retaining, and restoring server backups.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time       = 1
	argon2Memory     = 64 * 1024
	argon2Threads    = 4
	argon2KeyLen     = 32
	saltSize         = 16
	nonceSize        = 12
	backupHeaderSize = 55
	maxBaseNameSize  = 255
)

var backupMagic = []byte("BKUP1")

// EncryptionHeader describes the cleartext metadata required to authenticate
// and decrypt one encrypted backup.
type EncryptionHeader struct {
	Salt        []byte
	Memory      uint32
	Iterations  uint32
	Parallelism uint32
	Nonce       []byte
	KeyID       string
	BaseName    string
	Bytes       []byte
}

func deriveKeyParams(password string, salt []byte, memory, iterations, parallelism uint32) []byte {
	return argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), argon2KeyLen)
}

func deriveKey(password string, salt []byte) []byte {
	return deriveKeyParams(password, salt, argon2Memory, argon2Time, argon2Threads)
}

func keyID(key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("key-id"))
	return hex.EncodeToString(mac.Sum(nil)[:4])
}

// EncryptArchive encrypts a full backup. Use EncryptArchiveWithBase for an
// incremental archive whose header names its base backup.
func EncryptArchive(password string, plaintext []byte) ([]byte, error) {
	return EncryptArchiveWithBase(password, plaintext, "")
}

// EncryptArchiveWithBase encrypts an archive using the PART 21 BKUP1 header.
func EncryptArchiveWithBase(password string, plaintext []byte, baseName string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("backup: encryption password must not be empty")
	}
	if err := validateBaseName(baseName); err != nil {
		return nil, err
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
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("backup: generate nonce: %w", err)
	}
	header := make([]byte, backupHeaderSize+len(baseName))
	copy(header[:5], backupMagic)
	copy(header[5:21], salt)
	binary.BigEndian.PutUint32(header[21:25], argon2Memory)
	binary.BigEndian.PutUint32(header[25:29], argon2Time)
	binary.BigEndian.PutUint32(header[29:33], argon2Threads)
	copy(header[33:45], nonce)
	copy(header[45:53], keyID(key))
	binary.BigEndian.PutUint16(header[53:55], uint16(len(baseName)))
	copy(header[55:], baseName)
	ciphertext := gcm.Seal(nil, nonce, plaintext, header)
	return append(header, ciphertext...), nil
}

func validateBaseName(name string) error {
	if len(name) > maxBaseNameSize || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("backup: invalid base backup name")
	}
	return nil
}

// ParseEncryptionHeader rejects malformed fields before any password-derived
// work. Current archives use the fixed Argon2id parameters emitted above.
func ParseEncryptionHeader(blob []byte) (*EncryptionHeader, error) {
	if len(blob) < backupHeaderSize {
		return nil, fmt.Errorf("backup: encrypted archive header is truncated")
	}
	if subtle.ConstantTimeCompare(blob[:5], backupMagic) != 1 {
		return nil, fmt.Errorf("backup: invalid encrypted archive magic")
	}
	memory := binary.BigEndian.Uint32(blob[21:25])
	iterations := binary.BigEndian.Uint32(blob[25:29])
	parallelism := binary.BigEndian.Uint32(blob[29:33])
	if memory != argon2Memory || iterations != argon2Time || parallelism != argon2Threads {
		return nil, fmt.Errorf("backup: unsupported Argon2id parameters")
	}
	keyIDBytes := blob[45:53]
	for _, b := range keyIDBytes {
		isHexDigit := b >= '0' && b <= '9' || b >= 'a' && b <= 'f'
		if !isHexDigit {
			return nil, fmt.Errorf("backup: invalid key_id")
		}
	}
	baseLen := int(binary.BigEndian.Uint16(blob[53:55]))
	if baseLen > maxBaseNameSize {
		return nil, fmt.Errorf("backup: base backup name exceeds 255 bytes")
	}
	headerLen := backupHeaderSize + baseLen
	if len(blob) < headerLen {
		return nil, fmt.Errorf("backup: encrypted archive header is truncated")
	}
	baseName := string(blob[55:headerLen])
	if err := validateBaseName(baseName); err != nil {
		return nil, err
	}
	return &EncryptionHeader{Salt: append([]byte(nil), blob[5:21]...), Memory: memory, Iterations: iterations, Parallelism: parallelism, Nonce: append([]byte(nil), blob[33:45]...), KeyID: string(keyIDBytes), BaseName: baseName, Bytes: append([]byte(nil), blob[:headerLen]...)}, nil
}

// PasswordMatchesHeader compares a candidate password with an archive's key_id.
func PasswordMatchesHeader(password string, header *EncryptionHeader) (bool, error) {
	if header == nil {
		return false, fmt.Errorf("backup: encryption header is nil")
	}
	if password == "" {
		return false, fmt.Errorf("backup: encryption password must not be empty")
	}
	key := deriveKeyParams(password, header.Salt, header.Memory, header.Iterations, header.Parallelism)
	return subtle.ConstantTimeCompare([]byte(keyID(key)), []byte(header.KeyID)) == 1, nil
}

// DecryptArchive authenticates the complete header as AES-GCM additional data.
func DecryptArchive(password string, blob []byte) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("backup: decryption password must not be empty")
	}
	header, err := ParseEncryptionHeader(blob)
	if err != nil {
		return nil, err
	}
	if len(blob) < len(header.Bytes)+16 {
		return nil, fmt.Errorf("backup: encrypted archive too short or corrupted")
	}
	key := deriveKeyParams(password, header.Salt, header.Memory, header.Iterations, header.Parallelism)
	if subtle.ConstantTimeCompare([]byte(keyID(key)), []byte(header.KeyID)) != 1 {
		return nil, fmt.Errorf("backup: decrypt failed (wrong password or corrupted archive)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("backup: init AES-GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, header.Nonce, blob[len(header.Bytes):], header.Bytes)
	if err != nil {
		return nil, fmt.Errorf("backup: decrypt failed (wrong password or corrupted archive): %w", err)
	}
	return plaintext, nil
}
