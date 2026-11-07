package tor

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cretz/bine/control"
	tored25519 "github.com/cretz/bine/torutil/ed25519"
)

func TestLoadOrGenerateKeyMissingFileReturnsNil(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "site", "hs_ed25519_secret_key")

	key, err := loadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("loadOrGenerateKey() error = %v", err)
	}
	if key != nil {
		t.Errorf("key = %v, want nil when no key file exists yet", key)
	}
}

func TestSaveAndReloadKeyRoundTrips(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "site", "hs_ed25519_secret_key")

	generated := newTestED25519Key(t)

	if err := saveKey(path, generated); err != nil {
		t.Fatalf("saveKey() error = %v", err)
	}

	reloaded, err := loadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("loadOrGenerateKey() error = %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded key is nil, want a persisted key to be loaded back")
	}

	if reloaded.Type() != generated.Type() {
		t.Errorf("reloaded.Type() = %v, want %v", reloaded.Type(), generated.Type())
	}
	if reloaded.Blob() != generated.Blob() {
		t.Errorf("reloaded.Blob() = %q, want %q (persisted key must reproduce the same address)", reloaded.Blob(), generated.Blob())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Tor native key: %v", err)
	}
	if len(data) != 96 || !bytes.HasPrefix(data[:32], []byte(nativeSecretKeyHeader)) {
		t.Fatalf("persisted key format has %d bytes and header %q, want Tor's 96-byte native format", len(data), data[:min(len(data), 32)])
	}
}

func TestMigrateLegacyBineKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site", "hs_ed25519_secret_key")
	want := newTestED25519Key(t)
	legacy := []byte(string(want.Type()) + ":" + want.Blob())
	if err := ensureTorFile(path, legacy); err != nil {
		t.Fatalf("write legacy key: %v", err)
	}
	if err := migrateLegacyKey(path); err != nil {
		t.Fatalf("migrateLegacyKey() error = %v", err)
	}
	got, err := loadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("load migrated key: %v", err)
	}
	if got == nil || got.Blob() != want.Blob() {
		t.Fatalf("migrated key = %v, want same key blob", got)
	}
}

func TestSaveKeyEnforcesRestrictedPerms(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "site", "hs_ed25519_secret_key")

	key := newTestED25519Key(t)
	if err := saveKey(path, key); err != nil {
		t.Fatalf("saveKey() error = %v", err)
	}

	_, err := loadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("loadOrGenerateKey() after save error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved Tor key: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("saved key permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestLoadOrGenerateKeyRejectsCorruptFile(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "site", "hs_ed25519_secret_key")

	if err := ensureTorFile(path, []byte("not a valid key string")); err != nil {
		t.Fatalf("ensureTorFile() setup error = %v", err)
	}

	if _, err := loadOrGenerateKey(path); err == nil {
		t.Error("loadOrGenerateKey() error = nil, want an error for corrupt key content")
	}
}

func newTestED25519Key(t *testing.T) control.Key {
	t.Helper()
	keyPair, err := tored25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate Ed25519 test key: %v", err)
	}
	key, err := control.ED25519KeyFromBlob(base64.StdEncoding.EncodeToString(keyPair.PrivateKey()))
	if err != nil {
		t.Fatalf("create Bine Ed25519 test key: %v", err)
	}
	return key
}
