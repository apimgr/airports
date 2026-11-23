package i2p

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// I2PProvider identifies the provider that owns the eepsite tunnel.
type I2PProvider string

const (
	I2PProviderNone I2PProvider = "none"
	I2PProviderI2PD I2PProvider = "i2pd"
	I2PProviderSAM  I2PProvider = "sam"
)

// I2PProviderService tracks the active provider process or SAM session.
type I2PProviderService struct {
	provider    I2PProvider
	address     string
	backendPort int
	samAddress  string
	forwardConn net.Conn
	sessionLost <-chan struct{}
	forwardLost <-chan struct{}
	probeFails  int
	i2pd        *exec.Cmd
	i2pdDone    chan struct{}
	i2pdErr     error
	samConn     net.Conn
	pidPath     string
	mu          sync.Mutex
}

// I2PManager owns an opt-in I2P provider and its dedicated loopback backend.
type I2PManager struct {
	mu            sync.RWMutex
	cfg           I2PConfig
	dirs          I2PDirs
	service       *I2PProviderService
	listener      net.Listener
	http          *http.Server
	handler       http.Handler
	ctx           context.Context
	cancel        context.CancelFunc
	startupCtx    context.Context
	startupCancel context.CancelFunc
	retryAt       time.Time
	retryDelay    time.Duration
	lastErr       error
	running       bool
	state         string
	stateView     atomic.Value
	enabledView   atomic.Bool
	runningView   atomic.Bool
	addressView   atomic.Value
	providerView  atomic.Value
	provider      I2PProvider
	address       string
	port          int
}

// NewManager constructs an inactive manager without touching disk or network.
func NewI2PManager(cfg I2PConfig, dirs I2PDirs) *I2PManager {
	state := "disabled"
	if cfg.Enabled {
		state = "starting"
	}
	m := &I2PManager{cfg: cfg.WithDefaults(), dirs: dirs, provider: I2PProviderNone, state: state}
	m.stateView.Store(state)
	m.enabledView.Store(m.cfg.Enabled)
	m.addressView.Store("")
	m.providerView.Store(I2PProviderNone)
	return m
}

func (m *I2PManager) setState(state string) {
	m.state = state
	m.stateView.Store(state)
}

