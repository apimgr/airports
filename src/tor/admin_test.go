package tor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cretz/bine/control"
	tored25519 "github.com/cretz/bine/torutil/ed25519"
)

func TestStatusReportsDisabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	st := m.Status()
	if st.Enabled {
		t.Error("Status().Enabled = true, want false")
	}
	if st.Running {
		t.Error("Status().Running = true, want false when Tor was never started")
	}
	if st.OnionAddress != "" {
		t.Errorf("Status().OnionAddress = %q, want empty when never started", st.OnionAddress)
	}
}

func TestValidateNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.Validate(); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Validate() error = %v, want ErrNotEnabled", err)
	}
}

func TestValidateEnabledNotRunningNoKeyFile(t *testing.T) {
	yes := true
	m := NewManager("airports-test-novanity", 8080, Config{Enabled: &yes})

	if err := m.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil when no key file exists yet and Tor is not running", err)
	}
}

func TestRestartDisabledIsInert(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.Restart(context.Background()); err != nil {
		t.Errorf("Restart() error = %v, want nil (inert no-op when disabled)", err)
	}
}

func TestRegenerateNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.Regenerate(context.Background()); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Regenerate() error = %v, want ErrNotEnabled", err)
	}
}

func TestStartVanitySearchInvalidPrefix(t *testing.T) {
	yes := true
	m := NewManager("airports-test", 8080, Config{Enabled: &yes})

	for _, bad := range []string{"", "TOO-UPPER", "has space", "invalid1", strings.Repeat("a", 17)} {
		if err := m.StartVanitySearch(bad); err == nil {
			t.Errorf("StartVanitySearch(%q) error = nil, want a validation error", bad)
		}
	}
}

func TestStartVanitySearchNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.StartVanitySearch("a"); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("StartVanitySearch() error = %v, want ErrNotEnabled", err)
	}
}

func TestVanitySearchFindsSingleCharPrefixAndStatusReports(t *testing.T) {
	yes := true
	m := NewManager("airports-test", 8080, Config{Enabled: &yes})

	if _, ok := m.VanityStatus(); ok {
		t.Fatal("VanityStatus() ok = true before any search was started")
	}

	if err := m.StartVanitySearch("a"); err != nil {
		t.Fatalf("StartVanitySearch() error = %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	var res VanityResult
	for time.Now().Before(deadline) {
		var ok bool
		res, ok = m.VanityStatus()
		if !ok {
			t.Fatal("VanityStatus() ok = false after a search was started")
		}
		if res.Found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !res.Found {
		t.Fatal("vanity search for a single-character prefix did not find a match within 10s")
	}
	if res.Running {
		t.Error("VanityResult.Running = true after a match was found")
	}
	if !strings.HasSuffix(res.Onion, ".onion") {
		t.Errorf("VanityResult.Onion = %q, want a value ending in .onion", res.Onion)
	}
	if !strings.HasPrefix(res.Onion, "a") {
		t.Errorf("VanityResult.Onion = %q, want it to start with the requested prefix %q", res.Onion, "a")
	}
}

func TestApplyVanityNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.ApplyVanity(context.Background()); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ApplyVanity() error = %v, want ErrNotEnabled", err)
	}
}

func TestApplyVanityNoResultYet(t *testing.T) {
	yes := true
	m := NewManager("airports-test-apply-novanity", 8080, Config{Enabled: &yes})

	if err := m.ApplyVanity(context.Background()); err == nil {
		t.Error("ApplyVanity() error = nil, want an error when no vanity search has completed")
	}
}

func TestApplyVanityPersistsWinningKey(t *testing.T) {
	yes := true
	m := NewManager("airports-test-apply", 8080, Config{Enabled: &yes})
	t.Cleanup(func() { _ = os.Remove(m.dirs.KeyPath) })

	if err := m.StartVanitySearch("a"); err != nil {
		t.Fatalf("StartVanitySearch() error = %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if res, _ := m.VanityStatus(); res.Found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// ApplyVanity will attempt Start() afterward; with no real tor binary
	// configured that leaves Tor disabled and Start() returns nil per its
	// documented inert behavior, so the only thing under test here is that
	// the winning key was actually persisted to disk first.
	if err := m.ApplyVanity(context.Background()); err != nil {
		t.Fatalf("ApplyVanity() error = %v", err)
	}

	if _, err := os.Stat(m.dirs.KeyPath); err != nil {
		t.Errorf("expected key file at %s after ApplyVanity, stat error = %v", m.dirs.KeyPath, err)
	}
}

func TestImportKeysNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.ImportKeys(context.Background(), []byte("garbage")); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ImportKeys() error = %v, want ErrNotEnabled", err)
	}
}

func TestImportKeysOwnFormatRoundTrip(t *testing.T) {
	yes := true
	m := NewManager("airports-test-import-own", 8080, Config{Enabled: &yes})
	t.Cleanup(func() { _ = os.Remove(m.dirs.KeyPath) })

	kp, err := tored25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	blob := fmt.Sprintf("%s:%s", control.KeyTypeED25519V3, base64.StdEncoding.EncodeToString(kp.PrivateKey()))

	if err := m.ImportKeys(context.Background(), []byte(blob)); err != nil {
		t.Fatalf("ImportKeys() error = %v", err)
	}

	saved, err := os.ReadFile(m.dirs.KeyPath)
	if err != nil {
		t.Fatalf("read persisted key: %v", err)
	}
	if strings.TrimSpace(string(saved)) != blob {
		t.Errorf("persisted key = %q, want %q", strings.TrimSpace(string(saved)), blob)
	}
}

func TestParseImportedKeyNativeFormatRoundTrip(t *testing.T) {
	kp, err := tored25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	native := make([]byte, 32+64)
	copy(native[32:], kp.PrivateKey())

	key, err := parseImportedKey(native)
	if err != nil {
		t.Fatalf("parseImportedKey() error = %v", err)
	}
	if key.Type() != control.KeyTypeED25519V3 {
		t.Errorf("parsed key Type() = %v, want %v", key.Type(), control.KeyTypeED25519V3)
	}
	wantBlob := base64.StdEncoding.EncodeToString(kp.PrivateKey())
	if key.Blob() != wantBlob {
		t.Errorf("parsed key Blob() = %q, want %q", key.Blob(), wantBlob)
	}
}

func TestParseImportedKeyGarbageRejected(t *testing.T) {
	if _, err := parseImportedKey([]byte("not a valid key at all")); err == nil {
		t.Error("parseImportedKey() error = nil, want an error for unrecognized garbage input")
	}
}
