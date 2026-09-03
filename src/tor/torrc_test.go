package tor

import (
	"fmt"
	"strings"
	"testing"
)

const testLogFile = "/tmp/test-tor.log"

func TestGenerateTorrcHiddenServiceOnly(t *testing.T) {
	cfg := applyDefaults(Config{UseNetwork: false})

	content := generateTorrc(cfg, testLogFile)

	if !strings.Contains(content, "SocksPort 0") {
		t.Error("torrc missing 'SocksPort 0' when UseNetwork is false")
	}
	if strings.Contains(content, "SocksPort auto") {
		t.Error("torrc contains 'SocksPort auto' when UseNetwork is false")
	}
	if !strings.Contains(content, "ORPort 0") || !strings.Contains(content, "DirPort 0") {
		t.Error("torrc must disable ORPort and DirPort (never a relay/directory)")
	}
	if !strings.Contains(content, "ExitRelay 0") {
		t.Error("torrc must disable ExitRelay (never an exit node)")
	}
}

func TestGenerateTorrcOutboundEnabled(t *testing.T) {
	cfg := applyDefaults(Config{UseNetwork: true})

	content := generateTorrc(cfg, testLogFile)

	if !strings.Contains(content, "SocksPort auto") {
		t.Error("torrc missing 'SocksPort auto' when UseNetwork is true")
	}
}

func TestGenerateTorrcSafeLoggingToggle(t *testing.T) {
	on := generateTorrc(applyDefaults(Config{SafeLogging: true}), testLogFile)
	if !strings.Contains(on, "SafeLogging 1") {
		t.Error("torrc missing 'SafeLogging 1' when SafeLogging is true")
	}

	cfg := applyDefaults(Config{})
	cfg.SafeLogging = false
	off := generateTorrc(cfg, testLogFile)
	if !strings.Contains(off, "SafeLogging 0") {
		t.Error("torrc missing 'SafeLogging 0' when SafeLogging is false")
	}
}

func TestGenerateTorrcNeverUsesDefaultTorPorts(t *testing.T) {
	content := generateTorrc(applyDefaults(Config{UseNetwork: true}), testLogFile)

	if strings.Contains(content, "9050") || strings.Contains(content, "9051") {
		t.Error("torrc must never reference the system Tor default ports 9050/9051")
	}
}

func TestGenerateTorrcAccountingOmittedWhenUnlimited(t *testing.T) {
	cfg := applyDefaults(Config{MaxMonthlyBandwidth: "unlimited"})

	content := generateTorrc(cfg, testLogFile)

	if strings.Contains(content, "AccountingMax") {
		t.Error("torrc must omit AccountingMax when MaxMonthlyBandwidth is 'unlimited'")
	}
}

func TestGenerateTorrcAccountingPresentWhenLimited(t *testing.T) {
	cfg := applyDefaults(Config{MaxMonthlyBandwidth: "50 GB"})

	content := generateTorrc(cfg, testLogFile)

	if !strings.Contains(content, "AccountingMax 50 GB") {
		t.Error("torrc must include the configured AccountingMax when a monthly limit is set")
	}
}

func TestGenerateTorrcIncludesLogFileDirective(t *testing.T) {
	cfg := applyDefaults(Config{})

	content := generateTorrc(cfg, "/var/log/airports/tor.log")

	want := "Log notice file /var/log/airports/tor.log"
	if !strings.Contains(content, want) {
		t.Errorf("torrc missing %q", want)
	}
}

func TestGenerateTorrcWiresCircuitAndStreamSettings(t *testing.T) {
	cfg := applyDefaults(Config{
		MaxCircuits:          64,
		CircuitTimeout:       90,
		MaxStreamsPerCircuit: 200,
		NumIntroPoints:       7,
	})

	content := generateTorrc(cfg, testLogFile)

	checks := []string{
		fmt.Sprintf("MaxClientCircuitsPending %d", 64),
		fmt.Sprintf("CircuitBuildTimeout %d", 90),
		fmt.Sprintf("HiddenServiceMaxStreams %d", 200),
		fmt.Sprintf("HiddenServiceNumIntroductionPoints %d", 7),
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("torrc missing %q\n---\n%s", want, content)
		}
	}
}