// StartI2PService launches the provider only when explicitly enabled.
func (m *I2PManager) StartI2PService(parent context.Context, handler http.Handler) error {
	m.mu.Lock()
	if !m.cfg.Enabled {
		m.setState("disabled")
		m.mu.Unlock()
		return nil
	}
	if m.running || m.ctx != nil || m.startupCtx != nil {
		m.mu.Unlock()
		return nil
	}
	if handler == nil {
		m.mu.Unlock()
		return fmt.Errorf("I2P backend handler is required")
	}
	if err := m.cfg.ValidateI2PConfig(); err != nil {
		m.setState("error")
		m.mu.Unlock()
		return err
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	m.startupCtx, m.startupCancel, m.handler = ctx, cancel, handler
	m.setState("starting")
	m.mu.Unlock()

	go m.startWithRetry(ctx, handler)
	return nil
}

func (m *I2PManager) startWithRetry(ctx context.Context, handler http.Handler) {
	var delay time.Duration
	for {
		m.mu.Lock()
		if ctx.Err() != nil || m.startupCtx != ctx {
			m.mu.Unlock()
			return
		}
		err := m.startLocked(ctx, handler)
		if err == nil {
			m.retryAt, m.retryDelay, m.lastErr = time.Time{}, 0, nil
			m.mu.Unlock()
			return
		}
		if m.state == "no_provider" {
			m.mu.Unlock()
			return
		}
		m.lastErr = err
		m.setState("error")
		delay = nextI2PRetryDelay(err, delay)
		m.retryDelay = delay
		m.retryAt = time.Now().Add(delay)
		log.Printf("I2P eepsite startup failed; retrying in %s: %v", delay, err)
		m.mu.Unlock()

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func nextI2PRetryDelay(err error, previous time.Duration) time.Duration {
	if err != nil && strings.Contains(err.Error(), "DUPLICATED_DEST") {
		return 60 * time.Second
	}
	if previous <= 0 {
		return 5 * time.Second
	}
	delay := previous * 2
	if delay > 300*time.Second {
		return 300 * time.Second
	}
	return delay
}

func (m *I2PManager) startLocked(parent context.Context, handler http.Handler) error {
	if !m.cfg.Enabled {
		m.setState("disabled")
		return nil
	}
	if m.running {
		return nil
	}
	if handler == nil {
		return fmt.Errorf("I2P backend handler is required")
	}
	if err := m.cfg.ValidateI2PConfig(); err != nil {
		m.setState("error")
		return err
	}
	m.setState("starting")
	provider, binary, err := resolveProvider(m.cfg)
	if err != nil {
		m.setState("no_provider")
		log.Printf("I2P provider unavailable: %v", err)
		return err
	}
	m.provider = provider
	m.providerView.Store(provider)
	m.setState("building")
	if err := ensureDirs(m.dirs); err != nil {
		m.setState("error")
		return err
	}
	listener, port, err := allocateBackendListener()
	if err != nil {
		m.setState("error")
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	m.ctx, m.cancel, m.listener, m.handler, m.port = ctx, cancel, listener, handler, port
	m.http = &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func(srv *http.Server, ln net.Listener) {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("I2P backend listener stopped: %v", err)
		}
	}(m.http, listener)
	service, err := m.startProviderLocked(provider, binary, port)
	if err != nil {
		cancel()
		_ = m.http.Close()
		_ = listener.Close()
		m.resetLocked()
		m.provider = provider
		m.providerView.Store(provider)
		m.setState("error")
		return err
	}
	m.service, m.provider, m.address, m.running = service, provider, service.address, true
	m.providerView.Store(provider)
	m.addressView.Store(service.address)
	m.runningView.Store(true)
	m.setState("running")
	go m.monitor(ctx)
	return nil
}

// IsI2PEnabled reports the opt-in configuration.
func (m *I2PManager) IsI2PEnabled() bool {
	return m.enabledView.Load()
}

// GetI2PStatus returns the stable lifecycle state used by server health output.
func (m *I2PManager) GetI2PStatus() string {
	if !m.enabledView.Load() {
		return "disabled"
	}
	state, _ := m.stateView.Load().(string)
	if state == "" {
		return "starting"
	}
	return state
}

// IsI2PRunning reports whether the provider is active.
func (m *I2PManager) IsI2PRunning() bool {
	return m.runningView.Load()
}

// I2PProvider returns the running provider name or none.
func (m *I2PManager) GetI2PProvider() I2PProvider {
	provider, _ := m.providerView.Load().(I2PProvider)
	if provider == "" {
		return I2PProviderNone
	}
	return provider
}

// EepsiteAddress returns the published address when active.
func (m *I2PManager) EepsiteAddress() string {
	address, _ := m.addressView.Load().(string)
	return address
}

// BackendPort returns the provider's dedicated loopback target port.
func (m *I2PManager) BackendPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.port
}

// HealthCheck checks the provider and attempts a restart after failure.
func (m *I2PManager) HealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	if !m.running || m.service == nil {
		if m.state == "no_provider" || m.state == "starting" || m.state == "building" {
			return nil
		}
		if m.state == "error" && m.ctx != nil {
			return m.restartLocked(m.lastErr)
		}
		return fmt.Errorf("I2P provider is not running")
	}
	if err := m.service.healthCheck(); err != nil {
		log.Printf("I2P provider unresponsive, restarting: %v", err)
		return m.restartLocked(err)
	}
	return nil
}

func (m *I2PManager) restartLocked(cause error) error {
	if time.Now().Before(m.retryAt) {
		return errors.Join(cause, m.lastErr)
	}
	provider, binary, err := resolveProvider(m.cfg)
	if err != nil {
		m.running, m.provider, m.address = false, I2PProviderNone, ""
		m.providerView.Store(I2PProviderNone)
		m.runningView.Store(false)
		m.addressView.Store("")
		m.setState("no_provider")
		return errors.Join(cause, err)
	}
	if m.service != nil {
		_ = m.service.Close()
	}
	service, err := m.startProviderLocked(provider, binary, m.port)
	if err != nil {
		m.running, m.provider, m.address = false, provider, ""
		m.providerView.Store(provider)
		m.runningView.Store(false)
		m.addressView.Store("")
		m.lastErr = err
		m.retryDelay = nextI2PRetryDelay(err, m.retryDelay)
		m.retryAt = time.Now().Add(m.retryDelay)
		m.setState("error")
		return errors.Join(cause, err)
	}
	m.service, m.provider, m.address = service, provider, service.address
	m.providerView.Store(provider)
	m.addressView.Store(service.address)
	m.running = true
	m.runningView.Store(true)
	m.retryAt, m.retryDelay, m.lastErr = time.Time{}, 0, nil
	m.setState("running")
	return nil
}

// UpdateConfig applies a file reload by restarting I2P with the new settings.
func (m *I2PManager) ReloadI2PConfig(cfg I2PConfig) error {
	m.mu.Lock()
	parent, handler := m.startupCtx, m.handler
	if parent == nil {
		parent = m.ctx
	}
	if parent == nil {
		parent = context.Background()
	} else {
		parent = context.WithoutCancel(parent)
	}
	_ = m.closeLocked()
	m.resetLocked()
	m.startupCtx, m.startupCancel = nil, nil
	m.retryAt, m.retryDelay, m.lastErr = time.Time{}, 0, nil
	m.cfg = cfg.WithDefaults()
	m.enabledView.Store(m.cfg.Enabled)
	if !m.cfg.Enabled {
		m.setState("disabled")
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()
	return m.StartI2PService(parent, handler)
}

// RegenerateAddress deletes the destination identity and creates a new one.
func (m *I2PManager) RegenerateAddress() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled || !m.running {
		return "", fmt.Errorf("I2P provider is not running")
	}
	parent, handler := context.WithoutCancel(m.ctx), m.handler
	_ = m.closeLocked()
	if err := os.RemoveAll(m.dirs.siteDir()); err != nil {
		m.resetLocked()
		return "", fmt.Errorf("remove old I2P destination: %w", err)
	}
	m.resetLocked()
	if err := m.startLocked(parent, handler); err != nil {
		return "", err
	}
	return m.address, nil
}

// StopI2PService stops I2P and the dedicated backend listener.
func (m *I2PManager) StopI2PService() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.closeLocked()
	m.resetLocked()
	m.startupCtx, m.startupCancel, m.handler = nil, nil, nil
	return err
}

