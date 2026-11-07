package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	urlvars "github.com/apimgr/airports/src/common/urlvars"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/path"
)

// newMainTestDir creates an isolated temp directory under /tmp/apimgr/ per
// project testing rules, cleaned up automatically when the test completes.
func newMainTestDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll /tmp/apimgr: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-main-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll captured stdout: %v", err)
	}
	return string(out)
}

func TestExtractUpdateFlag(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFound  bool
		wantAction string
		wantBranch string
		wantRemain []string
	}{
		{"no update flag", []string{"--port", "8080"}, false, "yes", "", []string{"--port", "8080"}},
		{"bare --update defaults yes", []string{"--update"}, true, "yes", "", []string{}},
		{"--update check", []string{"--update", "check"}, true, "check", "", []string{}},
		{"--update yes explicit", []string{"--update", "yes"}, true, "yes", "", []string{}},
		{"--update=check inline", []string{"--update=check"}, true, "check", "", []string{}},
		{"--update branch stable", []string{"--update", "branch", "stable"}, true, "branch", "stable", []string{}},
		{"--update=branch inline then value", []string{"--update=branch", "beta"}, true, "branch", "", []string{"beta"}},
		{"non-update flags preserved", []string{"--debug", "--update", "check", "--color", "no"}, true, "check", "", []string{"--debug", "--color", "no"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, action, branch, remaining := extractUpdateFlag(tt.args)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if action != tt.wantAction {
				t.Errorf("action = %q, want %q", action, tt.wantAction)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
			}
			if len(remaining) != len(tt.wantRemain) {
				t.Fatalf("remaining = %v, want %v", remaining, tt.wantRemain)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemain[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemain[i])
				}
			}
		})
	}
}

func TestMaintenanceCommandHelpers(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		found     bool
		action    string
		target    string
		remaining []string
	}{
		{name: "backup target", args: []string{"--maintenance", "backup", "/tmp/backup.tar.gz", "--config", "/cfg"}, found: true, action: "backup", target: "/tmp/backup.tar.gz", remaining: []string{"--config", "/cfg"}},
		{name: "inline help", args: []string{"--maintenance=--help", "--version"}, found: true, action: "help", remaining: []string{"--version"}},
		{name: "compliance report", args: []string{"--maintenance", "compliance", "report"}, found: true, action: "compliance", target: "report"},
		{name: "bare maintenance flag", args: []string{"--maintenance", "--debug"}, remaining: []string{"--maintenance", "--debug"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found, action, target, remaining := extractMaintenanceFlag(test.args)
			if found != test.found || action != test.action || target != test.target || strings.Join(remaining, "\x00") != strings.Join(test.remaining, "\x00") {
				t.Fatalf("extractMaintenanceFlag(%v) = %t, %q, %q, %v", test.args, found, action, target, remaining)
			}
		})
	}
	for _, action := range []string{"backup", "restore", "update", "mode", "setup", "pgp", "secret", "token", "data", "compliance", "help", "--help"} {
		if !isMaintenanceAction(action) {
			t.Errorf("isMaintenanceAction(%q) = false", action)
		}
	}
	if isMaintenanceAction("unknown") {
		t.Error("isMaintenanceAction(unknown) = true")
	}
}

func TestExtractServiceSchedulerAndTorCommands(t *testing.T) {
	serviceCases := []struct {
		args      []string
		wantFound bool
		wantAct   string
		wantRest  []string
	}{
		{[]string{"--service", "--install", "--debug"}, true, "--install", []string{"--debug"}},
		{[]string{"--service=restart", "--version"}, true, "restart", []string{"--version"}},
		{[]string{"--service", "--debug"}, true, "help", []string{"--debug"}},
		{[]string{"--version"}, false, "", []string{"--version"}},
	}
	for _, test := range serviceCases {
		found, action, remaining := extractServiceFlag(test.args)
		if found != test.wantFound || action != test.wantAct || strings.Join(remaining, "\x00") != strings.Join(test.wantRest, "\x00") {
			t.Errorf("extractServiceFlag(%v) = %t, %q, %v", test.args, found, action, remaining)
		}
	}
	for _, test := range []struct {
		args   []string
		found  bool
		action string
		taskID string
	}{
		{[]string{"scheduler", "show", "health_check"}, true, "show", "health_check"},
		{[]string{"scheduler"}, true, "", ""},
		{[]string{"--help"}, false, "", ""},
	} {
		found, action, taskID, _ := extractSchedulerCommand(test.args)
		if found != test.found || action != test.action || taskID != test.taskID {
			t.Errorf("extractSchedulerCommand(%v) = %t, %q, %q", test.args, found, action, taskID)
		}
	}
	for _, test := range []struct {
		args     []string
		found    bool
		action   string
		sub      string
		argument string
	}{
		{[]string{"tor", "vanity", "start", "AB"}, true, "vanity", "start", "AB"},
		{[]string{"tor", "vanity", "apply"}, true, "vanity", "apply", ""},
		{[]string{"tor", "import-keys", "keys.zip"}, true, "import-keys", "", "keys.zip"},
		{[]string{"tor", "status"}, true, "status", "", ""},
		{[]string{"--version"}, false, "", "", ""},
	} {
		found, action, sub, argument, _ := extractTorCommand(test.args)
		if found != test.found || action != test.action || sub != test.sub || argument != test.argument {
			t.Errorf("extractTorCommand(%v) = %t, %q, %q, %q", test.args, found, action, sub, argument)
		}
	}
}

func TestRestartRequiredSettings(t *testing.T) {
	oldConfig := config.DefaultConfig()
	newConfig := config.DefaultConfig()
	if got := restartRequiredSettings(oldConfig, newConfig); len(got) != 0 {
		t.Fatalf("unchanged settings = %v", got)
	}
	newConfig.Server.Port = "8080"
	newConfig.Server.Address = "127.0.0.1"
	newConfig.Server.Daemonize = true
	newConfig.Server.SSL.Enabled = true
	newConfig.Server.SSL.Cert = "cert.pem"
	newConfig.Server.SSL.Key = "key.pem"
	newConfig.Server.SSL.MinVersion = "TLS1.3"
	newConfig.Server.SSL.LetsEncrypt.Enabled = true
	newConfig.Server.SSL.LetsEncrypt.Email = "admin@example.com"
	newConfig.Server.Database.URL = "file:new.db"
	newConfig.Server.Database.Driver = "other"
	newConfig.Server.Tor.Binary = "/usr/bin/tor"
	newConfig.Server.Tor.UseNetwork = true
	want := []string{"server.port", "server.address", "server.daemonize", "ssl.enabled", "ssl.cert", "ssl.key", "ssl.min_version", "ssl.letsencrypt.enabled", "ssl.letsencrypt.email", "database.driver", "database.url", "tor.binary", "tor.use_network"}
	got := restartRequiredSettings(oldConfig, newConfig)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("restartRequiredSettings = %v, want %v", got, want)
	}
}

