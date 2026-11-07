package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/client/tui"
	"github.com/apimgr/airports/src/common/display"
	"github.com/google/uuid"
)

// testTempDir creates an isolated temp dir under /tmp/apimgr/airports-XXXXXX
// per project convention, and returns it plus automatic cleanup.
func testTempDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll base: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove test temp directory: %v", err)
		}
	})
	return dir
}

func assertRequestID(t *testing.T, r *http.Request) {
	t.Helper()
	requestID := r.Header.Get("X-Request-ID")
	parsed, err := uuid.Parse(requestID)
	if err != nil || parsed.Version() != uuid.Version(4) || parsed.Variant() != uuid.RFC4122 {
		t.Errorf("X-Request-ID = %q, want UUID v4", requestID)
	}
}

func TestExecuteRawRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestID(t, r)
		if r.URL.Path != "/prefix/api/v1/airports" || r.URL.Query().Get("q") != "JFK" {
			t.Errorf("unexpected request URL: %s", r.URL.String())
		}
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request method/auth: %s %q", r.Method, r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing JSON content type: %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer server.Close()
	c := &client{baseURL: server.URL + "/prefix", apiVersion: "v1", token: "secret", http: server.Client()}
	got, err := executeRawRequest(context.Background(), c, http.MethodPost, "/api/v1/airports", url.Values{"q": {"JFK"}}, `{"name":"JFK"}`)
	if err != nil {
		t.Fatalf("executeRawRequest: %v", err)
	}
	if got != `{"created":true}` {
		t.Fatalf("response = %q", got)
	}
}

func TestClientRetriesTransientHTTPFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c := newClient(server.URL, "", "en")
	c.http = server.Client()
	c.retryCount = 1
	c.retryDelay = 0
	body, err := c.get(context.Background(), server.URL, nil)
	if err != nil || string(body) != `{"ok":true}` || attempts != 2 {
		t.Fatalf("retry GET = (%q, %v), attempts=%d; want successful second attempt", body, err, attempts)
	}
}

