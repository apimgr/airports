package i2p

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	paths "github.com/apimgr/airports/src/path"
)

func TestDefaultI2PConfigIsOptIn(t *testing.T) {
	cfg := DefaultI2PConfig()
	if cfg.Enabled {
		t.Fatal("I2P must be disabled by default")
	}
	if err := cfg.ValidateI2PConfig(); err != nil {
		t.Fatalf("default I2P configuration is invalid: %v", err)
	}
}

func TestManagerInitialLifecycleState(t *testing.T) {
	disabled := NewI2PManager(DefaultI2PConfig(), I2PDirs{})
	if disabled.GetI2PStatus() != "disabled" || disabled.IsI2PEnabled() {
		t.Fatalf("disabled manager state=%q enabled=%t", disabled.GetI2PStatus(), disabled.IsI2PEnabled())
	}
	enabledConfig := DefaultI2PConfig()
	enabledConfig.Enabled = true
	baseDir := t.TempDir()
	enabled := NewI2PManager(enabledConfig, I2PDirs{
		ConfigDir: filepath.Join(baseDir, "config"),
		DataDir:   filepath.Join(baseDir, "data"),
		LogDir:    filepath.Join(baseDir, "logs"),
	})
	if enabled.GetI2PStatus() != "starting" || !enabled.IsI2PEnabled() || enabled.IsI2PRunning() {
		t.Fatalf("enabled manager state=%q enabled=%t running=%t", enabled.GetI2PStatus(), enabled.IsI2PEnabled(), enabled.IsI2PRunning())
	}
}