func (m *I2PManager) closeLocked() error {
	var err error
	if m.startupCancel != nil {
		m.startupCancel()
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.service != nil {
		err = errors.Join(err, m.service.Close())
	}
	if m.http != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = errors.Join(err, m.http.Shutdown(ctx))
		cancel()
	}
	if m.listener != nil {
		err = errors.Join(err, m.listener.Close())
	}
	return err
}

func (m *I2PManager) startProviderLocked(provider I2PProvider, binary string, port int) (*I2PProviderService, error) {
	svc := &I2PProviderService{provider: provider, backendPort: port}
	switch provider {
	case I2PProviderI2PD:
		samAddress, err := startI2PD(m.ctx, binary, m.dirs, svc)
		if err != nil {
			return nil, err
		}
		address, err := startSAMEepsite(m.ctx, m.cfg, samAddress, m.dirs.keysPath(), port, svc)
		if err != nil {
			_ = svc.Close()
			return nil, err
		}
		svc.address, svc.samAddress = address, samAddress
	case I2PProviderSAM:
		address, err := startSAMEepsite(m.ctx, m.cfg, m.cfg.SAMAddress, m.dirs.keysPath(), port, svc)
		if err != nil {
			return nil, err
		}
		svc.address, svc.samAddress = address, m.cfg.SAMAddress
	default:
		return nil, fmt.Errorf("no I2P provider available")
	}
	log.Printf("I2P eepsite started (%s): %s:%d -> 127.0.0.1:%d", provider, svc.address, m.cfg.VirtualPort, port)
	return svc, nil
}

