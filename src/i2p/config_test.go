package i2p

import (
	"strconv"
	"testing"
)

func TestDefaultConfigIsOptIn(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Fatal("I2P must be disabled by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default I2P configuration is invalid: %v", err)
	}
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
	cfg := DefaultConfig()
	cfg.VirtualPort = 65536
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid virtual port was accepted")
	}
}
