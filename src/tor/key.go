package tor

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/cretz/bine/control"
)

// loadOrGenerateKey loads a persisted v3 onion key from Tor's native
// hs_ed25519_secret_key format, or returns nil if Tor has not generated one.
// The previous project format is accepted so existing installations migrate
// without changing their onion identity.
func loadOrGenerateKey(path string) (control.Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read onion key %s: %w", path, err)
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, string(control.KeyTypeED25519V3)+":") {
		key, err := control.KeyFromString(raw)
		if err != nil {
			return nil, fmt.Errorf("parse legacy onion key %s: %w", path, err)
		}
		return key, nil
	}
	if len(data) != 96 || !bytes.HasPrefix(data[:32], []byte(nativeSecretKeyHeader)) {
		return nil, fmt.Errorf("parse onion key %s: invalid Tor native key format", path)
	}
	key, err := control.ED25519KeyFromBlob(base64.StdEncoding.EncodeToString(data[32:]))
	if err != nil {
		return nil, fmt.Errorf("parse onion key %s: %w", path, err)
	}
	return key, nil
}

const nativeSecretKeyHeader = "== ed25519v1-secret: type0 =="

// saveKey persists a key in Tor's native format with 0600 permissions, so
// Tor's torrc-managed hidden service reuses the same v3 .onion address.
func saveKey(path string, key control.Key) error {
	blob, err := base64.StdEncoding.DecodeString(key.Blob())
	if err != nil {
		return fmt.Errorf("encode Tor native onion key: %w", err)
	}
	if len(blob) != 64 {
		return fmt.Errorf("encode Tor native onion key: got %d private-key bytes, want 64", len(blob))
	}
	content := make([]byte, 32+len(blob))
	copy(content[:32], nativeSecretKeyHeader)
	copy(content[32:], blob)
	return ensureTorFile(path, content)
}

// migrateLegacyKey converts the project's prior bine control-key format into
// Tor's native file format before Tor reads the HiddenServiceDir.
func migrateLegacyKey(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	raw := strings.TrimSpace(string(data))
	if !strings.HasPrefix(raw, string(control.KeyTypeED25519V3)+":") {
		return nil
	}
	key, err := control.KeyFromString(raw)
	if err != nil {
		return err
	}
	return saveKey(path, key)
}

// ensureTorFile writes content to path unconditionally with 0600
// permissions and current-user ownership (Windows chown skipped), used
// for files whose content must always reflect the latest value (unlike
// ensureFile, which preserves an existing file).
func ensureTorFile(path string, content []byte) error {
	return writeFile(path, content)
}
