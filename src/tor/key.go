package tor

import (
	"fmt"
	"os"
	"strings"

	"github.com/cretz/bine/control"
)

// loadOrGenerateKey loads a previously persisted ed25519-v3 onion key from
// path, or returns nil (signaling AddOnion should generate a fresh key)
// if no key file exists yet. The persisted format is "type:base64blob",
// which round-trips through bine's real control.Key API
// (key.Type()+":"+key.Blob() to save, control.KeyFromString to reload) —
// simpler and more accurate to the real library than raw ed25519 byte
// handling, since control.Key.Blob() already returns a base64 string.
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

	key, err := control.KeyFromString(raw)
	if err != nil {
		return nil, fmt.Errorf("parse onion key %s: %w", path, err)
	}
	return key, nil
}

// saveKey persists key to path in "type:base64blob" form with 0600
// permissions, so the same v3 .onion address is reused across restarts.
func saveKey(path string, key control.Key) error {
	content := fmt.Sprintf("%s:%s", key.Type(), key.Blob())
	return ensureTorFile(path, []byte(content))
}

// ensureTorFile writes content to path unconditionally with 0600
// permissions and current-user ownership (Windows chown skipped), used
// for files whose content must always reflect the latest value (unlike
// ensureFile, which preserves an existing file).
func ensureTorFile(path string, content []byte) error {
	return writeFile(path, content)
}
