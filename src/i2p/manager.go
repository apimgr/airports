package i2p

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Provider identifies the provider that owns the eepsite tunnel.
type Provider string

const (
	ProviderNone Provider = "none"
	ProviderI2PD Provider = "i2pd"
	ProviderSAM  Provider = "sam"
)

type destination struct {
	public  []byte
	private string
}

// Service tracks the active provider process or SAM session.
type Service struct {
	provider    Provider
	address     string
	backendPort int
	i2pd        *exec.Cmd
	i2pdDone    chan struct{}
	i2pdErr     error
	samConn     net.Conn
	samReader   *bufio.Reader
	pidPath     string
	mu          sync.Mutex
}

// Manager owns an opt-in I2P provider and its dedicated loopback backend.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	dirs     Dirs
	service  *Service
	listener net.Listener
	http     *http.Server
	handler  http.Handler
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	provider Provider
	address  string
	port     int
}

// NewManager constructs an inactive manager without touching disk or network.
func NewManager(cfg Config, dirs Dirs) *Manager {
	return &Manager{cfg: cfg.WithDefaults(), dirs: dirs, provider: ProviderNone}
}

// Start launches the provider only when explicitly enabled.
func (m *Manager) Start(parent context.Context, handler http.Handler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked(parent, handler)
}

func (m *Manager) startLocked(parent context.Context, handler http.Handler) error {
	if !m.cfg.Enabled {
		log.Printf("I2P eepsite disabled (opt-in)")
		return nil
	}
	if m.running {
		return nil
	}
	if handler == nil {
		return fmt.Errorf("I2P backend handler is required")
	}
	if err := m.cfg.Validate(); err != nil {
		return err
	}
	provider, binary, err := resolveProvider(m.cfg)
	if err != nil {
		return err
	}
	if err := ensureDirs(m.dirs); err != nil {
		return err
	}
	listener, port, err := allocateBackendListener()
	if err != nil {
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
		return err
	}
	m.service, m.provider, m.address, m.running = service, provider, service.address, true
	go m.monitor(ctx)
	return nil
}

// Enabled reports the opt-in configuration.
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.Enabled
}

// Running reports whether the provider is active.
func (m *Manager) Running() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Provider returns the running provider name or none.
func (m *Manager) Provider() Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.provider
}

// EepsiteAddress returns the published address when active.
func (m *Manager) EepsiteAddress() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running {
		return ""
	}
	return m.address
}

// BackendPort returns the provider's dedicated loopback target port.
func (m *Manager) BackendPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.port
}

// HealthCheck checks the provider and attempts a restart after failure.
func (m *Manager) HealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	if !m.running || m.service == nil {
		return fmt.Errorf("I2P provider is not running")
	}
	if err := m.service.healthCheck(); err != nil {
		log.Printf("I2P provider unresponsive, restarting: %v", err)
		provider, binary, resolveErr := resolveProvider(m.cfg)
		if resolveErr != nil {
			m.running, m.provider, m.address = false, ProviderNone, ""
			return errors.Join(err, resolveErr)
		}
		_ = m.service.Close()
		service, startErr := m.startProviderLocked(provider, binary, m.port)
		if startErr != nil {
			m.running, m.provider, m.address = false, ProviderNone, ""
			return errors.Join(err, startErr)
		}
		m.service, m.provider, m.address = service, provider, service.address
	}
	return nil
}

// UpdateConfig applies a file reload by restarting I2P with the new settings.
func (m *Manager) UpdateConfig(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	parent, handler := m.ctx, m.handler
	if parent == nil {
		parent = context.Background()
	} else {
		parent = context.WithoutCancel(parent)
	}
	_ = m.closeLocked()
	m.resetLocked()
	m.cfg = cfg.WithDefaults()
	if !m.cfg.Enabled {
		return nil
	}
	return m.startLocked(parent, handler)
}

// RegenerateAddress deletes the destination identity and creates a new one.
func (m *Manager) RegenerateAddress() (string, error) {
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

// Close stops I2P and the dedicated backend listener.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.closeLocked()
	m.resetLocked()
	return err
}

