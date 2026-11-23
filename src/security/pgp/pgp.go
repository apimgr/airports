package pgp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const privateFilePrefix = "airports-pgp-private:v1:"

// KeySet contains the public and private armor and the OpenPGP v4 fingerprint.
type KeySet struct {
	Public      []byte
	Private     []byte
	Fingerprint string
	ExpiresAt   time.Time
}

// Generate creates the Ed25519 and Curve25519 keypair required for security reports.
func Generate(identity, email string, now time.Time) (*KeySet, error) {
	if strings.TrimSpace(identity) == "" || strings.TrimSpace(email) == "" {
		return nil, errors.New("OpenPGP identity and security contact are required")
	}
	config := &packet.Config{
		Algorithm:       packet.PubKeyAlgoEdDSA,
		Curve:           packet.Curve25519,
		KeyLifetimeSecs: uint32((2 * 365 * 24 * time.Hour) / time.Second),
		Time:            func() time.Time { return now.UTC() },
	}
	entity, err := openpgp.NewEntity(identity, "", email, config)
	if err != nil {
		return nil, fmt.Errorf("generate OpenPGP keypair: %w", err)
	}
	public, err := serializeEntity(entity, false)
	if err != nil {
		return nil, err
	}
	private, err := serializeEntity(entity, true)
	if err != nil {
		return nil, err
	}
	fingerprint := strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint))
	return &KeySet{Public: public, Private: private, Fingerprint: fingerprint, ExpiresAt: now.UTC().AddDate(2, 0, 0)}, nil
}

// ParsePrivate reads and validates a single armored private key.
func ParsePrivate(data []byte) (*openpgp.Entity, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse armored OpenPGP private key: %w", err)
	}
	if len(entities) != 1 || entities[0].PrivateKey == nil {
		return nil, errors.New("expected exactly one OpenPGP private key")
	}
	return entities[0], nil
}

// Fingerprint returns the OpenPGP v4 fingerprint in uppercase hexadecimal.
func Fingerprint(publicArmor []byte) (string, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicArmor))
	if err != nil {
		return "", fmt.Errorf("parse armored OpenPGP public key: %w", err)
	}
	if len(entities) != 1 || entities[0].PrimaryKey == nil || entities[0].PrivateKey != nil {
		return "", errors.New("expected exactly one OpenPGP public key")
	}
	return strings.ToUpper(hex.EncodeToString(entities[0].PrimaryKey.Fingerprint)), nil
}

// SerializePrivateArmor emits the full private key as ASCII armor.
func SerializePrivateArmor(entity *openpgp.Entity) ([]byte, error) {
	return serializeEntity(entity, true)
}

// SerializePublicArmor emits the public part of the key as ASCII armor.
func SerializePublicArmor(entity *openpgp.Entity) ([]byte, error) {
	return serializeEntity(entity, false)
}

// SignNewIdentity certifies a generated key identity with the previous private key.
func SignNewIdentity(newPrivateArmor, previousPrivateArmor []byte) ([]byte, []byte, error) {
	newEntity, err := ParsePrivate(newPrivateArmor)
	if err != nil {
		return nil, nil, err
	}
	previousEntity, err := ParsePrivate(previousPrivateArmor)
	if err != nil {
		return nil, nil, err
	}
	identity := newEntity.PrimaryIdentity()
	if identity == nil {
		return nil, nil, errors.New("new OpenPGP key has no primary identity")
	}
	if err := newEntity.SignIdentity(identity.Name, previousEntity, nil); err != nil {
		return nil, nil, fmt.Errorf("sign new OpenPGP identity: %w", err)
	}
	private, err := SerializePrivateArmor(newEntity)
	if err != nil {
		return nil, nil, err
	}
	public, err := SerializePublicArmor(newEntity)
	if err != nil {
		return nil, nil, err
	}
	return private, public, nil
}

// EncryptTo encrypts message bytes to a single armored OpenPGP public key.
func EncryptTo(publicArmor, message []byte) ([]byte, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicArmor))
	if err != nil {
		return nil, fmt.Errorf("parse OpenPGP recipient key: %w", err)
	}
	if len(entities) != 1 || entities[0].PrimaryKey == nil {
		return nil, errors.New("expected exactly one OpenPGP recipient key")
	}
	var encrypted bytes.Buffer
	writer, err := openpgp.Encrypt(&encrypted, []*openpgp.Entity{entities[0]}, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt OpenPGP message: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return encrypted.Bytes(), nil
}

func serializeEntity(entity *openpgp.Entity, private bool) ([]byte, error) {
	var output bytes.Buffer
	kind := openpgp.PublicKeyType
	if private {
		kind = openpgp.PrivateKeyType
	}
	writer, err := armor.Encode(&output, kind, nil)
	if err != nil {
		return nil, fmt.Errorf("create OpenPGP armor: %w", err)
	}
	if private {
		err = entity.SerializePrivate(writer, nil)
	} else {
		err = entity.Serialize(writer)
	}
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("serialize OpenPGP key: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close OpenPGP armor: %w", err)
	}
	return output.Bytes(), nil
}

// EncryptPrivate encrypts private armor using a key derived from installationSecret.
func EncryptPrivate(installationSecret, plaintext []byte) ([]byte, error) {
	key, err := deriveKey(installationSecret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, []byte(privateFilePrefix))
	payload := append(nonce, ciphertext...)
	encoded := base64.StdEncoding.EncodeToString(payload)
	return []byte(privateFilePrefix + encoded + "\n"), nil
}

// DecryptPrivate decrypts the encrypted private-key file with installationSecret.
func DecryptPrivate(installationSecret, ciphertext []byte) ([]byte, error) {
	value := strings.TrimSpace(string(ciphertext))
	if !strings.HasPrefix(value, privateFilePrefix) {
		return nil, errors.New("encrypted OpenPGP private key has an unsupported format")
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, privateFilePrefix))
	if err != nil {
		return nil, errors.New("encrypted OpenPGP private key is malformed")
	}
	key, err := deriveKey(installationSecret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("encrypted OpenPGP private key is truncated")
	}
	plain, err := aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], []byte(privateFilePrefix))
	if err != nil {
		return nil, errors.New("encrypted OpenPGP private key could not be authenticated")
	}
	return plain, nil
}

func deriveKey(secret []byte) ([]byte, error) {
	if len(secret) != 32 {
		return nil, errors.New("installation secret must be 32 bytes")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("airports/openpgp/private-key/v1"))
	return mac.Sum(nil), nil
}

// WritePrivate atomically creates or replaces a private key file with mode 0600.
func WritePrivate(path string, installationSecret, privateArmor []byte) error {
	content, err := EncryptPrivate(installationSecret, privateArmor)
	if err != nil {
		return err
	}
	return atomicPrivateWrite(path, content)
}

// ReadPrivate reads and decrypts a private key file.
func ReadPrivate(path string, installationSecret []byte) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecryptPrivate(installationSecret, content)
}

func atomicPrivateWrite(path string, content []byte) (resultErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create private key directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".pgp-private-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err := os.Remove(tmpName); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary private key file: %w", err))
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}
