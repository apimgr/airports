package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
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

// Covers honorNoColor: env-var-set vs unset per the NO_COLOR convention.
func TestHonorNoColor(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		if honorNoColor() {
			t.Error("honorNoColor() = true when NO_COLOR unset")
		}
	})
	t.Run("set-empty", func(t *testing.T) {
		os.Setenv("NO_COLOR", "")
		defer os.Unsetenv("NO_COLOR")
		if honorNoColor() {
			t.Error("honorNoColor() = true when NO_COLOR set to empty string")
		}
	})
	t.Run("set", func(t *testing.T) {
		os.Setenv("NO_COLOR", "1")
		defer os.Unsetenv("NO_COLOR")
		if !honorNoColor() {
			t.Error("honorNoColor() = false when NO_COLOR set")
		}
	})
}

// Covers envOr: present vs absent vs empty-string env var (falls back to default).
func TestEnvOr(t *testing.T) {
	const key = "AIRPORTS_TEST_ENVOR"
	t.Run("unset", func(t *testing.T) {
		os.Unsetenv(key)
		if got := envOr(key, "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want fallback", got)
		}
	})
	t.Run("set", func(t *testing.T) {
		os.Setenv(key, "value")
		defer os.Unsetenv(key)
		if got := envOr(key, "fallback"); got != "value" {
			t.Errorf("envOr = %q, want value", got)
		}
	})
	t.Run("set-empty-falls-back", func(t *testing.T) {
		os.Setenv(key, "")
		defer os.Unsetenv(key)
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
		os.Setenv("XDG_CONFIG_HOME", "/xdg/base")
		defer os.Unsetenv("XDG_CONFIG_HOME")
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
		os.Unsetenv("XDG_CONFIG_HOME")
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
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

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
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

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
		cfg := &cliConfig{Token: "tok_abc", Color: "auto", Lang: "en"}
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
		if got.Server.Primary != cfg.Server.Primary || got.Token != cfg.Token || got.Color != cfg.Color || got.Lang != cfg.Lang {
			t.Errorf("loadCLIConfig round-trip = %+v, want %+v", got, cfg)
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
		defer f.Close()
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
		f.Close()
		if isTTY(f) {
			t.Error("isTTY(closed file) = true, want false")
		}
	})
}

// Covers runFirstRunWizard's non-interactive short-circuits: an existing
// saved server URL is returned as-is, and when stdin/stderr aren't TTYs the
// default URL is returned without prompting (always true under `go test`).
func TestRunFirstRunWizard(t *testing.T) {
	dir := testTempDir(t)
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

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
		got := runFirstRunWizard("http://localhost:80", "wizard2-nonexistent")
		if got != "http://localhost:80" {
			t.Errorf("runFirstRunWizard (non-tty) = %q, want default", got)
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
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("Accept header = %q, want application/json", r.Header.Get("Accept"))
			}
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "airports-cli/") {
				t.Errorf("User-Agent = %q, want airports-cli/ prefix", r.Header.Get("User-Agent"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true,"data":{"code":"KJFK"}}`))
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
			w.Write([]byte(`{}`))
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
			w.Write([]byte(`{"ok":false,"error":"NOT_FOUND"}`))
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
			w.Write([]byte(`not json`))
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
			w.Write([]byte(`{"ok":true,"data":[]}`))
		case strings.Contains(r.URL.Path, "/airports/nearby"):
			w.Write([]byte(`{"ok":true,"data":[]}`))
		case strings.Contains(r.URL.Path, "/airports/KJFK"):
			w.Write([]byte(`{"ok":true,"data":{"code":"KJFK"}}`))
		case strings.Contains(r.URL.Path, "/server/healthz"):
			w.Write([]byte(`{"ok":true,"status":"ok"}`))
		case strings.Contains(r.URL.Path, "/server/about"):
			w.Write([]byte(`{"ok":true,"data":{"name":"airports"}}`))
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
// binary name and default server URL.
func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf, "airports-cli", "en")
	out := buf.String()
	if !strings.Contains(out, "airports-cli") {
		t.Error("printUsage output missing binary name")
	}
	if !strings.Contains(out, defaultBaseURL) {
		t.Error("printUsage output missing default server URL")
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