func (m *Manager) closeLocked() error {
	var err error
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

func (m *Manager) startProviderLocked(provider Provider, binary string, port int) (*Service, error) {
	svc := &Service{provider: provider, backendPort: port}
	switch provider {
	case ProviderI2PD:
		address, err := startI2PD(m.ctx, m.cfg, binary, m.dirs, port, svc)
		if err != nil {
			return nil, err
		}
		svc.address = address
	case ProviderSAM:
		address, err := startSAMEepsite(m.ctx, m.cfg, m.dirs.keysPath(), port, svc)
		if err != nil {
			return nil, err
		}
		svc.address = address
	default:
		return nil, fmt.Errorf("no I2P provider available")
	}
	log.Printf("I2P eepsite started (%s): %s:%d -> 127.0.0.1:%d", provider, svc.address, m.cfg.VirtualPort, port)
	return svc, nil
}

func (m *Manager) monitor(ctx context.Context) {
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

func (m *Manager) resetLocked() {
	m.service, m.listener, m.http, m.cancel = nil, nil, nil, nil
	m.ctx, m.handler = nil, nil
	m.running, m.provider, m.address, m.port = false, ProviderNone, "", 0
}

func resolveProvider(cfg Config) (Provider, string, error) {
	if binary, err := resolveI2PDBinary(cfg.Binary); err == nil {
		return ProviderI2PD, binary, nil
	}
	if samReachable(cfg.SAMAddress) {
		return ProviderSAM, "", nil
	}
	return ProviderNone, "", fmt.Errorf("i2p enabled but no provider available (no i2pd binary, SAM %s unreachable)", cfg.SAMAddress)
}

func samReachable(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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

func (s *Service) healthCheck() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider == ProviderI2PD {
		select {
		case <-s.i2pdDone:
			err := s.i2pdErr
			if err == nil {
				return fmt.Errorf("i2pd process exited")
			}
			return fmt.Errorf("i2pd process exited: %w", err)
		default:
			return nil
		}
	}
	if s.provider != ProviderSAM || s.samConn == nil {
		return fmt.Errorf("SAM session is closed")
	}
	_ = s.samConn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintln(s.samConn, "PING airports-health"); err != nil {
		return err
	}
	for {
		line, err := s.samReader.ReadString('\n')
		if err != nil {
			_ = s.samConn.SetDeadline(time.Time{})
			return err
		}
		if line == "PONG airports-health\n" {
			_ = s.samConn.SetDeadline(time.Time{})
			return nil
		}
	}
}

// Close shuts down the provider process or SAM session.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if s.samConn != nil {
		err = errors.Join(err, s.samConn.Close())
		s.samConn = nil
	}
	if s.i2pd != nil && s.i2pd.Process != nil {
		if runtime.GOOS == "windows" {
			err = errors.Join(err, s.i2pd.Process.Kill())
		} else if signalErr := s.i2pd.Process.Signal(os.Interrupt); signalErr != nil {
			err = errors.Join(err, signalErr, s.i2pd.Process.Kill())
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

func startI2PD(ctx context.Context, cfg Config, binary string, dirs Dirs, port int, svc *Service) (string, error) {
	tunnels := fmt.Sprintf("[site]\ntype = server\nhost = 127.0.0.1\nport = %d\nkeys = %s\ninbound.length = %d\noutbound.length = %d\ninbound.quantity = %d\noutbound.quantity = %d\nsignaturetype = %d\n",
		port, dirs.keysPath(), cfg.InboundLength, cfg.OutboundLength, cfg.InboundQuantity, cfg.OutboundQuantity, cfg.SignatureType)
	i2pdConfigPath := filepath.Join(dirs.i2pConfigDir(), "i2pd.conf")
	i2pdConfig := "daemon = false\nservice = false\n[sam]\nenabled = false\n[http]\nenabled = false\n"
	if err := writePrivateFile(i2pdConfigPath, []byte(i2pdConfig)); err != nil {
		return "", fmt.Errorf("write i2pd.conf: %w", err)
	}
	tunnelsPath := filepath.Join(dirs.i2pConfigDir(), "tunnels.conf")
	if err := writePrivateFile(tunnelsPath, []byte(tunnels)); err != nil {
		return "", fmt.Errorf("write tunnels.conf: %w", err)
	}
	if err := os.MkdirAll(dirs.LogDir, 0700); err != nil {
		return "", err
	}
	logPath := filepath.Join(dirs.LogDir, "i2pd.log")
	cmd := exec.CommandContext(ctx, binary, "--conf", i2pdConfigPath, "--datadir", dirs.i2pDataDir(), "--tunconf", tunnelsPath, "--log=file", "--loglevel=warn", "--logfile="+logPath)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start i2pd: %w", err)
	}
	svc.i2pd, svc.i2pdDone = cmd, make(chan struct{})
	svc.pidPath = filepath.Join(dirs.i2pDataDir(), "i2pd.pid")
	if err := writePrivateFile(svc.pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid))); err != nil {
		_ = cmd.Process.Kill()
		return "", err
	}
	go func() {
		waitErr := cmd.Wait()
		svc.mu.Lock()
		svc.i2pdErr = waitErr
		close(svc.i2pdDone)
		svc.mu.Unlock()
	}()
	timeout, err := time.ParseDuration(cfg.BootstrapTimeout)
	if err != nil {
		timeout = 5 * time.Minute
	}
	address, waitErr := waitForI2PDAddress(ctx, dirs.keysPath(), timeout, svc.i2pdDone)
	if waitErr != nil {
		_ = svc.Close()
		return "", waitErr
	}
	return address, nil
}