func (m *I2PManager) monitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.HealthCheck(); err != nil {
				log.Printf("I2P health check failed: %v", err)
			}
		}
	}
}

func (m *I2PManager) resetLocked() {
	m.service, m.listener, m.http, m.cancel = nil, nil, nil, nil
	m.ctx = nil
	m.retryAt, m.retryDelay, m.lastErr = time.Time{}, 0, nil
	m.running, m.provider, m.address, m.port = false, I2PProviderNone, "", 0
	m.providerView.Store(I2PProviderNone)
	m.runningView.Store(false)
	m.addressView.Store("")
	if m.cfg.Enabled {
		m.setState("starting")
	} else {
		m.setState("disabled")
	}
}

func resolveProvider(cfg I2PConfig) (I2PProvider, string, error) {
	if binary, err := resolveI2PDBinary(cfg.Binary); err == nil {
		return I2PProviderI2PD, binary, nil
	}
	if samReachable(cfg.SAMAddress) {
		return I2PProviderSAM, "", nil
	}
	return I2PProviderNone, "", fmt.Errorf("i2p enabled but no provider available (no i2pd binary, SAM %s unreachable)", cfg.SAMAddress)
}

func samReachable(address string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return probeSAM(ctx, address) == nil
}

func resolveI2PDBinary(configured string) (string, error) {
	if configured != "" {
		return exec.LookPath(configured)
	}
	locations := []string{"/usr/bin/i2pd", "/usr/sbin/i2pd", "/usr/local/bin/i2pd"}
	if runtime.GOOS == "darwin" {
		locations = append(locations, "/opt/homebrew/bin/i2pd")
	}
	for _, candidate := range locations {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	if binary, err := exec.LookPath("i2pd"); err == nil {
		return binary, nil
	}
	return "", fmt.Errorf("i2pd binary not found")
}

func allocateBackendListener() (net.Listener, int, error) {
	for tries := 0; tries < 100; tries++ {
		n, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			return nil, 0, err
		}
		port := 64000 + int(n.Int64())
		listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return listener, port, nil
		}
	}
	return nil, 0, fmt.Errorf("could not allocate loopback port in 64000-64999")
}

func (s *I2PProviderService) healthCheck() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider == I2PProviderI2PD {
		select {
		case <-s.i2pdDone:
			if s.i2pdErr == nil {
				return fmt.Errorf("i2pd process exited")
			}
			return fmt.Errorf("i2pd process exited: %w", s.i2pdErr)
		default:
		}
	}
	if s.samConn == nil || s.forwardConn == nil {
		return fmt.Errorf("SAM session is closed")
	}
	select {
	case <-s.sessionLost:
		return fmt.Errorf("SAM session control connection closed")
	default:
	}
	select {
	case <-s.forwardLost:
		return fmt.Errorf("SAM stream forwarding connection closed")
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := probeSAM(ctx, s.samAddress); err != nil {
		s.probeFails++
		if s.probeFails >= 3 {
			return fmt.Errorf("SAM bridge failed three consecutive liveness probes: %w", err)
		}
		return nil
	}
	s.probeFails = 0
	return nil
}

func (s *I2PProviderService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if s.forwardConn != nil {
		err = errors.Join(err, s.forwardConn.Close())
		s.forwardConn = nil
	}
	if s.samConn != nil {
		err = errors.Join(err, s.samConn.Close())
		s.samConn = nil
	}
	if s.i2pd != nil && s.i2pd.Process != nil {
		if runtime.GOOS == "windows" {
			err = errors.Join(err, s.i2pd.Process.Kill())
		} else {
			err = errors.Join(err, s.i2pd.Process.Signal(os.Interrupt))
			select {
			case <-s.i2pdDone:
			case <-time.After(10 * time.Second):
				err = errors.Join(err, s.i2pd.Process.Kill())
				<-s.i2pdDone
			}
		}
		s.i2pd = nil
	}
	if s.pidPath != "" {
		if removeErr := os.Remove(s.pidPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
		s.pidPath = ""
	}
	return err
}
