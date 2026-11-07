package tor

import (
	"bytes"
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
	m := managerForTest(t, "airports-test", 8080, false)

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
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.Validate(); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Validate() error = %v, want ErrNotEnabled", err)
	}
}

func TestValidateEnabledNotRunningNoKeyFile(t *testing.T) {
	m := managerForTest(t, "airports-test-novanity", 8080, true)

	if err := m.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil when no key file exists yet and Tor is not running", err)
	}
}

func TestRestartDisabledIsInert(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.Restart(context.Background()); err != nil {
		t.Errorf("Restart() error = %v, want nil (inert no-op when disabled)", err)
	}
}

func TestRegenerateNotEnabled(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.Regenerate(context.Background()); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("Regenerate() error = %v, want ErrNotEnabled", err)
	}
}

func TestStartVanitySearchInvalidPrefix(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, true)

	for _, bad := range []string{"", "TOO-UPPER", "has space", "invalid1", strings.Repeat("a", 17)} {
		if err := m.StartVanitySearch(bad); err == nil {
			t.Errorf("StartVanitySearch(%q) error = nil, want a validation error", bad)
		}
	}
}

func TestStartVanitySearchNotEnabled(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.StartVanitySearch("a"); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("StartVanitySearch() error = %v, want ErrNotEnabled", err)
	}
}

func TestVanitySearchFindsSingleCharPrefixAndStatusReports(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, true)

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
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.ApplyVanity(context.Background()); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ApplyVanity() error = %v, want ErrNotEnabled", err)
	}
}

func TestApplyVanityNoResultYet(t *testing.T) {
	m := managerForTest(t, "airports-test-apply-novanity", 8080, true)

	if err := m.ApplyVanity(context.Background()); err == nil {
		t.Error("ApplyVanity() error = nil, want an error when no vanity search has completed")
	}
}

func TestApplyVanityPersistsWinningKey(t *testing.T) {
	m := managerForTest(t, "airports-test-apply", 8080, true)
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

	// ApplyVanity attempts Start() afterward. The manager's configured binary
	// path does not exist, so startup returns harmlessly after persisting the
	// winning key; this test only checks that persistence step.
	if err := m.ApplyVanity(context.Background()); err != nil {
		t.Fatalf("ApplyVanity() error = %v", err)
	}

	if _, err := os.Stat(m.dirs.KeyPath); err != nil {
		t.Errorf("expected key file at %s after ApplyVanity, stat error = %v", m.dirs.KeyPath, err)
	}
}

func TestImportKeysNotEnabled(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.ImportKeys(context.Background(), []byte("garbage")); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ImportKeys() error = %v, want ErrNotEnabled", err)
	}
}

func TestImportKeysLegacyFormatWritesTorNativeKey(t *testing.T) {
	m := managerForTest(t, "airports-test-import-own", 8080, true)
	t.Cleanup(func() { _ = os.RemoveAll(m.dirs.SiteDir) })

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
	if len(saved) != 96 || !bytes.HasPrefix(saved[:32], []byte(nativeSecretKeyHeader)) {
		t.Fatalf("persisted key does not use Tor native format: %d bytes", len(saved))
	}
	parsed, err := loadOrGenerateKey(m.dirs.KeyPath)
	if err != nil {
		t.Fatalf("load persisted key: %v", err)
	}
	if parsed == nil || parsed.Blob() != base64.StdEncoding.EncodeToString(kp.PrivateKey()) {
		t.Error("persisted key does not match imported onion identity")
	}
}

func TestParseImportedKeyNativeFormatRoundTrip(t *testing.T) {
	kp, err := tored25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	native := make([]byte, 32+64)
	copy(native[:32], nativeSecretKeyHeader)
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