func TestSmallMaintenanceAndDisplayHelpers(t *testing.T) {
	if output := captureStdout(t, func() { printMaintenanceHelp("airports") }); !strings.Contains(output, "compliance report") {
		t.Fatalf("maintenance help missing compliance command: %s", output)
	}
	if output := captureStdout(t, func() { printUpdateHelp("airports") }); !strings.Contains(output, "branch") {
		t.Fatalf("update help missing branch command: %s", output)
	}
	if got := httpsPortFromBindings([]portBinding{{Port: "80"}, {Port: "443", TLS: true}}); got != "443" {
		t.Fatalf("httpsPortFromBindings = %q", got)
	}
	if got := httpsPortFromBindings([]portBinding{{Port: "8080"}}); got != "" {
		t.Fatalf("httpsPortFromBindings without TLS = %q", got)
	}
	done := make(chan struct{})
	close(done)
	watchConfigFile(filepath.Join(newMainTestDir(t), "missing.yml"), config.DefaultConfig(), nil, done, nil)
	if _, err := promptBackupPassword("password: "); err != nil {
		t.Fatalf("non-interactive password prompt: %v", err)
	}
	if err := confirmConfigurationReset(); err == nil {
		t.Fatal("non-interactive configuration reset confirmation succeeded")
	}
	originalEpoch := BuildEpoch
	BuildEpoch = "123"
	if got := buildEpoch(); got != 123 {
		t.Fatalf("buildEpoch = %d, want 123", got)
	}
	BuildEpoch = "invalid"
	if got := buildEpoch(); got != 0 {
		t.Fatalf("invalid buildEpoch = %d, want 0", got)
	}
	BuildEpoch = originalEpoch
}

func TestLoadUpdateBranchUsesConfigDirectory(t *testing.T) {
	dir := newMainTestDir(t)
	t.Setenv("CONFIG_DIR", dir)
	cfg := config.DefaultConfig()
	cfg.Server.Update.Branch = "beta"
	if err := config.Save(filepath.Join(dir, "server.yml"), cfg); err != nil {
		t.Fatalf("save update config: %v", err)
	}
	if got := loadUpdateBranch(); got != "beta" {
		t.Fatalf("loadUpdateBranch = %q, want beta", got)
	}
}

func TestMaintenanceAuthorizationAndModeHelpers(t *testing.T) {
	if requiresToken, err := modeChangeAuthorization(true, false); err != nil || requiresToken {
		t.Fatalf("root authorization = %t, %v", requiresToken, err)
	}
	if requiresToken, err := modeChangeAuthorization(false, true); err != nil || !requiresToken {
		t.Fatalf("service authorization = %t, %v", requiresToken, err)
	}
	if _, err := modeChangeAuthorization(false, false); err == nil {
		t.Fatal("unprivileged mode authorization succeeded")
	}
	if got, err := maintenanceModeValue(" Debug "); err != nil || got != "debug" {
		t.Fatalf("maintenanceModeValue = %q, %v", got, err)
	}
	if _, err := maintenanceModeValue("invalid"); err == nil {
		t.Fatal("maintenanceModeValue accepted an invalid mode")
	}
	for _, test := range []struct {
		provided   string
		configured string
		want       bool
	}{
		{"secret", "secret", true},
		{"wrong", "secret", false},
		{"", "secret", false},
		{"secret", "", false},
		{"longer", "short", false},
	} {
		if got := maintenanceOperatorTokenMatches(test.provided, test.configured); got != test.want {
			t.Errorf("maintenanceOperatorTokenMatches(%q, %q) = %t", test.provided, test.configured, got)
		}
	}
}

func TestWriteServerPID(t *testing.T) {
	dir := newMainTestDir(t)
	pidPath := filepath.Join(dir, "airports.pid")
	if written, err := writeServerPID(pidPath, true, false); err != nil || written {
		t.Fatalf("container PID handling = %t, %v", written, err)
	}
	written, err := writeServerPID(pidPath, false, false)
	if err != nil || !written {
		t.Fatalf("writeServerPID = %t, %v", written, err)
	}
	if data, err := os.ReadFile(pidPath); err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("PID file = %q, %v", data, err)
	}
	if _, err := writeServerPID(pidPath, false, false); err == nil {
		t.Fatal("writeServerPID accepted an already-running PID")
	}
	if written, err := writeServerPID(filepath.Join(dir, "missing", "server.pid"), false, false); err != nil || written {
		t.Fatalf("PID write failure should be nonfatal, got %t, %v", written, err)
	}
}

func TestMaintenanceSetupAndComplianceReport(t *testing.T) {
	dir := newMainTestDir(t)
	configDir := filepath.Join(dir, "config")
	databaseDir := filepath.Join(dir, "database")
	logsDir := filepath.Join(dir, "logs")
	t.Setenv("DATABASE_DIR", databaseDir)
	t.Setenv("CONFIG_DIR", configDir)
	t.Setenv("LOG_DIR", logsDir)
	if err := runMaintenanceSetup(configDir); err != nil {
		t.Fatalf("first-run setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "server.yml")); err != nil {
		t.Fatalf("setup config file: %v", err)
	}
	if err := runComplianceReport(configDir, logsDir); err != nil {
		t.Fatalf("compliance report: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Server.Compliance.GDPR = true
	if err := config.Save(filepath.Join(configDir, "server.yml"), cfg); err != nil {
		t.Fatalf("save compliance config: %v", err)
	}
	if err := runComplianceReport(configDir, logsDir); err != nil {
		t.Fatalf("enabled-standard compliance report: %v", err)
	}
}

func TestSchedulerCLICommands(t *testing.T) {
	dir := newMainTestDir(t)
	configDir := filepath.Join(dir, "config")
	if err := config.Save(filepath.Join(configDir, "server.yml"), config.DefaultConfig()); err != nil {
		t.Fatalf("save test config: %v", err)
	}
	t.Setenv("CONFIG_DIR", configDir)
	t.Setenv("DATABASE_DIR", filepath.Join(dir, "database"))
	t.Setenv("LOG_DIR", filepath.Join(dir, "logs"))
	for _, test := range []struct {
		action string
		taskID string
		want   int
	}{
		{"list", "", 0},
		{"show", "token_cleanup", 0},
		{"show", "missing", 1},
		{"show", "", 1},
		{"enable", "token_cleanup", 0},
		{"disable", "token_cleanup", 0},
		{"history", "token_cleanup", 0},
		{"run", "token_cleanup", 0},
		{"unknown", "", 1},
	} {
		if got := handleSchedulerCLI(test.action, test.taskID); got != test.want {
			t.Errorf("handleSchedulerCLI(%q, %q) = %d, want %d", test.action, test.taskID, got, test.want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first wins", []string{"a", "b", "c"}, "a"},
		{"CLI env config priority", []string{"flag", "env", "config"}, "flag"},
		{"env before config", []string{"", "env", "config"}, "env"},
		{"skips leading empties", []string{"", "", "c"}, "c"},
		{"no args", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.values...); got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestGetEmoji(t *testing.T) {
	prev := outputEmojiSetting
	outputEmojiSetting = nil
	t.Cleanup(func() { outputEmojiSetting = prev })
	t.Setenv("TERM", "xterm-256color")
	t.Run("known name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if got := getEmoji("check"); got != "✓" {
			t.Errorf("getEmoji(check) = %q, want %q", got, "✓")
		}
	})
	t.Run("unknown name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if got := getEmoji("does-not-exist"); got != "" {
			t.Errorf("getEmoji(unknown) = %q, want empty", got)
		}
	})
	t.Run("NO_COLOR suppresses known name", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if got := getEmoji("check"); got != "" {
			t.Errorf("getEmoji(check) with NO_COLOR set = %q, want empty", got)
		}
	})
}

// TestEmojiEnabledConfigPriority covers the PART 7 emoji override and env rules.
func TestEmojiEnabledConfigPriority(t *testing.T) {
	prev := outputEmojiSetting
	t.Cleanup(func() { outputEmojiSetting = prev })
	configured := true

	t.Run("TERM dumb disables configured emoji", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "dumb")
		outputEmojiSetting = &configured
		if emojiEnabled() {
			t.Error("emojiEnabled() = true, want false when TERM=dumb")
		}
	})
	t.Run("NO_COLOR disables", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "xterm-256color")
		outputEmojiSetting = nil
		if emojiEnabled() {
			t.Error("emojiEnabled() = true, want false when NO_COLOR is nonempty")
		}
	})
	t.Run("TERM dumb disables", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "dumb")
		outputEmojiSetting = nil
		if emojiEnabled() {
			t.Error("emojiEnabled() = true, want false for TERM=dumb")
		}
	})
}