func TestManagerStartReturnsBeforeProviderResolution(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("create temp base: %v", err)
	}
	dir, err := os.MkdirTemp(base, "airports-i2p-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cfg := DefaultI2PConfig()
	cfg.Enabled = true
	cfg.Binary = filepath.Join(dir, "missing-i2pd")
	cfg.SAMAddress = "127.0.0.1:1"
	manager := NewI2PManager(cfg, I2PDirs{
		ConfigDir: filepath.Join(dir, "config"),
		DataDir:   filepath.Join(dir, "data"),
		LogDir:    filepath.Join(dir, "logs"),
	})
	t0 := time.Now()
	if err := manager.StartI2PService(context.Background(), http.NotFoundHandler()); err != nil {
		t.Fatalf("Start returned an unexpected error: %v", err)
	}
	if elapsed := time.Since(t0); elapsed > time.Second {
		t.Fatalf("Start blocked for provider resolution: %s", elapsed)
	}
	deadline := time.Now().Add(2 * time.Second)
	for manager.GetI2PStatus() == "starting" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if manager.GetI2PStatus() != "no_provider" {
		t.Fatalf("state after failed provider resolution = %q, want no_provider", manager.GetI2PStatus())
	}
	if err := manager.StopI2PService(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestManagerHealthStateReadsDoNotWaitForBootstrapLock(t *testing.T) {
	cfg := DefaultI2PConfig()
	cfg.Enabled = true
	manager := NewI2PManager(cfg, I2PDirs{})
	manager.mu.Lock()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_ = manager.GetI2PStatus()
		_ = manager.IsI2PEnabled()
		_ = manager.IsI2PRunning()
		_ = manager.EepsiteAddress()
		_ = manager.GetI2PProvider()
	}()
	select {
	case <-readDone:
	case <-time.After(100 * time.Millisecond):
		manager.mu.Unlock()
		t.Fatal("health state reads waited for the bootstrap mutex")
	}
	manager.mu.Unlock()
}

func TestBackendListenerUsesDedicatedLoopbackRange(t *testing.T) {
	listener, port, err := allocateBackendListener()
	if err != nil {
		t.Fatalf("allocate backend listener: %v", err)
	}
	defer func() { _ = listener.Close() }()
	if port < 64000 || port > 64999 {
		t.Fatalf("backend port %d is outside 64000-64999", port)
	}
	if listener.Addr().String() != "127.0.0.1:"+strconv.Itoa(port) {
		t.Fatalf("backend listener bound to %s", listener.Addr())
	}
}

func TestConfigRejectsInvalidVirtualPort(t *testing.T) {
	cfg := DefaultI2PConfig()
	cfg.VirtualPort = 65536
	if err := cfg.ValidateI2PConfig(); err == nil {
		t.Fatal("invalid virtual port was accepted")
	}
}

func TestBootstrapTimeoutUsesSecondsAndEnforcesLimits(t *testing.T) {
	for _, timeout := range []int{30, 300, 600} {
		cfg := DefaultI2PConfig()
		cfg.BootstrapTimeout = timeout
		if err := cfg.ValidateI2PConfig(); err != nil {
			t.Errorf("timeout %d should be valid: %v", timeout, err)
		}
	}
	for _, timeout := range []int{1, 29, 601} {
		cfg := DefaultI2PConfig()
		cfg.BootstrapTimeout = timeout
		if err := cfg.ValidateI2PConfig(); err == nil {
			t.Errorf("timeout %d should be rejected", timeout)
		}
	}
}

func TestI2PRetryDelayBacksOffAndHandlesDuplicateDestination(t *testing.T) {
	if got := nextI2PRetryDelay(errors.New("SESSION CREATE failed: DUPLICATED_DEST"), 10*time.Second); got != 60*time.Second {
		t.Fatalf("duplicate destination retry delay = %s, want 60s", got)
	}
	for _, test := range []struct {
		previous time.Duration
		want     time.Duration
	}{
		{previous: 0, want: 5 * time.Second},
		{previous: 5 * time.Second, want: 10 * time.Second},
		{previous: 150 * time.Second, want: 300 * time.Second},
		{previous: 300 * time.Second, want: 300 * time.Second},
	} {
		if got := nextI2PRetryDelay(errors.New("SAM session creation failed"), test.previous); got != test.want {
			t.Errorf("retry delay after %s = %s, want %s", test.previous, got, test.want)
		}
	}
}

func TestEnabledFeatureRemainsEnabledWithoutProvider(t *testing.T) {
	manager := NewI2PManager(I2PConfig{Enabled: true}, I2PDirs{})
	manager.setState("no_provider")
	if !manager.IsI2PEnabled() {
		t.Fatal("configured I2P feature reported disabled when no provider was available")
	}
}

func TestManagerDisabledAndInvalidLifecycleOperations(t *testing.T) {
	disabled := NewI2PManager(DefaultI2PConfig(), I2PDirs{})
	if err := disabled.StartI2PService(context.Background(), nil); err != nil {
		t.Fatalf("starting disabled I2P: %v", err)
	}
	if err := disabled.HealthCheck(); err != nil {
		t.Errorf("health check for disabled I2P: %v", err)
	}
	if _, err := disabled.RegenerateAddress(); err == nil {
		t.Error("regenerating an inactive destination succeeded")
	}
	if err := disabled.ReloadI2PConfig(DefaultI2PConfig()); err != nil {
		t.Fatalf("reloading disabled I2P config: %v", err)
	}
	if disabled.GetI2PStatus() != "disabled" || disabled.IsI2PEnabled() || disabled.IsI2PRunning() {
		t.Errorf("disabled state after reload: status=%q enabled=%t running=%t", disabled.GetI2PStatus(), disabled.IsI2PEnabled(), disabled.IsI2PRunning())
	}

	enabledConfig := DefaultI2PConfig()
	enabledConfig.Enabled = true
	enabled := NewI2PManager(enabledConfig, I2PDirs{})
	if err := enabled.StartI2PService(context.Background(), nil); err == nil {
		t.Error("enabled I2P accepted a missing backend handler")
	}
	if err := enabled.StartI2PService(context.Background(), http.NotFoundHandler()); err != nil {
		t.Fatalf("starting enabled manager with backend handler: %v", err)
	}
	invalidConfig := enabledConfig
	invalidConfig.VirtualPort = 65536
	if err := enabled.ReloadI2PConfig(invalidConfig); err == nil {
		t.Fatal("reloading accepted an invalid virtual port")
	}
	if enabled.GetI2PStatus() != "error" {
		t.Fatalf("invalid reloaded config state = %q, want error", enabled.GetI2PStatus())
	}
	if err := enabled.HealthCheck(); err == nil {
		t.Error("health check accepted enabled I2P in error state without a provider")
	}
	if err := enabled.StopI2PService(); err != nil {
		t.Errorf("stopping I2P after invalid config: %v", err)
	}
}