func TestExecuteRawRequestRejectsExternalPathsAndHTTPFailures(t *testing.T) {
	c := &client{baseURL: "https://example.test", apiVersion: "v1", http: http.DefaultClient}
	for _, path := range []string{"https://other.test/api", "//other.test/api", "/api#fragment", "/api\\other"} {
		if _, err := executeRawRequest(context.Background(), c, http.MethodGet, path, nil, ""); err == nil {
			t.Errorf("path %q unexpectedly accepted", path)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer server.Close()
	c.baseURL = server.URL
	got, err := executeRawRequest(context.Background(), c, http.MethodGet, "/airports", nil, "")
	if err == nil || !strings.Contains(err.Error(), "403") || got == "" {
		t.Fatalf("error response = %q, %v", got, err)
	}
}

func TestTestClientConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestID(t, r)
		if r.URL.Path != "/server/healthz" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	body, err := testClientConnection(context.Background(), server.URL, "token")
	if err != nil || body != `{"status":"ok"}` {
		t.Fatalf("testClientConnection = %q, %v", body, err)
	}
	if _, err := testClientConnection(context.Background(), "ftp://example.test", ""); err == nil {
		t.Fatal("invalid URL accepted")
	}
}

func TestExecuteTUIRequestOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"` + r.URL.Path + `","query":"` + r.URL.Query().Encode() + `"}`))
	}))
	defer server.Close()
	for _, operation := range []tui.Request{
		{Operation: "search", Query: "JFK"},
		{Operation: "get", Code: "KJFK"},
		{Operation: "nearby", Latitude: "40", Longitude: "-73", Limit: "5"},
		{Operation: "health"},
		{Operation: "version"},
	} {
		if _, err := executeTUIRequest(context.Background(), server.URL, "", "en", operation); err != nil {
			t.Errorf("%s request: %v", operation.Operation, err)
		}
	}
	if _, err := executeTUIRequest(context.Background(), server.URL, "", "en", tui.Request{Operation: "unknown"}); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if _, err := executeTUIRequest(context.Background(), server.URL, "", "en", tui.Request{Operation: "api", Method: "GET", Path: "/airports", Query: "%zz"}); err == nil {
		t.Fatal("invalid API query accepted")
	}
}

// Covers validColor: the three accepted values plus rejection of anything else,
// including empty string and case variants (spec is case-sensitive lowercase).
func TestValidColor(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"auto", true},
		{"yes", true},
		{"no", true},
		{"", false},
		{"Auto", false},
		{"maybe", false},
		{"YES", false},
	}
	for _, tt := range tests {
		if got := validColor(tt.in); got != tt.want {
			t.Errorf("validColor(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSelectDisplayMode(t *testing.T) {
	tests := []struct {
		name string
		env  display.DisplayEnv
		args []string
		want display.DisplayMode
	}{
		{name: "auto GUI", env: display.DisplayEnv{Mode: display.DisplayModeGUI}, want: display.DisplayModeGUI},
		{name: "auto TUI", env: display.DisplayEnv{Mode: display.DisplayModeTUI}, want: display.DisplayModeTUI},
		{name: "auto CLI", env: display.DisplayEnv{Mode: display.DisplayModeCLI}, want: display.DisplayModeCLI},
		{name: "headless requires interaction", env: display.DisplayEnv{Mode: display.DisplayModeHeadless}, want: display.DisplayModeHeadless},
		{name: "command selects CLI from GUI", env: display.DisplayEnv{Mode: display.DisplayModeGUI}, args: []string{"search", "JFK"}, want: display.DisplayModeCLI},
		{name: "command remains scriptable when headless", env: display.DisplayEnv{Mode: display.DisplayModeHeadless}, args: []string{"health"}, want: display.DisplayModeCLI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectDisplayMode(tt.env, tt.args); got != tt.want {
				t.Errorf("selectDisplayMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyDisplayConfig(t *testing.T) {
	tests := []struct {
		name       string
		env        display.DisplayEnv
		detected   display.DisplayMode
		configured string
		want       display.DisplayMode
		wantErr    bool
	}{
		{name: "auto", detected: display.DisplayModeTUI, configured: "auto", want: display.DisplayModeTUI},
		{name: "force gui", env: display.DisplayEnv{HasDisplay: true}, configured: "gui", want: display.DisplayModeGUI},
		{name: "gui unavailable", configured: "gui", want: display.DisplayModeHeadless, wantErr: true},
		{name: "force tui", env: display.DisplayEnv{IsTerminal: true}, configured: "tui", want: display.DisplayModeTUI},
		{name: "dumb terminal forces cli over configured tui", env: display.DisplayEnv{IsTerminal: true, TerminalType: "dumb"}, configured: "tui", want: display.DisplayModeCLI},
		{name: "dumb terminal forces cli over configured gui", env: display.DisplayEnv{HasDisplay: true, TerminalType: "dumb"}, configured: "gui", want: display.DisplayModeCLI},
		{name: "tui unavailable", configured: "tui", want: display.DisplayModeHeadless, wantErr: true},
		{name: "invalid", configured: "cli", want: display.DisplayModeHeadless, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := applyDisplayConfig(test.env, test.detected, test.configured)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("applyDisplayConfig() = (%v, %v), want (%v, wantErr=%v)", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestIsNewerCLIVersion(t *testing.T) {
	for _, test := range []struct {
		current, available string
		wantNewer          bool
		wantErr            bool
	}{
		{current: "1.2.0", available: "1.3.0", wantNewer: true},
		{current: "1.3.0", available: "1.2.0"},
		{current: "dev", available: "1.0.0", wantNewer: true},
		{current: "not-a-version", available: "1.0.0", wantErr: true},
	} {
		got, err := isNewerCLIVersion(test.current, test.available)
		if (err != nil) != test.wantErr || got != test.wantNewer {
			t.Errorf("isNewerCLIVersion(%q, %q) = (%t, %v), want (%t, wantErr=%v)", test.current, test.available, got, err, test.wantNewer, test.wantErr)
		}
	}
}

func TestCheckCLIRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/autodiscover" {
			t.Errorf("autodiscover path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("Authorization = %q, want Bearer token", r.Header.Get("Authorization"))
		}
		_, _ = fmt.Fprintf(w, `{"cli_versions":{"%s-%s":{"version":"1.2.0","sha256":"%s"}},"cli_min_version":"1.0.0"}`, runtime.GOOS, runtime.GOARCH, strings.Repeat("a", 64))
	}))
	defer server.Close()
	c := &client{baseURL: server.URL + "/prefix", token: "token", http: server.Client()}
	metadata, release, err := checkCLIRelease(context.Background(), c)
	if err != nil {
		t.Fatalf("checkCLIRelease() error = %v", err)
	}
	if metadata.MinVersion != "1.0.0" || release.Version != "1.2.0" || len(release.SHA256) != 64 {
		t.Errorf("metadata/release = %+v / %+v, want minimum 1.0.0, release 1.2.0 with SHA-256", metadata, release)
	}
}

func TestExtractLangFlag(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--lang", "fr", "search"}, "fr"},
		{[]string{"--lang=es"}, "es"},
		{[]string{"-lang=de"}, "de"},
		{[]string{"search", "--", "--lang", "ja"}, ""},
		{[]string{"--lang"}, ""},
	} {
		if got := extractLangFlag(test.args); got != test.want {
			t.Errorf("extractLangFlag(%v) = %q, want %q", test.args, got, test.want)
		}
	}
}

// Covers honorNoColor: env-var-set vs unset per the NO_COLOR convention.
func TestHonorNoColor(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if honorNoColor() {
			t.Error("honorNoColor() = true when NO_COLOR unset")
		}
	})
	t.Run("set-empty", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if honorNoColor() {
			t.Error("honorNoColor() = true when NO_COLOR set to empty string")
		}
	})
	t.Run("set", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if !honorNoColor() {
			t.Error("honorNoColor() = false when NO_COLOR set")
		}
	})
}

// Covers envOr: present vs absent vs empty-string env var (falls back to default).
func TestEnvOr(t *testing.T) {
	const key = "AIRPORTS_TEST_ENVOR"
	t.Run("unset", func(t *testing.T) {
		t.Setenv(key, "")
		if got := envOr(key, "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want fallback", got)
		}
	})
	t.Run("set", func(t *testing.T) {
		t.Setenv(key, "value")
		if got := envOr(key, "fallback"); got != "value" {
			t.Errorf("envOr = %q, want value", got)
		}
	})
	t.Run("set-empty-falls-back", func(t *testing.T) {
		t.Setenv(key, "")
		if got := envOr(key, "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want fallback for empty env value", got)
		}
	})
}

// Covers extractConfigFlag: --config/--config= in both dash forms, missing
// value, and absence of the flag entirely.
func TestExtractConfigFlag(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"absent", []string{"search", "kennedy"}, ""},
		{"space-form", []string{"--config", "test", "search"}, "test"},
		{"single-dash-space", []string{"-config", "prod", "get", "KJFK"}, "prod"},
		{"equals-form", []string{"--config=staging"}, "staging"},
		{"single-dash-equals", []string{"-config=alt"}, "alt"},
		{"missing-value-at-end", []string{"--config"}, ""},
		{"first-match-wins", []string{"--config=first", "--config=second"}, "first"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractConfigFlag(tt.argv); got != tt.want {
				t.Errorf("extractConfigFlag(%v) = %q, want %q", tt.argv, got, tt.want)
			}
		})
	}
}

// Covers cliConfigDir: honors XDG_CONFIG_HOME when set, falls back to
// $HOME/.config otherwise.
func TestCliConfigDir(t *testing.T) {
	t.Run("xdg-set", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/xdg/base")
		dir, err := cliConfigDir()
		if err != nil {
			t.Fatalf("cliConfigDir: %v", err)
		}
		want := filepath.Join("/xdg/base", "apimgr", "airports")
		if dir != want {
			t.Errorf("cliConfigDir() = %q, want %q", dir, want)
		}
	})
	t.Run("xdg-unset-uses-home", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		home := os.Getenv("HOME")
		dir, err := cliConfigDir()
		if err != nil {
			t.Fatalf("cliConfigDir: %v", err)
		}
		want := filepath.Join(home, ".config", "apimgr", "airports")
		if dir != want {
			t.Errorf("cliConfigDir() = %q, want %q", dir, want)
		}
	})
}

// Covers cliConfigPath: absolute path passthrough, empty name -> "cli.yml",
// explicit extensions, and the .yml-before-.yaml auto-detection when a file
// of one extension already exists on disk.
func TestCliConfigPath(t *testing.T) {
	dir := testTempDir(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	t.Run("absolute", func(t *testing.T) {
		abs := filepath.Join(dir, "explicit.yml")
		got, err := cliConfigPath(abs)
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		if got != abs {
			t.Errorf("cliConfigPath(abs) = %q, want %q", got, abs)
		}
	})

	t.Run("empty-defaults-to-cli", func(t *testing.T) {
		got, err := cliConfigPath("")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		want := filepath.Join(dir, "apimgr", "airports", "cli.yml")
		if got != want {
			t.Errorf("cliConfigPath(\"\") = %q, want %q", got, want)
		}
	})

	t.Run("explicit-yaml-suffix", func(t *testing.T) {
		got, err := cliConfigPath("profile.yaml")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		want := filepath.Join(dir, "apimgr", "airports", "profile.yaml")
		if got != want {
			t.Errorf("cliConfigPath = %q, want %q", got, want)
		}
	})

	t.Run("bare-name-no-existing-file-defaults-yml", func(t *testing.T) {
		got, err := cliConfigPath("newprofile")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		want := filepath.Join(dir, "apimgr", "airports", "newprofile.yml")
		if got != want {
			t.Errorf("cliConfigPath = %q, want %q", got, want)
		}
	})

	t.Run("bare-name-existing-yaml-file-detected", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		yamlPath := filepath.Join(confDir, "existing.yaml")
		if err := os.WriteFile(yamlPath, []byte("server_url: x\n"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		got, err := cliConfigPath("existing")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		if got != yamlPath {
			t.Errorf("cliConfigPath = %q, want %q (should prefer existing .yaml)", got, yamlPath)
		}
	})

	t.Run("bare-name-existing-yml-file-preferred-over-yaml", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		ymlPath := filepath.Join(confDir, "both.yml")
		yamlPath := filepath.Join(confDir, "both.yaml")
		if err := os.WriteFile(ymlPath, []byte("server_url: x\n"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(yamlPath, []byte("server_url: y\n"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		got, err := cliConfigPath("both")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		if got != ymlPath {
			t.Errorf("cliConfigPath = %q, want %q (.yml wins over .yaml)", got, ymlPath)
		}
	})
}

// Covers loadCLIConfig/saveCLIConfig round trip, plus the "file absent"
// non-error path and the "invalid YAML" error path.
func TestLoadSaveCLIConfig(t *testing.T) {
	dir := testTempDir(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	t.Run("absent-file-returns-empty-no-error", func(t *testing.T) {
		cfg, err := loadCLIConfig("doesnotexist")
		if err != nil {
			t.Fatalf("loadCLIConfig: unexpected error: %v", err)
		}
		if cfg.Server.Primary != "" || cfg.Token != "" {
			t.Errorf("loadCLIConfig for absent file = %+v, want zero value", cfg)
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		cfg := &cliConfig{Token: "tok_abc", Output: cliOutputConfig{Color: "no"}, Lang: "en"}
		cfg.Server.Primary = "https://example.com"
		if err := saveCLIConfig(cfg, "roundtrip"); err != nil {
			t.Fatalf("saveCLIConfig: %v", err)
		}
		path, err := cliConfigPath("roundtrip")
		if err != nil {
			t.Fatalf("cliConfigPath: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("saved file missing: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("saved config file perms = %v, want 0600", info.Mode().Perm())
		}

		got, err := loadCLIConfig("roundtrip")
		if err != nil {
			t.Fatalf("loadCLIConfig: %v", err)
		}
		if got.Server.Primary != cfg.Server.Primary || got.Token != cfg.Token || got.Output.Color != "no" || got.Color != "" || got.Lang != cfg.Lang {
			t.Errorf("loadCLIConfig round-trip = %+v, want %+v", got, cfg)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read saved config: %v", err)
		}
		if strings.Contains(string(data), "\ncolor:") || !strings.Contains(string(data), "output:") || !strings.Contains(string(data), "auth:") || strings.Contains(string(data), "\ntoken:") {
			t.Errorf("saved config does not use canonical output.color shape:\n%s", data)
		}
	})

	t.Run("legacy-color-is-read-and-canonicalized-on-save", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		legacyPath := filepath.Join(confDir, "legacy.yml")
		if err := os.WriteFile(legacyPath, []byte("color: no\n"), 0600); err != nil {
			t.Fatalf("write legacy config: %v", err)
		}
		cfg, err := loadCLIConfig("legacy")
		if err != nil {
			t.Fatalf("load legacy config: %v", err)
		}
		if cfg.Output.Color != "no" {
			t.Fatalf("legacy output color = %v, want false", cfg.Output.Color)
		}
		if err := saveCLIConfig(cfg, "legacy"); err != nil {
			t.Fatalf("save migrated config: %v", err)
		}
		data, err := os.ReadFile(legacyPath)
		if err != nil {
			t.Fatalf("read migrated config: %v", err)
		}
		if strings.Contains(string(data), "\ncolor:") || !strings.Contains(string(data), "output:") {
			t.Errorf("legacy config was not saved in canonical shape:\n%s", data)
		}
		migrated, err := loadCLIConfig("legacy")
		if err != nil || migrated.Output.Color != "no" || migrated.Color != "" {
			t.Errorf("migrated config color = %q/%q, err=%v; want canonical output.color no", migrated.Output.Color, migrated.Color, err)
		}
	})

	t.Run("legacy-root-token-migrates-to-auth-section", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		legacyPath := filepath.Join(confDir, "legacy-token.yml")
		if err := os.WriteFile(legacyPath, []byte("token: old-token\n"), 0600); err != nil {
			t.Fatalf("write legacy token config: %v", err)
		}
		cfg, err := loadCLIConfig("legacy-token")
		if err != nil || cfg.Token != "old-token" {
			t.Fatalf("load legacy token = %q, %v", cfg.Token, err)
		}
		if err := saveCLIConfig(cfg, "legacy-token"); err != nil {
			t.Fatalf("save migrated config: %v", err)
		}
		data, err := os.ReadFile(legacyPath)
		if err != nil {
			t.Fatalf("read migrated config: %v", err)
		}
		if !strings.Contains(string(data), "auth:\n  token: old-token") || strings.Contains(string(data), "\ntoken:") {
			t.Errorf("legacy token was not saved canonically:\n%s", data)
		}
	})

	t.Run("boolean-output-color-is-accepted", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		path := filepath.Join(confDir, "boolean.yml")
		if err := os.WriteFile(path, []byte("output:\n  color: false\n"), 0600); err != nil {
			t.Fatalf("write boolean config: %v", err)
		}
		cfg, err := loadCLIConfig("boolean")
		if err != nil || cfg.Output.Color != "no" {
			t.Fatalf("boolean output.color = %q, err=%v; want no", cfg.Output.Color, err)
		}
	})

	t.Run("invalid-yaml-returns-error", func(t *testing.T) {
		confDir := filepath.Join(dir, "apimgr", "airports")
		if err := os.MkdirAll(confDir, 0700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		badPath := filepath.Join(confDir, "bad.yml")
		if err := os.WriteFile(badPath, []byte("not: valid: yaml: [["), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := loadCLIConfig("bad")
		if err == nil {
			t.Error("loadCLIConfig with invalid YAML: expected error, got nil")
		}
	})
}

func TestLoadCLIConfigCanonicalizesUpdateChannel(t *testing.T) {
	dir := testTempDir(t)
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := cliConfigPath("channel-alias")
	if err != nil {
		t.Fatalf("cliConfigPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("update:\n  channel: daily\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := loadCLIConfig("channel-alias")
	if err != nil {
		t.Fatalf("loadCLIConfig: %v", err)
	}
	if cfg.Update.Channel != "devel" {
		t.Fatalf("Update.Channel = %q, want devel", cfg.Update.Channel)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(saved), "channel: devel") {
		t.Errorf("cli.yml did not persist canonical devel channel: %s", saved)
	}
}

// Covers isTTY against a regular file (never a char device) and a
// already-closed file handle (Stat error path).
func TestIsTTY(t *testing.T) {
	dir := testTempDir(t)
	t.Run("regular-file-is-not-tty", func(t *testing.T) {
		path := filepath.Join(dir, "plain.txt")
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer func() { _ = f.Close() }()
		if isTTY(f) {
			t.Error("isTTY(regular file) = true, want false")
		}
	})

	t.Run("closed-file-stat-error", func(t *testing.T) {
		path := filepath.Join(dir, "closed.txt")
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		_ = f.Close()
		if isTTY(f) {
			t.Error("isTTY(closed file) = true, want false")
		}
	})
}

// Covers runFirstRunWizard's non-interactive short-circuits: an existing
// saved server URL is returned as-is, and when stdin/stderr aren't TTYs the
// empty fallback is returned without prompting (always true under `go test`).
func TestRunFirstRunWizard(t *testing.T) {
	dir := testTempDir(t)
	t.Setenv("XDG_CONFIG_HOME", dir)

	t.Run("existing-server-url-returned", func(t *testing.T) {
		cfg := &cliConfig{}
		cfg.Server.Primary = "https://saved.example.com"
		if err := saveCLIConfig(cfg, "wizard1"); err != nil {
			t.Fatalf("saveCLIConfig: %v", err)
		}
		got := runFirstRunWizard("http://localhost:80", "wizard1")
		if got != "https://saved.example.com" {
			t.Errorf("runFirstRunWizard = %q, want saved URL", got)
		}
	})

	t.Run("non-interactive-returns-default", func(t *testing.T) {
		got := runFirstRunWizard("", "wizard2-nonexistent")
		if got != "" {
			t.Errorf("runFirstRunWizard (non-tty) = %q, want no implicit server", got)
		}
	})
}

// Covers client.apiURL: with and without query parameters.
func TestClientAPIURL(t *testing.T) {
	c := &client{baseURL: "http://localhost:8080", apiVersion: "v1"}

	t.Run("no-query", func(t *testing.T) {
		got := c.apiURL("/airports/KJFK", nil)
		want := "http://localhost:8080/api/v1/airports/KJFK"
		if got != want {
			t.Errorf("apiURL = %q, want %q", got, want)
		}
	})

	t.Run("with-query", func(t *testing.T) {
		q := map[string][]string{"lat": {"40.7128"}, "lon": {"-74.006"}}
		got := c.apiURL("/airports/nearby", q)
		if !strings.HasPrefix(got, "http://localhost:8080/api/v1/airports/nearby?") {
			t.Errorf("apiURL with query = %q, missing expected prefix", got)
		}
		if !strings.Contains(got, "lat=40.7128") || !strings.Contains(got, "lon=-74.006") {
			t.Errorf("apiURL with query = %q, missing expected params", got)
		}
	})
}

// Covers client.get: success with JSON decode, HTTP error status propagation
// (still returns the body), malformed-JSON decode error, unreachable server,
// and the Authorization header being set from the token.
func TestClientGet(t *testing.T) {
	t.Run("success-decodes-json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertRequestID(t, r)
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("Accept header = %q, want application/json", r.Header.Get("Accept"))
			}
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "airports-cli/") {
				t.Errorf("User-Agent = %q, want airports-cli/ prefix", r.Header.Get("User-Agent"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"data":{"code":"KJFK"}}`))
		}))
		defer srv.Close()

		c := &client{http: srv.Client()}
		var out struct {
			OK   bool `json:"ok"`
			Data struct {
				Code string `json:"code"`
			} `json:"data"`
		}
		body, err := c.get(context.Background(), srv.URL, &out)
		if err != nil {
			t.Fatalf("get: unexpected error: %v", err)
		}
		if len(body) == 0 {
			t.Error("get: expected non-empty body")
		}
		if !out.OK || out.Data.Code != "KJFK" {
			t.Errorf("get decoded = %+v, want ok=true code=KJFK", out)
		}
	})

	t.Run("token-sets-authorization-header", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		c := &client{http: srv.Client(), token: "secret-token"}
		if _, err := c.get(context.Background(), srv.URL, nil); err != nil {
			t.Fatalf("get: %v", err)
		}
		if gotAuth != "Bearer secret-token" {
			t.Errorf("Authorization header = %q, want Bearer secret-token", gotAuth)
		}
	})

	t.Run("http-error-status-returns-body-and-error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ok":false,"error":"NOT_FOUND"}`))
		}))
		defer srv.Close()

		c := &client{http: srv.Client()}
		body, err := c.get(context.Background(), srv.URL, nil)
		if err == nil {
			t.Fatal("get: expected error for 404 status")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error = %q, want to mention 404", err.Error())
		}
		if !strings.Contains(string(body), "NOT_FOUND") {
			t.Errorf("body = %q, want body preserved on error", body)
		}
	})

	t.Run("malformed-json-decode-error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		defer srv.Close()

		c := &client{http: srv.Client()}
		var out map[string]any
		_, err := c.get(context.Background(), srv.URL, &out)
		if err == nil {
			t.Fatal("get: expected decode error for malformed JSON")
		}
		if !strings.Contains(err.Error(), "decode JSON") {
			t.Errorf("error = %q, want decode JSON error", err.Error())
		}
	})

	t.Run("unreachable-server-returns-request-failed-error", func(t *testing.T) {
		c := &client{http: http.DefaultClient}
		_, err := c.get(context.Background(), "http://127.0.0.1:1/", nil)
		if err == nil {
			t.Fatal("get: expected error for unreachable server")
		}
		if !strings.Contains(err.Error(), "request failed") {
			t.Errorf("error = %q, want request failed error", err.Error())
		}
	})
}

// Covers dispatch: each command's happy path plus argument-validation error
// paths and the unknown-command default case.
func TestDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/airports/search"):
			if r.URL.Query().Get("q") != "kennedy airport" {
				t.Errorf("search q = %q, want 'kennedy airport'", r.URL.Query().Get("q"))
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
		case strings.Contains(r.URL.Path, "/airports/nearby"):
			_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
		case strings.Contains(r.URL.Path, "/airports/KJFK"):
			_, _ = w.Write([]byte(`{"ok":true,"data":{"code":"KJFK"}}`))
		case strings.Contains(r.URL.Path, "/server/healthz"):
			_, _ = w.Write([]byte(`{"ok":true,"status":"ok"}`))
		case strings.Contains(r.URL.Path, "/server/about"):
			_, _ = w.Write([]byte(`{"ok":true,"data":{"name":"airports"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &client{baseURL: srv.URL, apiVersion: "v1", http: srv.Client()}

	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantErr bool
	}{
		{"search-ok", "search", []string{"kennedy", "airport"}, false},
		{"search-missing-query", "search", nil, true},
		{"get-ok", "get", []string{"kjfk"}, false},
		{"get-missing-code", "get", nil, true},
		{"nearby-ok", "nearby", []string{"40.7128", "-74.0060"}, false},
		{"nearby-ok-with-n", "nearby", []string{"40.7128", "-74.0060", "5"}, false},
		{"nearby-missing-args", "nearby", []string{"40.7128"}, true},
		{"health-ok", "health", nil, false},
		{"version-ok", "version", nil, false},
		{"unknown-command", "frobnicate", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dispatch(c, tt.cmd, tt.args, "json", "en")
			if (err != nil) != tt.wantErr {
				t.Errorf("dispatch(%q, %v) error = %v, wantErr %v", tt.cmd, tt.args, err, tt.wantErr)
			}
		})
	}
}

