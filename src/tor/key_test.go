package tor

import (
	"path/filepath"
	"testing"

	"github.com/cretz/bine/control"
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

	generated := control.GenKey(control.KeyAlgoED25519V3)

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
}

func TestSaveKeyEnforcesRestrictedPerms(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "site", "hs_ed25519_secret_key")

	key := control.GenKey(control.KeyAlgoED25519V3)
	if err := saveKey(path, key); err != nil {
		t.Fatalf("saveKey() error = %v", err)
	}

	_, err := loadOrGenerateKey(path)
	if err != nil {
		t.Fatalf("loadOrGenerateKey() after save error = %v", err)
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