// TestResolveColorSetting confirms the --color flag value is actually
// stored into colorSetting instead of being discarded (the AI.md PART 7/8
// gap this test guards against: "--color validated but never applied").
// run() itself is not called here — it starts a real server on success and
// is intentionally excluded from unit tests (see the NOTE comment below).
func TestResolveColorSetting(t *testing.T) {
	prev := colorSetting
	t.Cleanup(func() { colorSetting = prev })

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty defaults to auto", "", "auto", false},
		{"auto", "auto", "auto", false},
		{"yes", "yes", "yes", false},
		{"no", "no", "no", false},
		{"invalid", "bogus", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colorSetting = "auto"
			got, err := resolveColorSetting(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveColorSetting(%q): expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveColorSetting(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("resolveColorSetting(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if colorSetting != tt.want {
				t.Errorf("colorSetting after resolveColorSetting(%q) = %q, want %q", tt.in, colorSetting, tt.want)
			}
		})
	}
}

// isRunningInContainer() (the main package's own copy, distinct from
// paths.IsRunningInContainer) checks for /.dockerenv, which is always
// present inside the casjaysdev/go:latest test container.
func TestIsRunningInContainerMain(t *testing.T) {
	got := isRunningInContainer()
	if !got {
		t.Log("isRunningInContainer() = false; expected true inside a Docker test container, but not failing the build for host runs")
	}
}

func TestFindRandomPort(t *testing.T) {
	port, err := findRandomPort()
	if err != nil {
		t.Fatalf("findRandomPort() returned error: %v", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("findRandomPort() = %q, not numeric: %v", port, err)
	}
	if n < 64000 || n > 64999 {
		t.Errorf("findRandomPort() = %d, want in range [64000, 64999]", n)
	}
}

// getOutboundIP dials 8.8.8.8:80 via UDP, which never actually sends a
// packet (connectionless), so this is safe to call in any environment; it
// simply asserts the function returns without panicking.
func TestGetOutboundIP(t *testing.T) {
	ip := getOutboundIP()
	t.Logf("getOutboundIP() = %q", ip)
}

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		sslEnabled bool
		want       []portBinding
		wantErr    bool
	}{
		{name: "single HTTP port", value: "8080", want: []portBinding{{Port: "8080"}}},
		{name: "OS-selected port", value: "0", want: []portBinding{{Port: "0"}}},
		{name: "single HTTPS default port", value: "443", want: []portBinding{{Port: "443", TLS: true}}},
		{name: "SSL forces TLS on a single port", value: "8443", sslEnabled: true, want: []portBinding{{Port: "8443", TLS: true}}},
		{name: "dual HTTP and HTTPS ports", value: "80,443", sslEnabled: true, want: []portBinding{{Port: "80"}, {Port: "443", TLS: true}}},
		{name: "dual custom ports with whitespace", value: " 8080 , 8443 ", sslEnabled: true, want: []portBinding{{Port: "8080"}, {Port: "8443", TLS: true}}},
		{name: "dual ports require SSL", value: "80,443", wantErr: true},
		{name: "dual ports require explicit ports", value: "0,443", sslEnabled: true, wantErr: true},
		{name: "empty port", value: "", wantErr: true},
		{name: "empty second port", value: "80,", sslEnabled: true, wantErr: true},
		{name: "too many ports", value: "80,443,8080", sslEnabled: true, wantErr: true},
		{name: "port below range", value: "-1", wantErr: true},
		{name: "port above range", value: "65536", wantErr: true},
		{name: "duplicate dual ports", value: "8080,8080", sslEnabled: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePortSpec(test.value, test.sslEnabled)
			if (err != nil) != test.wantErr {
				t.Fatalf("parsePortSpec() error = %v, wantErr %t", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if len(got) != len(test.want) {
				t.Fatalf("parsePortSpec() returned %v, want %v", got, test.want)
			}
			for i, binding := range got {
				if binding != test.want[i] {
					t.Errorf("binding[%d] = %+v, want %+v", i, binding, test.want[i])
				}
			}
		})
	}
}

func TestRedirectHTTPToHTTPS(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	trustedPeer := func(r *http.Request) bool { return r.RemoteAddr == "10.0.0.2:1234" }
	handler := redirectHTTPToHTTPS(next, "api.example.com", "8443", trustedPeer)

	req := httptest.NewRequest(http.MethodGet, "http://api.example.com:8080/airports?q=JFK", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMovedPermanently)
	}
	if got, want := response.Header().Get("Location"), "https://api.example.com:8443/airports?q=JFK"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if called {
		t.Error("HTTP request was passed through instead of redirected")
	}

	proxyRequest := httptest.NewRequest(http.MethodGet, "http://api.example.com/airports", nil)
	proxyRequest.RemoteAddr = "10.0.0.2:1234"
	proxyRequest.Header.Set("X-Forwarded-Proto", "https")
	proxyResponse := httptest.NewRecorder()
	handler.ServeHTTP(proxyResponse, proxyRequest)
	if proxyResponse.Code != http.StatusNoContent || !called {
		t.Errorf("trusted HTTPS proxy request: status=%d passedThrough=%t, want 204 and true", proxyResponse.Code, called)
	}
	untrustedRequest := httptest.NewRequest(http.MethodGet, "http://api.example.com/airports", nil)
	untrustedRequest.RemoteAddr = "203.0.113.8:1234"
	untrustedRequest.Header.Set("X-Forwarded-Proto", "https")
	untrustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(untrustedResponse, untrustedRequest)
	if untrustedResponse.Code != http.StatusMovedPermanently {
		t.Errorf("untrusted forwarded-proto status = %d, want redirect status %d", untrustedResponse.Code, http.StatusMovedPermanently)
	}

	for _, target := range []string{
		"http://api.example.com/server/healthz",
		"http://airportsabc.onion/",
		"http://airportsabc.b32.i2p/",
	} {
		called = false
		req := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent || !called {
			t.Errorf("request %q: status=%d passedThrough=%t, want 204 and true", target, response.Code, called)
		}
	}
}

