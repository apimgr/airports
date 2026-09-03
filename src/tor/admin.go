package tor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cretz/bine/control"
	tored25519 "github.com/cretz/bine/torutil/ed25519"

	"github.com/cretz/bine/torutil"
)

// vanityPrefixPattern restricts a requested vanity prefix to the onion v3
// base32 alphabet (RFC 4648 without padding, lowercase per bine's own
// encoding), 1-16 characters, per AI.md PART 31 CLI control channel.
var vanityPrefixPattern = regexp.MustCompile(`^[a-z2-7]{1,16}$`)

// Status is a point-in-time snapshot of Tor's current state, backing the
// CLI-to-server internal control channel's `GET /server/tor/status`
// endpoint per AI.md PART 31.
type Status struct {
	Enabled      bool   `json:"enabled"`
	Running      bool   `json:"running"`
	OnionAddress string `json:"onion_address,omitempty"`
	VirtualPort  int    `json:"virtual_port,omitempty"`
	ServerPort   int    `json:"server_port,omitempty"`
}

// Status returns a snapshot of Tor's current state for the CLI
// `tor status` subcommand.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Enabled:     m.enabled,
		Running:     m.t != nil,
		VirtualPort: m.cfg.VirtualPort,
		ServerPort:  m.serverPort,
	}
	if m.serviceID != "" {
		st.OnionAddress = m.serviceID + ".onion"
	}
	return st
}

// Validate confirms the persisted onion key (if any) parses correctly and,
// when Tor is running, that the control connection still responds. It
// never mutates state.
func (m *Manager) Validate() error {
	m.mu.Lock()
	enabled := m.enabled
	running := m.t != nil
	keyPath := m.dirs.KeyPath
	m.mu.Unlock()

	if !enabled {
		return ErrNotEnabled
	}

	if _, err := os.Stat(keyPath); err == nil {
		if _, err := loadOrGenerateKey(keyPath); err != nil {
			return fmt.Errorf("tor: validate onion key: %w", err)
		}
	}

	if running {
		return m.HealthCheck()
	}
	return nil
}

// Restart stops the dedicated Tor process (if running) and starts it again,
// reusing the same persisted onion key so the .onion address stays stable.
func (m *Manager) Restart(ctx context.Context) error {
	if err := m.Stop(); err != nil {
		return fmt.Errorf("tor: restart: stop: %w", err)
	}
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("tor: restart: start: %w", err)
	}
	return nil
}

// Regenerate discards the current persisted onion key and starts a brand
// new v3 identity (a new .onion address), restarting the hidden service so
// the new key takes effect immediately.
func (m *Manager) Regenerate(ctx context.Context) error {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return ErrNotEnabled
	}
	keyPath := m.dirs.KeyPath
	running := m.t != nil
	oldServiceID := m.serviceID
	t := m.t
	m.mu.Unlock()

	if running && t != nil {
		if err := t.Control.DelOnion(oldServiceID); err != nil {
			return fmt.Errorf("tor: regenerate: delete old onion: %w", err)
		}
	}

	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("tor: regenerate: remove old key: %w", err)
	}

	if running {
		return m.Restart(ctx)
	}
	return m.Start(ctx)
}

// vanityJob tracks an in-progress or completed vanity onion-address search
// started via StartVanitySearch. All fields are guarded by the owning
// Manager's mu.
type vanityJob struct {
	prefix    string
	running   bool
	found     bool
	onionID   string // 56-char v3 service ID, no ".onion" suffix
	keyBlob   string // "ED25519-V3:<base64>", ready for control.KeyFromString
	startedAt time.Time
	cancel    context.CancelFunc
}

