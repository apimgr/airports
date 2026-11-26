package pgp

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateAndSerializeOpenPGPKeys(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.FixedZone("test", -5*60*60))
	if _, err := Generate(" ", "security@example.test", now); err == nil {
		t.Fatal("Generate accepted an empty identity")
	}
	if _, err := Generate("Airports Security", " ", now); err == nil {
		t.Fatal("Generate accepted an empty email")
	}
	keys, err := Generate("Airports Security", "security@example.test", now)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(keys.Fingerprint) != 40 || keys.Fingerprint != strings.ToUpper(keys.Fingerprint) {
		t.Fatalf("fingerprint = %q, want uppercase OpenPGP v4 fingerprint", keys.Fingerprint)
	}
	if !keys.ExpiresAt.Equal(now.UTC().AddDate(2, 0, 0)) {
		t.Fatalf("ExpiresAt = %v, want two years from UTC generation time", keys.ExpiresAt)
	}
	entity, err := ParsePrivate(keys.Private)
	if err != nil {
		t.Fatalf("ParsePrivate() error = %v", err)
	}
	fingerprint, err := Fingerprint(keys.Public)
	if err != nil || fingerprint != keys.Fingerprint {
		t.Fatalf("Fingerprint() = (%q, %v), want %q", fingerprint, err, keys.Fingerprint)
	}
	public, err := SerializePublicArmor(entity)
	if err != nil || !bytes.Contains(public, []byte("BEGIN PGP PUBLIC KEY BLOCK")) {
		t.Fatalf("SerializePublicArmor() error = %v", err)
	}
	private, err := SerializePrivateArmor(entity)
	if err != nil || !bytes.Contains(private, []byte("BEGIN PGP PRIVATE KEY BLOCK")) {
		t.Fatalf("SerializePrivateArmor() error = %v", err)
	}
	if _, err := ParsePrivate(keys.Public); err == nil {
		t.Fatal("ParsePrivate accepted a public key")
	}
	if _, err := Fingerprint(keys.Private); err == nil {
		t.Fatal("Fingerprint accepted private armor where public armor is required")
	}
	if _, err := Fingerprint([]byte("invalid armor")); err == nil {
		t.Fatal("Fingerprint accepted invalid armor")
	}
}

func TestSignAndEncryptOpenPGP(t *testing.T) {
	now := time.Now().UTC()
	oldKey, err := Generate("Old", "old@example.test", now)
	if err != nil {
		t.Fatal(err)
	}
	newKey, err := Generate("New", "new@example.test", now)
	if err != nil {
		t.Fatal(err)
	}
	private, public, err := SignNewIdentity(newKey.Private, oldKey.Private)
	if err != nil {
		t.Fatalf("SignNewIdentity() error = %v", err)
	}
	if _, err := ParsePrivate(private); err != nil {
		t.Fatalf("ParsePrivate(signed key) error = %v", err)
	}
	if fingerprint, err := Fingerprint(public); err != nil || fingerprint != newKey.Fingerprint {
		t.Fatalf("Fingerprint(signed key) = (%q, %v), want %q", fingerprint, err, newKey.Fingerprint)
	}
	if _, _, err := SignNewIdentity([]byte("bad"), oldKey.Private); err == nil {
		t.Fatal("SignNewIdentity accepted an invalid new key")
	}
	if _, _, err := SignNewIdentity(newKey.Private, []byte("bad")); err == nil {
		t.Fatal("SignNewIdentity accepted an invalid previous key")
	}
	ciphertext, err := EncryptTo(newKey.Public, []byte("security report"))
	if err != nil || len(ciphertext) == 0 {
		t.Fatalf("EncryptTo() returned %d bytes, error = %v", len(ciphertext), err)
	}
	if _, err := EncryptTo([]byte("bad"), []byte("report")); err == nil {
		t.Fatal("EncryptTo accepted an invalid recipient key")
	}
}

func TestPrivateKeyEncryptionAndAtomicStorage(t *testing.T) {
	secret := bytes.Repeat([]byte{0x42}, 32)
	plaintext := []byte("armored private key")
	if _, err := EncryptPrivate(secret[:31], plaintext); err == nil {
		t.Fatal("EncryptPrivate accepted a short installation secret")
	}
	ciphertext, err := EncryptPrivate(secret, plaintext)
	if err != nil {
		t.Fatalf("EncryptPrivate() error = %v", err)
	}
	if !bytes.HasPrefix(ciphertext, []byte(privateFilePrefix)) {
		t.Fatal("EncryptPrivate did not write the versioned file prefix")
	}
	decrypted, err := DecryptPrivate(secret, ciphertext)
	if err != nil || !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("DecryptPrivate() = (%q, %v), want original plaintext", decrypted, err)
	}
	if _, err := DecryptPrivate(secret[:31], ciphertext); err == nil {
		t.Fatal("DecryptPrivate accepted a short secret")
	}
	if _, err := DecryptPrivate(secret, []byte("unsupported")); err == nil {
		t.Fatal("DecryptPrivate accepted an unsupported format")
	}
	if _, err := DecryptPrivate(secret, []byte(privateFilePrefix+"!")); err == nil {
		t.Fatal("DecryptPrivate accepted malformed base64")
	}
	if _, err := DecryptPrivate(secret, []byte(privateFilePrefix+"AA==")); err == nil {
		t.Fatal("DecryptPrivate accepted a truncated payload")
	}
	encodedPayload := strings.TrimPrefix(strings.TrimSpace(string(ciphertext)), privateFilePrefix)
	decodedPayload, err := base64.StdEncoding.DecodeString(encodedPayload)
	if err != nil {
		t.Fatalf("decode encrypted payload for tamper check: %v", err)
	}
	decodedPayload[len(decodedPayload)-1] ^= 1
	tampered := []byte(privateFilePrefix + base64.StdEncoding.EncodeToString(decodedPayload))
	if _, err := DecryptPrivate(secret, tampered); err == nil {
		t.Fatal("DecryptPrivate accepted modified ciphertext")
	}
	if _, err := DecryptPrivate(bytes.Repeat([]byte{0x24}, 32), ciphertext); err == nil {
		t.Fatal("DecryptPrivate accepted a different installation secret")
	}

	privatePath := filepath.Join(t.TempDir(), "security", "pgp.priv.asc.enc")
	if err := WritePrivate(privatePath, secret, plaintext); err != nil {
		t.Fatalf("WritePrivate() error = %v", err)
	}
	info, err := os.Stat(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %04o, want 0600", info.Mode().Perm())
	}
	stored, err := ReadPrivate(privatePath, secret)
	if err != nil || !bytes.Equal(stored, plaintext) {
		t.Fatalf("ReadPrivate() = (%q, %v), want original plaintext", stored, err)
	}
	if _, err := ReadPrivate(filepath.Join(t.TempDir(), "missing"), secret); err == nil {
		t.Fatal("ReadPrivate accepted a missing private key file")
	}
}