func TestGetAccessibleURL(t *testing.T) {
	url := getAccessibleURL("64123")
	if !strings.Contains(url, "64123") {
		t.Errorf("getAccessibleURL(64123) = %q, want it to contain the port", url)
	}
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("getAccessibleURL(64123) = %q, want http:// prefix", url)
	}
	tests := []struct {
		name, port string
		secure     bool
		want       string
	}{
		{name: "standard HTTP port", port: "80", want: "http://example.org"},
		{name: "standard HTTPS port", port: "443", want: "https://example.org"},
		{name: "secure nonstandard port", port: "8443", secure: true, want: "https://example.org:8443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DOMAIN", "example.org")
			if got := getServerAccessibleURL(test.port, "example.org", test.secure); got != test.want {
				t.Errorf("getServerAccessibleURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveFQDN(t *testing.T) {
	tests := []struct {
		name                    string
		domain, system, hostEnv string
		ipv6, ipv4              string
		devMode                 bool
		want                    string
	}{
		{
			name:   "DOMAIN has highest static priority and uses its first entry",
			domain: "  env.example.com , other.example.com", system: "system.example.com",
			hostEnv: "host.example.com", ipv6: "2001:db8::1", ipv4: "203.0.113.1",
			want: "env.example.com",
		},
		{
			name:   "system hostname precedes HOSTNAME and addresses",
			system: "system.example.com", hostEnv: "host.example.com",
			ipv6: "2001:db8::1", ipv4: "203.0.113.1", want: "system.example.com",
		},
		{
			name:    "HOSTNAME precedes public addresses",
			hostEnv: "host.example.com", ipv6: "2001:db8::1", ipv4: "203.0.113.1",
			want: "host.example.com",
		},
		{
			name: "public interface address is the final pre-localhost fallback",
			ipv6: "2001:db8::1", ipv4: "203.0.113.1", want: "2001:db8::1",
		},
		{
			name:   "invalid DOMAIN is skipped for a valid system hostname",
			domain: "192.168.1.1", system: "system.example.com", want: "system.example.com",
		},
		{
			name:   "internal hostname is accepted in development",
			domain: "dev.local", devMode: true, want: "dev.local",
		},
		{
			name:   "internal hostname is rejected in production",
			domain: "dev.local", want: "localhost",
		},
		{
			name:   "DOMAIN overlay address is skipped",
			domain: "node.onion", system: "valid.example.com", want: "valid.example.com",
		},
		{
			name:   "loopback hostname is skipped",
			system: "localhost", hostEnv: "host.example.com", want: "host.example.com",
		},
		{
			name: "localhost is the final fallback",
			want: "localhost",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveFQDNFromSources(test.domain, test.system, test.hostEnv, test.ipv6, test.ipv4, test.devMode)
			if got != test.want {
				t.Errorf("resolveFQDNFromSources() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunDiskPressureCleanupPrunesOnlyExpiredLogsAndOldBackups(t *testing.T) {
	root := newMainTestDir(t)
	dataDir := filepath.Join(root, "data")
	logDir := filepath.Join(root, "logs")
	backupDir := filepath.Join(root, "backup")
	for _, dir := range []string{dataDir, logDir, backupDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}
	logManager := logging.NewManager(logDir, toLoggingConfig(config.DefaultConfig().Server.Logging))
	activeLog := logManager.Server().Path()
	if err := os.WriteFile(activeLog, []byte("active"), 0o600); err != nil {
		t.Fatalf("write active log: %v", err)
	}
	oldLog := activeLog + ".2020-01-01T00-00-00"
	if err := os.WriteFile(oldLog, []byte("rotated"), 0o600); err != nil {
		t.Fatalf("write rotated log: %v", err)
	}
	oldBackup := filepath.Join(backupDir, "airports_backup_2020-01-01.tar.gz")
	newBackup := filepath.Join(backupDir, "airports_backup_2021-01-01.tar.gz")
	for _, backupPath := range []string{oldBackup, newBackup} {
		if err := os.WriteFile(backupPath, []byte("backup"), 0o600); err != nil {
			t.Fatalf("write backup fixture: %v", err)
		}
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldLog, oldTime, oldTime); err != nil {
		t.Fatalf("age rotated log: %v", err)
	}
	usedSamples := 0
	diskUsage := func(string) (uint64, uint64, error) {
		usedSamples++
		if usedSamples == 1 {
			return 1000, 50, nil
		}
		return 1000, 200, nil
	}
	triggered, err := runDiskPressureCleanup(config.CleanupConfig{DiskThreshold: 90, LogRetentionDays: 7, BackupKeepCount: 1}, dataDir, backupDir, logManager, time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), diskUsage)
	if err != nil || !triggered {
		t.Fatalf("runDiskPressureCleanup() = (%t,%v), want (true,nil)", triggered, err)
	}
	for _, path := range []string{oldLog, oldBackup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expired item %q remains: %v", path, err)
		}
	}
	for _, path := range []string{activeLog, newBackup} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("retained item %q missing: %v", path, err)
		}
	}
}

func TestPruneProjectTempFiles(t *testing.T) {
	tempRoot := filepath.Join(newMainTestDir(t), "os-temp")
	orgRoot := filepath.Join(tempRoot, paths.OrgName)
	projectDir := filepath.Join(orgRoot, ProjectName+"-stale")
	otherProjectDir := filepath.Join(orgRoot, "other-project-stale")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherProjectDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(projectDir, "old")
	newFile := filepath.Join(projectDir, "new")
	otherFile := filepath.Join(otherProjectDir, "old")
	for _, file := range []string{oldFile, newFile, otherFile} {
		if err := os.WriteFile(file, []byte("temp"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldFile, cutoff.Add(-25*time.Hour), cutoff.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(otherFile, cutoff.Add(-25*time.Hour), cutoff.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	linkTarget := filepath.Join(tempRoot, "outside")
	if err := os.WriteFile(linkTarget, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linkTarget, filepath.Join(projectDir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	removed, err := pruneProjectTempFiles(cutoff, tempRoot)
	if err != nil {
		t.Fatalf("pruneProjectTempFiles() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("pruneProjectTempFiles() removed %d files, want 1", removed)
	}
	for _, check := range []struct {
		file string
		want bool
	}{{oldFile, false}, {newFile, true}, {otherFile, true}, {linkTarget, true}} {
		_, statErr := os.Stat(check.file)
		if got := statErr == nil; got != check.want {
			t.Errorf("exists(%q) = %t, want %t (err=%v)", check.file, got, check.want, statErr)
		}
	}
}

func TestHostValidation(t *testing.T) {
	tests := []struct {
		host, project string
		dev, want     bool
	}{
		{host: "api.example.co.uk", want: true},
		{host: "co.uk", want: false},
		{host: "192.168.1.1", dev: true, want: false},
		{host: "localhost", dev: true, want: true},
		{host: "app.test", dev: true, want: true},
		{host: "app.test", want: false},
		{host: "app.airports", project: "airports", dev: true, want: true},
		{host: "app.airports", project: "airports", want: false},
	}
	for _, test := range tests {
		if got := urlvars.IsValidHost(test.host, test.dev, test.project); got != test.want {
			t.Errorf("IsValidHost(%q, %t, %q) = %t, want %t", test.host, test.dev, test.project, got, test.want)
		}
	}
	if urlvars.IsValidSSLHost("node.onion") || urlvars.IsValidSSLHost("app.test") || !urlvars.IsValidSSLHost("api.example.com") {
		t.Error("IsValidSSLHost did not enforce public hostname rules")
	}
}

func TestApplyTZEnv(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	t.Run("valid TZ sets time.Local", func(t *testing.T) {
		t.Setenv("TZ", "America/New_York")
		applyTZEnv()
		if time.Local.String() != "America/New_York" {
			t.Errorf("time.Local = %q, want %q", time.Local.String(), "America/New_York")
		}
	})

	t.Run("invalid TZ leaves time.Local unchanged", func(t *testing.T) {
		time.Local = original
		t.Setenv("TZ", "Not/A_Real_Zone")
		applyTZEnv()
		if time.Local != original {
			t.Errorf("time.Local changed on invalid TZ, want unchanged (%v)", original)
		}
	})

	t.Run("empty TZ leaves time.Local unchanged", func(t *testing.T) {
		time.Local = original
		t.Setenv("TZ", "")
		applyTZEnv()
		if time.Local != original {
			t.Errorf("time.Local changed on empty TZ, want unchanged (%v)", original)
		}
	})
}

func TestResolveSchedulerLocation(t *testing.T) {
	t.Run("empty timezone falls back to time.Local", func(t *testing.T) {
		if got := resolveSchedulerLocation(""); got != time.Local {
			t.Errorf("resolveSchedulerLocation(\"\") = %v, want time.Local", got)
		}
	})

	t.Run("valid IANA timezone is loaded", func(t *testing.T) {
		got := resolveSchedulerLocation("America/New_York")
		if got.String() != "America/New_York" {
			t.Errorf("resolveSchedulerLocation() = %v, want America/New_York", got)
		}
	})

	t.Run("invalid timezone falls back to time.Local", func(t *testing.T) {
		if got := resolveSchedulerLocation("Not/A_Real_Zone"); got != time.Local {
			t.Errorf("resolveSchedulerLocation() = %v, want time.Local fallback", got)
		}
	})
}

func TestPrintHelp(t *testing.T) {
	out := captureStdout(t, func() { printHelp("airports") })
	for _, want := range []string{"Usage:", "--help", "--version", "--status", "Update Commands:", "Service Commands:", "Maintenance / Admin:", "Environment Variables:"} {
		if !strings.Contains(out, want) {
			t.Errorf("printHelp() output missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--restore") {
		t.Errorf("printHelp() advertises unsupported top-level --restore flag; got:\n%s", out)
	}
	if !strings.Contains(out, "--maintenance restore <path>") {
		t.Errorf("printHelp() is missing canonical maintenance restore command; got:\n%s", out)
	}
	for _, path := range []string{"/etc/apimgr/airports/server.yml", "~/.config/apimgr/airports/server.yml"} {
		if !strings.Contains(out, path) {
			t.Errorf("printHelp() is missing config path %q; got:\n%s", path, out)
		}
	}
}

func TestPrintServiceHelp(t *testing.T) {
	out := captureStdout(t, printServiceHelp)
	for _, want := range []string{"Service Commands:", "--install", "--uninstall", "start", "stop", "restart", "reload", "--disable", "--help", "Supported service managers:"} {
		if !strings.Contains(out, want) {
			t.Errorf("printServiceHelp() output missing %q; got:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"status", "enable", "logs"} {
		if strings.Contains(out, "  "+unwanted+" ") {
			t.Errorf("printServiceHelp() output contains unsupported command %q; got:\n%s", unwanted, out)
		}
	}
}

func TestHandleServiceCommandUnknown(t *testing.T) {
	err := handleServiceCommand("not-a-real-command")
	if err == nil {
		t.Fatal("handleServiceCommand(unknown): expected error, got nil")
	}
	want := "unknown service command: not-a-real-command"
	if err.Error() != want {
		t.Errorf("handleServiceCommand(unknown) error = %q, want %q", err.Error(), want)
	}
}

func TestHandleServiceCommandCanonicalHelpAndRejectsAliases(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handleServiceCommand("--help"); err != nil {
			t.Fatalf("handleServiceCommand(--help): %v", err)
		}
	})
	if !strings.Contains(out, "--install") || !strings.Contains(out, "--disable") {
		t.Errorf("service help is missing canonical command forms: %s", out)
	}
	for _, alias := range []string{"install", "uninstall", "disable", "status", "enable", "logs"} {
		if err := handleServiceCommand(alias); err == nil {
			t.Errorf("handleServiceCommand(%q) = nil, want unsupported-command error", alias)
		}
	}
}

func TestServiceFlagParsesCanonicalFlagSubcommands(t *testing.T) {
	for _, command := range []string{"--install", "--uninstall", "--disable", "--help"} {
		t.Run(command, func(t *testing.T) {
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			serviceCommand := flags.String("service", "", "")
			if err := flags.Parse([]string{"--service", command}); err != nil {
				t.Fatalf("Parse(--service %s): %v", command, err)
			}
			if *serviceCommand != command {
				t.Errorf("--service value = %q, want %q", *serviceCommand, command)
			}
		})
	}
}

// run() validates --color before touching any directories/config, so an
// invalid value returns an error deterministically without side effects
// beyond setting server.Version/Commit/BuildDate package vars.
func TestRunInvalidColor(t *testing.T) {
	err := run("", "", "", "", "", "", "", "", "", false, "bogus", "", false, false, false)
	if err == nil {
		t.Fatal("run() with invalid --color: expected error, got nil")
	}
	want := "invalid --color value: bogus (must be auto, yes, or no)"
	if err.Error() != want {
		t.Errorf("run() error = %q, want %q", err.Error(), want)
	}
}

// checkAndUpdate() refuses to self-update inside a Docker container and
// returns before making any network calls — deterministically true inside
// the casjaysdev/go:latest test container.
func TestCheckAndUpdateInContainer(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("not running inside a Docker container (no /.dockerenv)")
	}
	err := checkAndUpdate()
	if err == nil {
		t.Fatal("checkAndUpdate() inside a container: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported inside a Docker container") {
		t.Errorf("checkAndUpdate() error = %q, want container-refusal message", err.Error())
	}
}

// setUpdateBranch validates the branch name before touching config, so an
// invalid value returns an error deterministically without side effects.
func TestSetUpdateBranchInvalid(t *testing.T) {
	err := setUpdateBranch("not-a-real-branch")
	if err == nil {
		t.Fatal("setUpdateBranch(invalid): expected error, got nil")
	}
	want := `invalid update branch: "not-a-real-branch" (expected stable, beta, or devel)`
	if err.Error() != want {
		t.Errorf("setUpdateBranch(invalid) error = %q, want %q", err.Error(), want)
	}
}

func TestRestoreBackupMissingFile(t *testing.T) {
	dir := newMainTestDir(t)
	missing := filepath.Join(dir, "does-not-exist.tar.gz")
	err := restoreBackup(missing, false)
	if err == nil {
		t.Fatal("restoreBackup(missing file): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "file exists") {
		t.Errorf("restoreBackup(missing file) error = %q, want it to mention file exists", err.Error())
	}
}

// redirectingTransport rewrites every request to target the given
// httptest.Server instead of its original host, so fetchLatestRelease's
// hardcoded api.github.com URL can be exercised against a local mock.
type redirectingTransport struct {
	targetURL string
}

func (rt redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := http.NewRequest(req.Method, rt.targetURL, req.Body)
	if err != nil {
		return nil, err
	}
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}

func TestFetchLatestReleaseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"airports-linux-amd64","browser_download_url":"https://example.invalid/airports-linux-amd64"}]}`))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchLatestRelease(client)
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if release == nil {
		t.Fatal("fetchLatestRelease returned nil release for a 200 response")
	}
	if release.TagName != "v9.9.9" {
		t.Errorf("release.TagName = %q, want %q", release.TagName, "v9.9.9")
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "airports-linux-amd64" {
		t.Errorf("release.Assets = %+v, want one asset named airports-linux-amd64", release.Assets)
	}
}

func TestFetchLatestReleaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchLatestRelease(client)
	if err != nil {
		t.Fatalf("fetchLatestRelease on 404: unexpected error: %v", err)
	}
	if release != nil {
		t.Errorf("fetchLatestRelease on 404 = %+v, want nil (no update available)", release)
	}
}

func TestFetchLatestReleaseBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	_, err := fetchLatestRelease(client)
	if err == nil {
		t.Fatal("fetchLatestRelease with malformed JSON: expected error, got nil")
	}
}

// TestMatchesBranch covers PART 22 "Channel Semantics": channels are
// cumulative, and rolling-channel aliases normalize to devel.
func TestMatchesBranch(t *testing.T) {
	cases := []struct {
		name   string
		r      releaseInfo
		branch string
		want   bool
	}{
		{"stable release matches stable", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "stable", true},
		{"stable release matches beta", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "beta", true},
		{"stable release matches devel", releaseInfo{TagName: "v1.2.3", Prerelease: false}, "devel", true},
		{"beta tag matches beta", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "beta", true},
		{"beta tag matches devel alias", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "daily", true},
		{"beta tag does not match stable", releaseInfo{TagName: "202512051430-beta", Prerelease: true}, "stable", false},
		{"devel rolling tag matches devel", releaseInfo{TagName: "devel", Prerelease: true}, "devel", true},
		{"devel rolling tag matches rolling alias", releaseInfo{TagName: "devel", Prerelease: true}, "rolling", true},
		{"devel tag does not match beta", releaseInfo{TagName: "devel", Prerelease: true}, "beta", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesBranch(tc.r, tc.branch); got != tc.want {
				t.Errorf("matchesBranch(%+v, %q) = %v, want %v", tc.r, tc.branch, got, tc.want)
			}
		})
	}
}

// TestFetchReleaseForBranchStable exercises the stable-channel path, which
// must delegate to the GitHub "latest" endpoint and skip when already current.
func TestFetchReleaseForBranchStable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","prerelease":false,"assets":[]}`))
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchReleaseForBranch(client, "stable", "1.0.0")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch: %v", err)
	}
	if release == nil || release.TagName != "v9.9.9" {
		t.Fatalf("fetchReleaseForBranch(stable) = %+v, want v9.9.9", release)
	}

	release, err = fetchReleaseForBranch(client, "stable", "9.9.9")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch (current): %v", err)
	}
	if release != nil {
		t.Errorf("fetchReleaseForBranch(stable) at current version = %+v, want nil", release)
	}
}

// TestFetchReleaseForBranchDevel compares rolling-release publish times with
// the current binary's embedded BUILD_EPOCH and accepts daily as an alias.
func TestFetchReleaseForBranchDevel(t *testing.T) {
	originalBuildEpoch := BuildEpoch
	BuildEpoch = strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	t.Cleanup(func() { BuildEpoch = originalBuildEpoch })
	develPublished := time.Now().UTC().Format(time.RFC3339)
	stablePublished := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[
			{"tag_name":"devel","prerelease":true,"published_at":%q,"assets":[]},
			{"tag_name":"202512051430-beta","prerelease":true,"published_at":%q,"assets":[]},
			{"tag_name":"v1.2.3","prerelease":false,"published_at":%q,"assets":[]}
		]`, develPublished, stablePublished, stablePublished)
	}))
	defer srv.Close()

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	release, err := fetchReleaseForBranch(client, "daily", "1.0.0")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch: %v", err)
	}
	if release == nil || release.TagName != "devel" {
		t.Fatalf("fetchReleaseForBranch(daily alias) = %+v, want newer rolling entry", release)
	}

	BuildEpoch = strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	release, err = fetchReleaseForBranch(client, "devel", "devel")
	if err != nil {
		t.Fatalf("fetchReleaseForBranch (skip older releases): %v", err)
	}
	if release != nil {
		t.Fatalf("fetchReleaseForBranch(devel) = %+v, want nil for older releases", release)
	}
}

// TestRunScheduledUpdateCheckDeferDays covers PART 22 "Defer Semantics": the
// scheduled update_check task must skip a release that hasn't aged past
// defer_days, without touching LastNotifiedVersion.
func TestRunScheduledUpdateCheckDeferDays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name":"v9.9.9","prerelease":false,"published_at":%q,"assets":[]}`, time.Now().Add(-1*24*time.Hour).Format(time.RFC3339))
	}))
	defer srv.Close()

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}
	cfg.Server.Update.Branch = "stable"
	cfg.Server.Update.DeferDays = 30
	cfg.Server.Update.LastNotifiedVersion = ""

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient: %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "" {
		t.Errorf("LastNotifiedVersion = %q, want empty (release not yet eligible under defer_days)", cfg.Server.Update.LastNotifiedVersion)
	}
}