func waitForI2PDAddress(ctx context.Context, keysPath string, timeout time.Duration, processDone <-chan struct{}) (string, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if address, err := addressFromI2PDKeys(keysPath); err == nil {
			if runtime.GOOS != "windows" {
				_ = os.Chmod(keysPath, 0600)
				_ = os.Chown(keysPath, os.Getuid(), os.Getgid())
			}
			return address, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-processDone:
			return "", fmt.Errorf("i2pd exited before publishing destination")
		case <-timer.C:
			return "", fmt.Errorf("timed out waiting for i2pd destination")
		case <-ticker.C:
		}
	}
}

func addressFromI2PDKeys(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 0 && data[0] >= 'A' && data[0] <= 'z' {
		if decoded, decodeErr := base64.StdEncoding.DecodeString(string(data)); decodeErr == nil {
			data = decoded
		}
	}
	if len(data) < 387 {
		return "", fmt.Errorf("i2pd destination key file is incomplete")
	}
	certLength := int(data[385])<<8 | int(data[386])
	destinationLength := 387 + certLength
	if destinationLength > len(data) {
		return "", fmt.Errorf("invalid i2pd destination certificate length")
	}
	return b32Address(data[:destinationLength]), nil
}

func startSAMEepsite(ctx context.Context, cfg Config, keysPath string, port int, svc *Service) (string, error) {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", cfg.SAMAddress)
	if err != nil {
		return "", fmt.Errorf("dial SAM %s: %w", cfg.SAMAddress, err)
	}
	reader := bufio.NewReader(conn)
	timeout, err := time.ParseDuration(cfg.BootstrapTimeout)
	if err != nil || timeout <= 0 {
		timeout = 5 * time.Minute
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprint(conn, "HELLO VERSION MIN=3.0 MAX=3.3\n"); err != nil {
		_ = conn.Close()
		return "", err
	}
	if _, err := readSAMReply(reader, "HELLO REPLY"); err != nil {
		_ = conn.Close()
		return "", err
	}
	dest, err := loadOrCreateSAMDestination(conn, reader, keysPath, cfg.SignatureType)
	if err != nil {
		_ = conn.Close()
		return "", err
	}
	if _, err := fmt.Fprintf(conn, "SESSION CREATE STYLE=STREAM ID=site DESTINATION=%s inbound.length=%d outbound.length=%d inbound.quantity=%d outbound.quantity=%d\n",
		dest.private, cfg.InboundLength, cfg.OutboundLength, cfg.InboundQuantity, cfg.OutboundQuantity); err != nil {
		_ = conn.Close()
		return "", err
	}
	if _, err := readSAMReply(reader, "SESSION STATUS"); err != nil {
		_ = conn.Close()
		return "", err
	}
	if _, err := fmt.Fprintf(conn, "STREAM FORWARD ID=site PORT=%d HOST=127.0.0.1\n", port); err != nil {
		_ = conn.Close()
		return "", err
	}
	if _, err := readSAMReply(reader, "STREAM STATUS"); err != nil {
		_ = conn.Close()
		return "", err
	}
	_ = conn.SetDeadline(time.Time{})
	svc.samConn, svc.samReader = conn, reader
	return b32Address(dest.public), nil
}

func readSAMReply(reader *bufio.Reader, prefix string) (map[string]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
		return nil, fmt.Errorf("unexpected SAM reply %q", strings.TrimSpace(line))
	}
	reply := make(map[string]string)
	for _, field := range strings.Fields(line)[2:] {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			reply[key] = strings.Trim(value, "\"' ")
		}
	}
	if status := reply["RESULT"]; status != "" && status != "OK" {
		return nil, fmt.Errorf("SAM command failed: %s", status)
	}
	return reply, nil
}

func loadOrCreateSAMDestination(conn net.Conn, reader *bufio.Reader, path string, signatureType int) (destination, error) {
	if dest, err := readSAMDestination(path); err == nil {
		return dest, nil
	}
	if _, err := fmt.Fprintf(conn, "DEST GENERATE SIGNATURE_TYPE=%d\n", signatureType); err != nil {
		return destination{}, err
	}
	reply, err := readSAMReply(reader, "DEST REPLY")
	if err != nil {
		return destination{}, err
	}
	publicText, private := reply["PUB"], reply["PRIV"]
	public, err := base64.StdEncoding.DecodeString(publicText)
	if err != nil || len(public) == 0 || private == "" {
		return destination{}, fmt.Errorf("SAM DEST REPLY contained invalid PUB/PRIV")
	}
	if err := writePrivateFile(path, []byte("PUB="+publicText+"\nPRIV="+private+"\n")); err != nil {
		return destination{}, err
	}
	return destination{public: public, private: private}, nil
}

func readSAMDestination(path string) (destination, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return destination{}, err
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			fields[key] = value
		}
	}
	public, err := base64.StdEncoding.DecodeString(fields["PUB"])
	if err != nil || len(public) == 0 || fields["PRIV"] == "" {
		return destination{}, fmt.Errorf("I2P SAM destination file is invalid")
	}
	return destination{public: public, private: fields["PRIV"]}, nil
}

func b32Address(publicDestination []byte) string {
	hash := sha256.Sum256(publicDestination)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])) + ".b32.i2p"
}
