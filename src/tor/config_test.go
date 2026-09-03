package tor

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled != nil {
		t.Errorf("Enabled = %v, want nil (auto-detect)", cfg.Enabled)
	}
	if cfg.Binary != "" {
		t.Errorf("Binary = %q, want empty (auto-detect)", cfg.Binary)
	}
	if cfg.UseNetwork {
		t.Error("UseNetwork = true, want false by default")
	}
	if cfg.MaxCircuits != 32 {
		t.Errorf("MaxCircuits = %d, want 32", cfg.MaxCircuits)
	}
	if cfg.CircuitTimeout != 60 {
		t.Errorf("CircuitTimeout = %d, want 60", cfg.CircuitTimeout)
	}
	if cfg.BootstrapTimeout != 180 {
		t.Errorf("BootstrapTimeout = %d, want 180", cfg.BootstrapTimeout)
	}
	if !cfg.SafeLogging {
		t.Error("SafeLogging = false, want true by default")
	}
	if cfg.MaxStreamsPerCircuit != 100 {
		t.Errorf("MaxStreamsPerCircuit = %d, want 100", cfg.MaxStreamsPerCircuit)
	}
	if !cfg.CloseCircuitOnStreamLimit {
		t.Error("CloseCircuitOnStreamLimit = false, want true by default")
	}
	if cfg.BandwidthRate != "1 MB" {
		t.Errorf("BandwidthRate = %q, want %q", cfg.BandwidthRate, "1 MB")
	}
	if cfg.BandwidthBurst != "2 MB" {
		t.Errorf("BandwidthBurst = %q, want %q", cfg.BandwidthBurst, "2 MB")
	}
	if cfg.MaxMonthlyBandwidth != "100 GB" {
		t.Errorf("MaxMonthlyBandwidth = %q, want %q", cfg.MaxMonthlyBandwidth, "100 GB")
	}
	if cfg.NumIntroPoints != 3 {
		t.Errorf("NumIntroPoints = %d, want 3", cfg.NumIntroPoints)
	}
	if cfg.VirtualPort != 80 {
		t.Errorf("VirtualPort = %d, want 80", cfg.VirtualPort)
	}
}

func TestApplyDefaultsFillsZeroValuesOnly(t *testing.T) {
	partial := Config{
		MaxCircuits: 8,
		// everything else left zero-value
	}

	got := applyDefaults(partial)

	if got.MaxCircuits != 8 {
		t.Errorf("MaxCircuits = %d, want unchanged 8", got.MaxCircuits)
	}
	if got.CircuitTimeout != 60 {
		t.Errorf("CircuitTimeout = %d, want filled default 60", got.CircuitTimeout)
	}
	if got.BandwidthRate != "1 MB" {
		t.Errorf("BandwidthRate = %q, want filled default", got.BandwidthRate)
	}
	if got.VirtualPort != 80 {
		t.Errorf("VirtualPort = %d, want filled default 80", got.VirtualPort)
	}
}

func TestApplyDefaultsPreservesExplicitValues(t *testing.T) {
	explicit := Config{
		MaxCircuits:          64,
		CircuitTimeout:       120,
		BootstrapTimeout:     300,
		MaxStreamsPerCircuit: 200,
		BandwidthRate:        "5 MB",
		BandwidthBurst:       "10 MB",
		MaxMonthlyBandwidth:  "unlimited",
		NumIntroPoints:       7,
		VirtualPort:          8080,
	}

	got := applyDefaults(explicit)

	if got != explicit {
		t.Errorf("applyDefaults changed an explicitly-set config: got %+v, want %+v", got, explicit)
	}
}

func TestResolveEnabledExplicitTrue(t *testing.T) {
	yes := true
	cfg := Config{Enabled: &yes}

	got := resolveEnabled(cfg, func(string) (string, error) {
		return "", ErrBinaryNotFound
	})

	if !got {
		t.Error("resolveEnabled() = false, want true (explicit override wins even with no binary)")
	}
}

func TestResolveEnabledExplicitFalse(t *testing.T) {
	no := false
	cfg := Config{Enabled: &no}

	got := resolveEnabled(cfg, func(string) (string, error) {
		return "/usr/bin/tor", nil
	})

	if got {
		t.Error("resolveEnabled() = true, want false (explicit override wins even with a binary present)")
	}
}

func TestResolveEnabledAutoDetectFound(t *testing.T) {
	cfg := Config{}

	got := resolveEnabled(cfg, func(string) (string, error) {
		return "/usr/bin/tor", nil
	})

	if !got {
		t.Error("resolveEnabled() = false, want true (auto-detect found a binary)")
	}
}

func TestResolveEnabledAutoDetectNotFound(t *testing.T) {
	cfg := Config{}

	got := resolveEnabled(cfg, func(string) (string, error) {
		return "", ErrBinaryNotFound
	})

	if got {
		t.Error("resolveEnabled() = true, want false (auto-detect found no binary)")
	}
}
