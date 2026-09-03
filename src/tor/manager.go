package tor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cretz/bine/control"
	"github.com/cretz/bine/tor"
)

// Manager owns the full lifecycle of this server's dedicated Tor process
// and hidden service, per AI.md PART 31. It is always safe to construct
// and call Start/Stop/HealthCheck on a Manager even when Tor is not
// enabled or not installed — every method is inert in that case.
type Manager struct {
	mu sync.Mutex

	projectName string
	cfg         Config
	serverPort  int
	dirs        Dirs

	enabled bool

	t         *tor.Tor
	serviceID string
	dialer    *tor.Dialer

	// vanity tracks the most recent vanity-address search started via
	// StartVanitySearch, per AI.md PART 31 CLI control channel. nil until
	// the first search is started.
	vanity *vanityJob
}

// NewManager builds a Manager for projectName, forwarding the hidden
// service's virtual port to 127.0.0.1:serverPort (the server's existing
// HTTP listener). cfg is validated/defaulted internally; construction
// never starts any process or touches the filesystem.
func NewManager(projectName string, serverPort int, cfg Config) *Manager {
	cfg = applyDefaults(cfg)
	return &Manager{
		projectName: projectName,
		cfg:         cfg,
		serverPort:  serverPort,
		dirs:        resolveDirs(projectName),
		enabled:     resolveEnabled(cfg, findTorBinary),
	}
}

// Enabled reports whether this Manager resolved to an enabled state at
// construction time (explicit config true, or auto-detected tor binary).
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// Start locates the tor binary, prepares directories/torrc, launches a
// dedicated Tor process, waits for bootstrap, and creates (or reuses) the
// v3 hidden service. If Tor is not enabled, Start logs an INFO line and
// returns nil — this is never a startup failure per PART 31 "Tor is
// OPTIONAL. Missing Tor is NOT an error."
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		log.Println("Tor hidden service: disabled (tor not installed or not enabled)")
		return nil
	}

	binary, err := findTorBinary(m.cfg.Binary)
	if err != nil {
		log.Printf("Tor binary not found, hidden service disabled: %v", err)
		m.enabled = false
		return nil
	}

	if err := ensureDirs(m.dirs); err != nil {
		return fmt.Errorf("tor: prepare directories: %w", err)
	}

	created, err := ensureFile(m.dirs.TorrcPath, []byte(generateTorrc(m.cfg, m.dirs.LogFile)))
	if err != nil {
		return fmt.Errorf("tor: prepare torrc: %w", err)
	}
	if created {
		log.Printf("Created new torrc at %s", m.dirs.TorrcPath)
	}

	startConf := &tor.StartConf{
		ExePath:         binary,
		TorrcFile:       m.dirs.TorrcPath,
		DataDir:         m.dirs.DataDir,
		NoAutoSocksPort: true,
	}

	log.Println("Starting Tor hidden service...")
	t, err := tor.Start(ctx, startConf)
	if err != nil {
		return fmt.Errorf("tor: start dedicated process: %w", err)
	}

	bootstrapTimeout := time.Duration(m.cfg.BootstrapTimeout) * time.Second
	bootCtx, cancel := context.WithTimeout(ctx, bootstrapTimeout)
	defer cancel()

	// Per AI.md PART 31 "Console Output Rules": bootstrap is silent for the
	// first 30 seconds; if it is still in progress after that, show a single
	// "connecting..." status line so the operator knows the server has not
	// hung.
	bootDone := make(chan struct{})
	go func() {
		select {
		case <-bootDone:
		case <-time.After(30 * time.Second):
			log.Println("Tor: connecting...")
		}
	}()

	bootErr := t.EnableNetwork(bootCtx, true)
	close(bootDone)
	if bootErr != nil {
		t.Close()
		log.Printf("Tor: bootstrap failed: %v", bootErr)
		return fmt.Errorf("tor: bootstrap failed: %w", bootErr)
	}

	key, err := loadOrGenerateKey(m.dirs.KeyPath)
	if err != nil {
		t.Close()
		return fmt.Errorf("tor: load onion key: %w", err)
	}
	if key == nil {
		key = control.GenKey(control.KeyAlgoED25519V3)
	}

	req := &control.AddOnionRequest{
		Key: key,
		Ports: []*control.KeyVal{
			control.NewKeyVal(fmt.Sprintf("%d", m.cfg.VirtualPort), fmt.Sprintf("127.0.0.1:%d", m.serverPort)),
		},
	}

	resp, err := t.Control.AddOnion(req)
	if err != nil {
		t.Close()
		return fmt.Errorf("tor: create hidden service: %w", err)
	}

	if resp.Key != nil {
		if err := saveKey(m.dirs.KeyPath, resp.Key); err != nil {
			log.Printf("Warning: failed to save onion key: %v", err)
		}
	} else if err := saveKey(m.dirs.KeyPath, key); err != nil {
		log.Printf("Warning: failed to save onion key: %v", err)
	}

	m.t = t
	m.serviceID = resp.ServiceID

	if m.cfg.UseNetwork {
		dialer, err := t.Dialer(ctx, nil)
		if err != nil {
			log.Printf("Warning: failed to create Tor outbound dialer: %v", err)
		} else {
			m.dialer = dialer
			log.Println("Tor outbound connections enabled")
		}
	}

	log.Printf("Tor: %s.onion", m.serviceID)
	return nil
}

// Stop gracefully terminates the dedicated Tor process if one is running.
// It is always safe to call, including when Tor was never started.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.t == nil {
		return nil
	}

	log.Println("Stopping Tor process...")
	err := m.t.Close()
	m.t = nil
	m.serviceID = ""
	m.dialer = nil
	return err
}

// HealthCheck verifies the dedicated Tor process's control connection is
// alive. It returns ErrNotEnabled when Tor is not configured/enabled (the
// scheduler must treat this as "skipped", not a failure), ErrNotRunning
// when Tor is enabled but Start has not succeeded yet, or the underlying
// control-connection error if the running process stopped responding.
func (m *Manager) HealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return ErrNotEnabled
	}
	if m.t == nil {
		return ErrNotRunning
	}

	if _, err := m.t.Control.GetInfo("version"); err != nil {
		return fmt.Errorf("tor: control connection unhealthy: %w", err)
	}
	return nil
}

// OnionAddress returns the current full .onion address, or "" if Tor is
// disabled or not currently running.
func (m *Manager) OnionAddress() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.serviceID == "" {
		return ""
	}
	return m.serviceID + ".onion"
}

// GetHTTPClient returns an HTTP client for outbound requests. When useTor
// is true and an outbound Tor dialer is available, requests are routed
// through the dedicated Tor process's SOCKS proxy; otherwise a normal
// direct-connection client is returned.
func (m *Manager) GetHTTPClient(useTor bool) *http.Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !useTor || m.dialer == nil {
		return &http.Client{Timeout: 30 * time.Second}
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: m.dialer.DialContext,
		},
	}
}