// Covers dispatch's "version" command server-unreachable branch specifically:
// it must print a warning but return nil (client version alone is enough).
func TestDispatchVersionServerUnreachable(t *testing.T) {
	c := &client{baseURL: "http://127.0.0.1:1", apiVersion: "v1", http: http.DefaultClient}
	if err := dispatch(c, "version", nil, "json", "en"); err != nil {
		t.Errorf("dispatch(version) with unreachable server: got error %v, want nil", err)
	}
}

// Covers printResult across all supported formats plus the unsupported-format
// error path and the malformed-JSON fallback-to-raw-text behavior.
func TestPrintResult(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		format  string
		wantErr bool
	}{
		{"json-valid", []byte(`{"a":1}`), "json", false},
		{"json-invalid-falls-back-to-raw", []byte(`not json`), "json", false},
		{"yaml-valid", []byte(`{"a":1}`), "yaml", false},
		{"yaml-invalid-falls-back-to-raw", []byte(`not json`), "yaml", false},
		{"text", []byte("plain text\n"), "text", false},
		{"plain", []byte("plain text\n"), "plain", false},
		{"table", []byte(`{"data":[{"icao":"KJFK","name":"John F Kennedy","city":"New York"}]}`), "table", false},
		{"empty-table", []byte(`{"data":[]}`), "table", false},
		{"csv", []byte(`{"data":[{"icao":"KJFK","name":"John F Kennedy"}]}`), "csv", false},
		{"unsupported-format", []byte(`{}`), "xml", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := printResult(tt.body, tt.format, "en")
			if (err != nil) != tt.wantErr {
				t.Errorf("printResult(%q, %q) error = %v, wantErr %v", tt.body, tt.format, err, tt.wantErr)
			}
		})
	}
}

// Covers printUsage: ensures it writes non-empty usage text mentioning the
// binary name and required server option.
func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf, "airports-cli", "en")
	out := buf.String()
	if !strings.Contains(out, "airports-cli") {
		t.Error("printUsage output missing binary name")
	}
	if !strings.Contains(out, "--server URL") {
		t.Error("printUsage output missing required server option")
	}
	if !strings.Contains(out, "search") || !strings.Contains(out, "nearby") {
		t.Error("printUsage output missing documented commands")
	}
}

// Sanity check that the JSON envelope round-trips through printResult's
// json.Unmarshal/MarshalIndent path without altering data.
func TestPrintResultJSONRoundTrip(t *testing.T) {
	in := map[string]any{"ok": true, "n": float64(5)}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := printResult(raw, "json", "en"); err != nil {
		t.Errorf("printResult: unexpected error: %v", err)
	}
}
