package tor

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestNewManagerDisabledWhenExplicitFalse(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if m.Enabled() {
		t.Error("Manager.Enabled() = true, want false when Enabled is explicitly false")
	}
}

func TestNewManagerAutoDetectDisabledWithoutTorBinary(t *testing.T) {
	if _, err := exec.LookPath("tor"); err == nil {
		t.Skip("a real tor binary is installed on this machine; auto-detect enabled path is exercised instead")
	}

	m := NewManager("airports-test", 8080, Config{})

	if m.Enabled() {
		t.Error("Manager.Enabled() = true, want false when no tor binary can be found and Enabled is unset")
	}
}

func TestManagerStartInertWhenDisabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil (inert no-op when disabled)", err)
	}

	if addr := m.OnionAddress(); addr != "" {
		t.Errorf("OnionAddress() = %q, want empty when Tor was never started", addr)
	}
}

func TestManagerHealthCheckNotEnabled(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	err := m.HealthCheck()
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("HealthCheck() error = %v, want ErrNotEnabled", err)
	}
}

func TestManagerHealthCheckEnabledButNotStarted(t *testing.T) {
	yes := true
	m := NewManager("airports-test", 8080, Config{Enabled: &yes})

	err := m.HealthCheck()
	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("HealthCheck() error = %v, want ErrNotRunning (enabled but Start never succeeded)", err)
	}
}

func TestManagerStopWithoutStartIsNoop(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

	if err := m.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil when Tor was never started", err)
	}
}

func TestManagerGetHTTPClientDirectWhenNoDialer(t *testing.T) {
	no := false
	m := NewManager("airports-test", 8080, Config{Enabled: &no})

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

	yes := true
	m := NewManager("airports-test", 18080, Config{Enabled: &yes, BootstrapTimeout: 60})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer m.Stop()

	if err := m.HealthCheck(); err != nil {
		t.Errorf("HealthCheck() error = %v, want nil once Tor has started", err)
	}

	if addr := m.OnionAddress(); addr == "" {
		t.Error("OnionAddress() is empty after a successful Start with a real tor binary")
	}
}