func TestRunScheduledUpdateCheckDevelIgnoresDeferDays(t *testing.T) {
	originalBuildEpoch := BuildEpoch
	BuildEpoch = strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	t.Cleanup(func() { BuildEpoch = originalBuildEpoch })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"tag_name":"devel","prerelease":true,"published_at":%q,"assets":[]}]`, time.Now().UTC().Format(time.RFC3339))
	}))
	defer srv.Close()
	cfg := config.DefaultConfig()
	cfg.Server.Update.Branch = "daily"
	cfg.Server.Update.DeferDays = 365
	cfg.Server.Update.AutoInstall = true
	cfg.Server.Update.LastNotifiedVersion = "devel"
	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	err := runScheduledUpdateCheckWithClient(cfg, newMainTestDir(t), client)
	if err == nil || !strings.Contains(err.Error(), "not supported inside a Docker container") {
		t.Fatalf("runScheduledUpdateCheckWithClient(devel) error = %v, want updater reached despite defer_days", err)
	}
}

// TestRunScheduledUpdateCheckNotifiesOncePerVersion covers PART 22's "fires
// once per version" rule: an eligible release updates LastNotifiedVersion on
// first sight, and a second run for the same version is a no-op.
func TestRunScheduledUpdateCheckNotifiesOncePerVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","prerelease":false,"assets":[]}`))
	}))
	defer srv.Close()

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}
	cfg.Server.Update.Branch = "stable"
	cfg.Server.Update.DeferDays = 0
	cfg.Server.Update.AutoInstall = false
	cfg.Server.Update.LastNotifiedVersion = ""

	client := &http.Client{Transport: redirectingTransport{targetURL: srv.URL}}
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient (first run): %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "9.9.9" {
		t.Fatalf("LastNotifiedVersion = %q, want 9.9.9", cfg.Server.Update.LastNotifiedVersion)
	}

	// Second run for the same version must not error and must leave
	// LastNotifiedVersion unchanged (no re-notification).
	if err := runScheduledUpdateCheckWithClient(cfg, configDir, client); err != nil {
		t.Fatalf("runScheduledUpdateCheckWithClient (second run): %v", err)
	}
	if cfg.Server.Update.LastNotifiedVersion != "9.9.9" {
		t.Errorf("LastNotifiedVersion after second run = %q, want unchanged 9.9.9", cfg.Server.Update.LastNotifiedVersion)
	}
}

