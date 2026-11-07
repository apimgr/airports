package tor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/pires/go-proxyproto"
)

// Manager owns the full lifecycle of this server's dedicated Tor process
// and hidden service, per AI.md PART 31. It is always safe to construct
// and call Start/Stop/HealthCheck on a Manager when Tor is not installed —
// every method is inert in that case.
type Manager struct {
	mu sync.Mutex
	// starting lets health endpoints report Tor as starting without waiting
	// for the bootstrap operation, which holds mu while it waits on the network.
	starting atomic.Bool

	projectName string
	cfg         Config
	serverPort  int
	dirs        Dirs

	enabled bool

	t           *tor.Tor
	serviceID   string
	dialer      *tor.Dialer
	backend     net.Listener
	backendPort int

	// vanity tracks the most recent vanity-address search started via
	// StartVanitySearch, per AI.md PART 31 CLI control channel. nil until
	// the first search is started.
	vanity *vanityJob
}

// NewManager builds a Manager for projectName. The hidden service is forwarded
// to a dedicated PROXY-protocol listener allocated during Start. cfg is
// validated/defaulted internally; construction never starts any process or
// touches the filesystem.
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

// Enabled reports whether a Tor binary was found when this Manager was
// constructed.
func (m *Manager) Enabled() bool {
	if m.starting.Load() {
		return true
	}
	if !m.mu.TryLock() {
		// Treat a busy lifecycle operation as enabled so status surfaces it as
		// starting/degraded instead of blocking a public health request.
		return true
	}
	defer m.mu.Unlock()
	return m.enabled
}

// Start locates the tor binary, prepares directories/torrc, launches a
// dedicated Tor process, waits for bootstrap, and creates (or reuses) the
// v3 hidden service. If Tor is not enabled, Start logs an INFO line and
// returns nil — this is never a startup failure per PART 31 "Tor is
// OPTIONAL. Missing Tor is NOT an error."
func (m *Manager) Start(ctx context.Context) (retErr error) {
	m.starting.Store(true)
	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.starting.Store(false)
	defer func() {
		if retErr == nil || m.backend == nil {
			return
		}
		if err := m.backend.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			retErr = errors.Join(retErr, fmt.Errorf("tor: close backend listener after startup failure: %w", err))
		}
		m.backend = nil
		m.backendPort = 0
	}()

	if !m.enabled {
		log.Println("Tor hidden service: unavailable (tor binary not found)")
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
	if err := migrateLegacyKey(m.dirs.KeyPath); err != nil {
		return fmt.Errorf("tor: migrate onion key: %w", err)
	}
	if m.backend == nil {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("tor: bind PROXY-protocol backend: %w", err)
		}
		m.backend = &proxyproto.Listener{
			Listener: listener,
			ConnPolicy: func(opts proxyproto.ConnPolicyOptions) (proxyproto.Policy, error) {
				if !isLoopbackAddr(opts.Upstream) {
					return proxyproto.REJECT, fmt.Errorf("tor: PROXY-protocol peer is not loopback")
				}
				return proxyproto.REQUIRE, nil
			},
			ValidateHeader:    validateTorCircuitHeader,
			ReadHeaderTimeout: 5 * time.Second,
		}
		m.backendPort = listener.Addr().(*net.TCPAddr).Port
	}

	if err := writeFile(m.dirs.TorrcPath, []byte(generateTorrc(m.cfg, m.dirs.LogFile, m.dirs.SiteDir, m.backendPort))); err != nil {
		return fmt.Errorf("tor: prepare torrc: %w", err)
	}
	log.Printf("Regenerated torrc at %s (backend port %d)", m.dirs.TorrcPath, m.backendPort)

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
		_ = t.Close()
		log.Printf("Tor: bootstrap failed: %v", bootErr)
		return fmt.Errorf("tor: bootstrap failed: %w", bootErr)
	}

	onion, err := readOnionHostname(m.dirs.SiteDir)
	if err != nil {
		_ = t.Close()
		return fmt.Errorf("tor: read hidden-service hostname: %w", err)
	}

	m.t = t
	m.serviceID = strings.TrimSuffix(strings.TrimSpace(onion), ".onion")

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

	var stopErr error
	if m.t != nil {
		log.Println("Stopping Tor process...")
		stopErr = m.t.Close()
		m.t = nil
	}
	if m.backend != nil {
		if err := m.backend.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			stopErr = errors.Join(stopErr, fmt.Errorf("tor: close backend listener: %w", err))
		}
		m.backend = nil
		m.backendPort = 0
	}
	m.serviceID = ""
	m.dialer = nil
	return stopErr
}

// BackendListener returns the dedicated loopback listener used only by Tor's
// hidden service. It requires PROXY protocol on every accepted connection.
// The caller serves the application handler on this listener and closes it
// during process shutdown.
func (m *Manager) BackendListener() net.Listener {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.backend
}

// isLoopbackAddr rejects malformed and non-loopback upstream peers before
// proxyproto parses any supplied identity metadata.
func isLoopbackAddr(addr net.Addr) bool {
	if addr == nil {
		return false
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateTorCircuitHeader(header *proxyproto.Header) error {
	if header == nil || header.Version != 1 {
		return fmt.Errorf("tor: missing or unsupported PROXY-protocol v1 header")
	}
	source, ok := header.SourceAddr.(*net.TCPAddr)
	if !ok || source.IP == nil || source.IP.To4() != nil {
		return fmt.Errorf("tor: PROXY-protocol source is not IPv6")
	}
	ip := source.IP.To16()
	if ip == nil || ip[0] != 0xfc {
		return fmt.Errorf("tor: PROXY-protocol source is not a Tor circuit ID")
	}
	return nil
}

func readOnionHostname(siteDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(siteDir, "hostname"))
	if err != nil {
		return "", err
	}
	hostname := strings.TrimSpace(string(data))
	if !strings.HasSuffix(strings.ToLower(hostname), ".onion") {
		return "", fmt.Errorf("invalid hidden-service hostname")
	}
	return hostname, nil
}

// HealthCheck verifies the dedicated Tor process's control connection is
// alive. It returns ErrNotEnabled when Tor is not configured/enabled (the
// scheduler must treat this as "skipped", not a failure), ErrNotRunning
// when Tor is enabled but Start has not succeeded yet, or the underlying
// control-connection error if the running process stopped responding.
func (m *Manager) HealthCheck() error {
	if m.starting.Load() {
		return ErrNotRunning
	}
	if !m.mu.TryLock() {
		return ErrNotRunning
	}
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

// CircuitEstablished reports whether Tor currently reports an established
// circuit through its control connection. It returns false when disabled,
// stopped, or when the control query fails.
func (m *Manager) CircuitEstablished() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled || m.t == nil {
		return false
	}
	values, err := m.t.Control.GetInfo("status/circuit-established")
	if err != nil {
		return false
	}
	for _, value := range values {
		if value != nil && value.Key == "status/circuit-established" {
			return value.Val == "1"
		}
	}
	return false
}

// OnionAddress returns the current full .onion address, or "" if Tor is
// disabled or not currently running.
func (m *Manager) OnionAddress() string {
	if m.starting.Load() {
		return ""
	}
	if !m.mu.TryLock() {
		return ""
	}
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
