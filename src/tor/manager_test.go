package tor

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func managerForTest(t *testing.T, projectName string, port int, enabled bool) *Manager {
	t.Helper()
	missingBinary := filepath.Join(t.TempDir(), "tor-not-installed")
	m := NewManager(projectName, port, Config{Binary: missingBinary})
	// Tests that exercise manager methods without launching Tor set this
	// internal state directly; production enablement remains binary-detected.
	m.enabled = enabled
	return m
}

func TestNewManagerDisabledWhenTorBinaryMissing(t *testing.T) {
	m := NewManager("airports-test", 8080, Config{Binary: filepath.Join(t.TempDir(), "tor-not-installed")})

	if m.Enabled() {
		t.Error("Manager.Enabled() = true, want false when no Tor binary can be found")
	}
}

func TestNewManagerAutoEnablesWhenTorBinaryFound(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "tor")
	if err := os.WriteFile(binary, []byte("tor fixture"), 0o700); err != nil {
		t.Fatalf("create Tor binary fixture: %v", err)
	}
	m := NewManager("airports-test", 8080, Config{Binary: binary})
	if !m.Enabled() {
		t.Error("Manager.Enabled() = false, want true when a configured Tor binary is found")
	}
}

func TestManagerStartInertWhenDisabled(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil (inert no-op when disabled)", err)
	}

	if addr := m.OnionAddress(); addr != "" {
		t.Errorf("OnionAddress() = %q, want empty when Tor was never started", addr)
	}
}

func TestCircuitEstablishedFalseWhenNotRunning(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, true)
	if m.CircuitEstablished() {
		t.Fatal("CircuitEstablished() = true before Tor starts, want false")
	}
}

func TestManagerHealthCheckNotEnabled(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	err := m.HealthCheck()
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("HealthCheck() error = %v, want ErrNotEnabled", err)
	}
}

func TestManagerHealthCheckEnabledButNotStarted(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, true)

	err := m.HealthCheck()
	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("HealthCheck() error = %v, want ErrNotRunning (enabled but Start never succeeded)", err)
	}
}

func TestManagerStopWithoutStartIsNoop(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	if err := m.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil when Tor was never started", err)
	}
}

func TestManagerStopClosesBackendWithoutTorProcess(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for backend fixture: %v", err)
	}
	m.backend = listener

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if m.backend != nil {
		t.Fatal("Stop() retained backend listener after closing it")
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond); err == nil {
		t.Fatal("backend listener still accepts connections after Stop()")
	}
}

func TestManagerGetHTTPClientDirectWhenNoDialer(t *testing.T) {
	m := managerForTest(t, "airports-test", 8080, false)

	client := m.GetHTTPClient(true)
	if client == nil {
		t.Fatal("GetHTTPClient() returned nil")
	}
	if client.Transport != nil {
		t.Error("GetHTTPClient(true) with no outbound dialer should return a plain direct-connection client")
	}
}

func TestManagerStartWithRealTorBinary(t *testing.T) {
	if _, err := exec.LookPath("tor"); err != nil {
		t.Skip("tor binary not installed, skipping real process start/stop test")
	}

	m := NewManager("airports-test", 18080, Config{BootstrapTimeout: 60})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = m.Stop() }()

	if err := m.HealthCheck(); err != nil {
		t.Errorf("HealthCheck() error = %v, want nil once Tor has started", err)
	}

	if addr := m.OnionAddress(); addr == "" {
		t.Error("OnionAddress() is empty after a successful Start with a real tor binary")
	}
}