// VanityResult is the snapshot returned by VanityStatus, backing the CLI
// `tor vanity start` progress polling and the `tor vanity apply`
// precondition check.
type VanityResult struct {
	Prefix    string    `json:"prefix"`
	Running   bool      `json:"running"`
	Found     bool      `json:"found"`
	Onion     string    `json:"onion,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// StartVanitySearch begins a background brute-force search for a v3 onion
// address beginning with prefix (base32 alphabet a-z2-7, 1-16 characters),
// using all available CPU cores. Only one search runs at a time; starting a
// new one cancels any prior in-progress search. Candidates are generated
// entirely locally (no Tor control-port round-trip per attempt) using the
// same ed25519 keypair and service-ID derivation Tor itself uses.
func (m *Manager) StartVanitySearch(prefix string) error {
	if !vanityPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("tor: vanity prefix must be 1-16 lowercase base32 characters (a-z, 2-7)")
	}

	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return ErrNotEnabled
	}
	if m.vanity != nil && m.vanity.cancel != nil {
		m.vanity.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &vanityJob{prefix: prefix, running: true, startedAt: time.Now(), cancel: cancel}
	m.vanity = job
	m.mu.Unlock()

	go m.runVanitySearch(ctx, job)
	return nil
}

// runVanitySearch drives the worker pool for job until a match is found or
// ctx is canceled (superseded by a newer search). Exactly one winning
// candidate is recorded on job.
func (m *Manager) runVanitySearch(ctx context.Context, job *vanityJob) {
	var found int32
	var wg sync.WaitGroup

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if atomic.LoadInt32(&found) != 0 {
					return
				}

				kp, err := tored25519.GenerateKey(rand.Reader)
				if err != nil {
					continue
				}
				id := torutil.OnionServiceIDFromV3PublicKey(kp.PublicKey())
				if !strings.HasPrefix(id, job.prefix) {
					continue
				}
				if !atomic.CompareAndSwapInt32(&found, 0, 1) {
					return
				}

				keyBlob := fmt.Sprintf("%s:%s", control.KeyTypeED25519V3, base64.StdEncoding.EncodeToString(kp.PrivateKey()))

				m.mu.Lock()
				job.running = false
				job.found = true
				job.onionID = id
				job.keyBlob = keyBlob
				m.mu.Unlock()
				return
			}
		}()
	}

	wg.Wait()

	m.mu.Lock()
	job.running = false
	m.mu.Unlock()
}

// VanityStatus reports the state of the most recently started vanity
// search, if any. The bool return is false if no search has ever been
// started.
func (m *Manager) VanityStatus() (VanityResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.vanity == nil {
		return VanityResult{}, false
	}
	res := VanityResult{
		Prefix:    m.vanity.prefix,
		Running:   m.vanity.running,
		Found:     m.vanity.found,
		StartedAt: m.vanity.startedAt,
	}
	if m.vanity.found {
		res.Onion = m.vanity.onionID + ".onion"
	}
	return res, true
}

// ApplyVanity installs the winning key from the most recently completed
// vanity search as the server's persisted onion identity and restarts the
// hidden service so the new vanity .onion address takes effect. Returns an
// error if no completed vanity search result is available to apply.
func (m *Manager) ApplyVanity(ctx context.Context) error {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return ErrNotEnabled
	}
	if m.vanity == nil || !m.vanity.found {
		m.mu.Unlock()
		return fmt.Errorf("tor: no completed vanity search result to apply")
	}
	keyBlob := m.vanity.keyBlob
	keyPath := m.dirs.KeyPath
	running := m.t != nil
	oldServiceID := m.serviceID
	t := m.t
	m.mu.Unlock()

	key, err := control.KeyFromString(keyBlob)
	if err != nil {
		return fmt.Errorf("tor: apply vanity: parse generated key: %w", err)
	}

	if running && t != nil {
		if err := t.Control.DelOnion(oldServiceID); err != nil {
			return fmt.Errorf("tor: apply vanity: delete old onion: %w", err)
		}
	}

	if err := saveKey(keyPath, key); err != nil {
		return fmt.Errorf("tor: apply vanity: save key: %w", err)
	}

	m.mu.Lock()
	m.vanity = nil
	m.mu.Unlock()

	if running {
		return m.Restart(ctx)
	}
	return m.Start(ctx)
}

// parseImportedKey parses raw key material supplied to ImportKeys. It first
// tries this project's own persisted "type:base64blob" format (the same
// format saveKey writes and control.KeyFromString reads); if that fails, it
// falls back to detecting Tor's native 96-byte hs_ed25519_secret_key file
// format (a 32-byte magic header followed by the 64-byte expanded private
// key), so a key exported from a real Tor hidden service can be migrated in
// directly.
func parseImportedKey(data []byte) (control.Key, error) {
	if key, err := control.KeyFromString(strings.TrimSpace(string(data))); err == nil {
		return key, nil
	}

	const nativeHeaderLen = 32
	const nativeKeyLen = nativeHeaderLen + 64
	if len(data) == nativeKeyLen {
		return control.ED25519KeyFromBlob(base64.StdEncoding.EncodeToString(data[nativeHeaderLen:]))
	}

	return nil, fmt.Errorf("tor: unrecognized key format (expected %q or a %d-byte native hs_ed25519_secret_key file)", control.KeyTypeED25519V3, nativeKeyLen)
}

// ImportKeys installs externally supplied onion key material (either this
// project's own persisted format or a native Tor hs_ed25519_secret_key
// file) as the server's onion identity, replacing whatever is currently
// persisted, and restarts the hidden service so the imported .onion address
// takes effect.
func (m *Manager) ImportKeys(ctx context.Context, data []byte) error {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return ErrNotEnabled
	}
	keyPath := m.dirs.KeyPath
	running := m.t != nil
	oldServiceID := m.serviceID
	t := m.t
	m.mu.Unlock()

	key, err := parseImportedKey(data)
	if err != nil {
		return err
	}

	if running && t != nil {
		if err := t.Control.DelOnion(oldServiceID); err != nil {
			return fmt.Errorf("tor: import keys: delete old onion: %w", err)
		}
	}

	if err := saveKey(keyPath, key); err != nil {
		return fmt.Errorf("tor: import keys: save key: %w", err)
	}

	if running {
		return m.Restart(ctx)
	}
	return m.Start(ctx)
}