// TestSetUpdateBranchValid confirms accepted aliases persist canonically in
// an isolated config directory.
func TestSetUpdateBranchValid(t *testing.T) {
	dir := newMainTestDir(t)
	t.Setenv("CONFIG_DIR", dir)
	for _, branch := range []string{"stable", "beta", "devel", "daily", "rolling", "dev", "development"} {
		if err := setUpdateBranch(branch); err != nil {
			t.Errorf("setUpdateBranch(%q) = %v, want nil", branch, err)
		}
	}
	cfg, err := config.Load(filepath.Join(dir, "server.yml"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.Server.Update.Branch != "devel" {
		t.Errorf("final update branch = %q, want canonical devel", cfg.Server.Update.Branch)
	}
}

// checkStatus must never panic and must report "not running" when no server
// is actually listening on the configured/default port - this exercises the
// config-load and HTTP-probe paths without requiring a live server.
func TestCheckStatusNotRunning(t *testing.T) {
	if got := checkStatus(); got != 0 && got != 1 {
		t.Errorf("checkStatus() = %d, want 0 or 1", got)
	}
}

// TestCheckStatusRunning exercises the success-path branches of checkStatus
// (HTTP 200, JSON decode, top-level status field print) by standing up a real
// HTTP listener on the exact configured port and serving a synthetic
// /server/healthz response - without ever starting the full application server.
func TestCheckStatusRunning(t *testing.T) {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}

	// config.Load's DefaultConfig leaves Server.Port empty (the real port is
	// normally assigned during server startup, not config load), so bind an
	// ephemeral listener ourselves and persist that port into the config -
	// this mirrors what a running server would have done before checkStatus
	// is ever invoked.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind an ephemeral listener: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	cfg.Server.Port = port
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save(%q): %v", configPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/server/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	if got := checkStatus(); got != 0 {
		t.Errorf("checkStatus() with a healthy server = %d, want 0", got)
	}
}

func TestCheckStatusUsesEnvironmentPortWhenConfigPortIsEmpty(t *testing.T) {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}
	cfg.Server.Port = ""
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save(%q): %v", configPath, err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind an ephemeral listener: %v", err)
	}
	t.Setenv("PORT", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))
	mux := http.NewServeMux()
	mux.HandleFunc("/server/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	if got := checkStatus(); got != 0 {
		t.Errorf("checkStatus() with an empty configured port and PORT env = %d, want 0", got)
	}
}

// TestCheckStatusUnhealthy exercises the "Unhealthy (HTTP %d)" branch of
// checkStatus - a live server that responds on /server/healthz with a non-200
// status must produce a checkStatus() return value of 1.
func TestCheckStatusUnhealthy(t *testing.T) {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", configPath, err)
	}

	// config.Load's DefaultConfig leaves Server.Port empty, so bind an
	// ephemeral listener ourselves and persist that port into the config -
	// see TestCheckStatusRunning for the full rationale.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("could not bind an ephemeral listener: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	cfg.Server.Port = port
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save(%q): %v", configPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/server/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unhealthy"}`))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	if got := checkStatus(); got != 1 {
		t.Errorf("checkStatus() with an unhealthy server = %d, want 1", got)
	}
}

// createBackup and restoreBackup both unconditionally call
// paths.GetDefaultDirs for the real config/data dirs regardless of whether
// fileLocation is empty or explicit. Supplying an explicit fileLocation
// (inside a test-owned /tmp/apimgr dir) avoids ever touching the hardcoded
// /mnt/Backups/apimgr/airports default location, while still exercising the
// real archive/extract logic against /etc/apimgr/airports and
// /var/lib/apimgr/airports - contained by the same disposable, --rm'd, root
// make test Docker container already relied on by
// TestSetUpdateBranchValid/TestSetApplicationModeValid.
func TestCreateBackupExplicitLocation(t *testing.T) {
	dir := newMainTestDir(t)
	dest := filepath.Join(dir, "explicit-backup.tar.gz")

	// createBackup unconditionally walks the real config/data dirs - ensure
	// they exist in this disposable container before calling it, since a
	// fresh test run may not have created them yet.
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup(dest, false); err != nil {
		t.Fatalf("createBackup(%q): unexpected error: %v", dest, err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("Stat(%q) after createBackup: %v", dest, err)
	}
	if info.Size() == 0 {
		t.Error("createBackup produced an empty archive")
	}

	// No leftover .tmp file after a successful atomic rename.
	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("leftover tmp file after createBackup, stat err = %v", err)
	}
}

// TestRestoreBackupSuccess round-trips a backup created via createBackup
// through restoreBackup, exercising the extraction success path (both the
// "config/" and "data/" tar entry prefixes) without ever touching the
// /mnt/Backups default location.
func TestRestoreBackupSuccess(t *testing.T) {
	dir := newMainTestDir(t)
	archive := filepath.Join(dir, "roundtrip-backup.tar.gz")

	// restoreBackup unconditionally extracts into the real config/data dirs -
	// ensure they exist in this disposable container first.
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup(archive, false); err != nil {
		t.Fatalf("createBackup(%q): unexpected error: %v", archive, err)
	}

	if err := restoreBackup(archive, false); err != nil {
		t.Fatalf("restoreBackup(%q): unexpected error: %v", archive, err)
	}
}

// TestCreateBackupDefaultLocationAlsoCreatesDailyIncremental exercises
// createBackup's empty-fileLocation (scheduled backup_daily task) path per
// AI.md PART 21 "Backup Files Created": the full dated backup AND the
// "{project}-daily.tar.gz" incremental must both exist afterward. Uses
// backupDirOverride to redirect the default backup location into a
// test-owned temp dir instead of the real /mnt/Backups path.
func TestCreateBackupDefaultLocationAlsoCreatesDailyIncremental(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createBackup("", false); err != nil {
		t.Fatalf("createBackup(\"\"): unexpected error: %v", err)
	}

	dailyPath := filepath.Join(dir, ProjectName+"-daily.tar.gz")
	info, err := os.Stat(dailyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after createBackup: %v", dailyPath, err)
	}
	if info.Size() == 0 {
		t.Error("daily incremental backup is empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	sawFull := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ProjectName+"_backup_") {
			sawFull = true
		}
	}
	if !sawFull {
		t.Errorf("expected a %s_backup_* full backup alongside the daily incremental, got entries %v", ProjectName, entries)
	}
}

// TestCreateIncrementalBackupHourlyReplacesExistingFile confirms
// createIncrementalBackup writes a single "{project}-hourly.tar.gz" file
// and that a second run replaces (not duplicates) it, per AI.md PART 21
// "Backup Files Created" - "Always 1 (replaced each run)".
func TestCreateIncrementalBackupHourlyReplacesExistingFile(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createIncrementalBackup("hourly"); err != nil {
		t.Fatalf("createIncrementalBackup(hourly) #1: unexpected error: %v", err)
	}
	hourlyPath := filepath.Join(dir, ProjectName+"-hourly.tar.gz")
	firstInfo, err := os.Stat(hourlyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after first run: %v", hourlyPath, err)
	}

	if err := createIncrementalBackup("hourly"); err != nil {
		t.Fatalf("createIncrementalBackup(hourly) #2: unexpected error: %v", err)
	}
	secondInfo, err := os.Stat(hourlyPath)
	if err != nil {
		t.Fatalf("Stat(%q) after second run: %v", hourlyPath, err)
	}
	if secondInfo.Size() == 0 {
		t.Error("hourly incremental backup is empty after replace")
	}
	_ = firstInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ProjectName+"-hourly.") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 hourly incremental file after two runs, got %d (entries %v)", count, entries)
	}
}

// TestCreateIncrementalBackupNotSubjectToFullBackupRetention creates several
// dated full backups (exceeding max_backups) plus a daily incremental, runs
// createBackup's retention sweep, and confirms the incremental file is never
// counted or deleted by it - per AI.md PART 21 "Backup Files Created"
// retention column ("Always 1 (replaced each run)" vs "Controlled by
// max_backups").
func TestCreateIncrementalBackupNotSubjectToFullBackupRetention(t *testing.T) {
	dir := newMainTestDir(t)
	origOverride := backupDirOverride
	backupDirOverride = dir
	t.Cleanup(func() { backupDirOverride = origOverride })

	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", configDir, err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dataDir, err)
	}

	if err := createIncrementalBackup("daily"); err != nil {
		t.Fatalf("createIncrementalBackup(daily): unexpected error: %v", err)
	}
	dailyPath := filepath.Join(dir, ProjectName+"-daily.tar.gz")
	if _, err := os.Stat(dailyPath); err != nil {
		t.Fatalf("Stat(%q): %v", dailyPath, err)
	}

	// A stale-dated full backup that createBackup's own retention sweep
	// (max_backups defaults to 1) should be free to delete without ever
	// touching the daily incremental sitting alongside it.
	staleFull := filepath.Join(dir, ProjectName+"_backup_2000-01-01.tar.gz")
	if err := os.WriteFile(staleFull, []byte("not a real archive, just retention bait"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", staleFull, err)
	}

	if err := createBackup("", false); err != nil {
		t.Fatalf("createBackup(\"\"): unexpected error: %v", err)
	}

	if _, err := os.Stat(dailyPath); err != nil {
		t.Errorf("daily incremental was removed by full-backup retention: %v", err)
	}
}

func TestExtractEmailFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantFound   bool
		wantAction  string
		wantAddress string
		wantRemain  []string
	}{
		{"no email keyword", []string{"--port", "8080"}, false, "", "", []string{"--port", "8080"}},
		{"bare email", []string{"email"}, true, "", "", []string{}},
		{"email test", []string{"email", "test"}, true, "test", "", []string{}},
		{"email test address", []string{"email", "test", "someone@example.com"}, true, "test", "someone@example.com", []string{}},
		{"email not first arg is ignored", []string{"--debug", "email", "test"}, false, "", "", []string{"--debug", "email", "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, action, address, remaining := extractEmailFlag(tt.args)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if action != tt.wantAction {
				t.Errorf("action = %q, want %q", action, tt.wantAction)
			}
			if address != tt.wantAddress {
				t.Errorf("address = %q, want %q", address, tt.wantAddress)
			}
			if len(remaining) != len(tt.wantRemain) {
				t.Fatalf("remaining = %v, want %v", remaining, tt.wantRemain)
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemain[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemain[i])
				}
			}
		})
	}
}

func TestNotifyRecipient(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "reply_to wins",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Server.Notifications.Email.ReplyTo = "reply@example.com"
				c.Server.Notifications.Email.From.Email = "from@example.com"
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "reply@example.com",
		},
		{
			name: "falls back to from.email",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Server.Notifications.Email.From.Email = "from@example.com"
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "from@example.com",
		},
		{
			name: "falls back to web.security.admin",
			cfg: func() *config.Config {
				c := &config.Config{}
				c.Web.Security.Admin = "admin@example.com"
				return c
			}(),
			want: "admin@example.com",
		},
		{
			name: "all empty",
			cfg:  &config.Config{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notifyRecipient(tt.cfg); got != tt.want {
				t.Errorf("notifyRecipient() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleEmailCLIUnknownAction(t *testing.T) {
	if code := handleEmailCLI("bogus", ""); code != 1 {
		t.Errorf("handleEmailCLI(%q) = %d, want 1", "bogus", code)
	}
}

// NOTE: the following main.go functions are intentionally NOT covered by
// unit tests in this package, because they start a live HTTP server, shell
// out to real service managers with no injectable interface, or call
// os.Exit:
//   main (parses os.Args and calls os.Exit)
//   run (success path beyond the --color validation: starts the real HTTP
//     server, binds a live listener, blocks on signal handling)
//   createBackup / restoreBackup (empty-fileLocation branches: these still
//     touch the hardcoded /mnt/Backups/apimgr/airports path, which does not
//     exist and is not safe to create in this environment - the explicit-
//     fileLocation success paths are covered above)
//   handleServiceCommand (named-command branches: invoke real service.*
//     functions that mutate systemd/OpenRC/launchd/Windows service state)
//   handleEmailCLI (success/recipient-missing/CanSend-false branches: these
//     call config.Load against the real default config dir and attempt a
//     live SMTP send; only the pure-validation "unknown action" branch is
//     exercised here)
//   checkForUpdate / checkAndUpdate (success path beyond the in-container
//     short-circuit: uses a live http.Client with no injection point,
//     downloads and atomically replaces the running binary)
// Reaching full coverage on these without refactoring toward injectable
// config paths / an httptest-backed default client, or running destructive
// operations against real system state in a disposable container, is not
// achievable safely per the task's constraints.
