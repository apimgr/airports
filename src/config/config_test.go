package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestDir creates an isolated temp dir under /tmp/apimgr/airports-XXXXXX
// and returns its path along with a cleanup function.
func newTestDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll /tmp/apimgr: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// resetGlobalState clears the package-level current/configPath globals so
// tests do not leak state into each other (this package has mutable global
// state shared with bool_test.go's tests, none of which touch it, but
// config_test.go tests must not interfere with each other).
func resetGlobalState(t *testing.T) {
	t.Helper()
	mu.Lock()
	current = nil
	configPath = ""
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		current = nil
		configPath = ""
		mu.Unlock()
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Address != "[::]" {
		t.Errorf("Address = %q, want [::]", cfg.Server.Address)
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("Mode = %q, want production", cfg.Server.Mode)
	}
	if cfg.Server.Update.Branch != "stable" {
		t.Errorf("Update.Branch = %q, want stable", cfg.Server.Update.Branch)
	}
	if cfg.Server.Update.AutoInstall {
		t.Error("Update.AutoInstall = true, want false")
	}
	if cfg.Server.Update.DeferDays != 0 {
		t.Errorf("Update.DeferDays = %d, want 0", cfg.Server.Update.DeferDays)
	}
	if !cfg.Server.Scheduler.Enabled {
		t.Error("Scheduler.Enabled = false, want true")
	}
	if cfg.Server.SSL.Enabled {
		t.Error("SSL.Enabled = true, want false")
	}
	if !cfg.Server.GeoIP.Enabled {
		t.Error("GeoIP.Enabled = false, want true")
	}
	if cfg.Web.UI.Theme != "dark" {
		t.Errorf("Theme = %q, want dark", cfg.Web.UI.Theme)
	}
	if len(cfg.Server.CORS.AllowedOrigins) != 1 || cfg.Server.CORS.AllowedOrigins[0] != "*" {
		t.Errorf("CORS.AllowedOrigins = %v, want [*]", cfg.Server.CORS.AllowedOrigins)
	}
	// Slices must be non-nil so they marshal as [] not null.
	if cfg.Server.GeoIP.AllowCountries == nil {
		t.Error("AllowCountries is nil, want empty slice")
	}
	if len(cfg.Web.Robots.Allow) == 0 {
		t.Error("Web.Robots.Allow should have default entries")
	}
	ad := cfg.Server.Security.AbuseDetection
	if ad.RequestFlood.Multiplier != 10 {
		t.Errorf("AbuseDetection.RequestFlood.Multiplier = %d, want 10", ad.RequestFlood.Multiplier)
	}
	if ad.RequestFlood.BlockDuration != "1h" {
		t.Errorf("AbuseDetection.RequestFlood.BlockDuration = %q, want 1h", ad.RequestFlood.BlockDuration)
	}
	if !ad.AutoBlockIP {
		t.Error("AbuseDetection.AutoBlockIP = false, want true")
	}
	if !ad.AutoAlert {
		t.Error("AbuseDetection.AutoAlert = false, want true")
	}
	pool := cfg.Server.Database.Pool
	if pool.MaxOpen != 4 || pool.MaxIdle != 4 {
		t.Errorf("Database.Pool = %+v, want MaxOpen=4 MaxIdle=4", pool)
	}
	if pool.MaxLifetime != "5m" || pool.MaxIdleTime != "1m" {
		t.Errorf("Database.Pool durations = %+v, want MaxLifetime=5m MaxIdleTime=1m", pool)
	}
	if cfg.Server.Logging.App.Filename != "app.log" || cfg.Server.Logging.App.Format != "logfmt" {
		t.Errorf("Logging.App = %+v, want app.log/logfmt", cfg.Server.Logging.App)
	}
	if cfg.Server.Logging.Auth.Filename != "auth.log" || cfg.Server.Logging.Auth.Format != "syslog" {
		t.Errorf("Logging.Auth = %+v, want auth.log/syslog", cfg.Server.Logging.Auth)
	}
	if !cfg.Server.Logging.Audit.MaskEmails {
		t.Error("Logging.Audit.MaskEmails = false, want true")
	}
	events := cfg.Server.Logging.Audit.Events
	if !events.Configuration || !events.Security || !events.Backup || !events.Server {
		t.Errorf("Logging.Audit.Events = %+v, want all true", events)
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to be created at %s: %v", path, err)
	}
	if Get().Server.Mode != "production" {
		t.Errorf("Get().Server.Mode = %q, want production", Get().Server.Mode)
	}
}

func TestLoadReadsExistingFile(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	yamlContent := "server:\n  port: \"12345\"\n  fqdn: \"example.com\"\n  address: \"127.0.0.1\"\n  mode: \"development\"\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != "12345" {
		t.Errorf("Port = %q, want 12345", cfg.Server.Port)
	}
	if cfg.Server.FQDN != "example.com" {
		t.Errorf("FQDN = %q, want example.com", cfg.Server.FQDN)
	}
	if cfg.Server.Mode != "development" {
		t.Errorf("Mode = %q, want development", cfg.Server.Mode)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if err := os.WriteFile(path, []byte("server: [this is not: valid: yaml"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Error("expected error loading invalid YAML, got nil")
	}
}

func TestMigrateYamlToYml(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)

	t.Run("yaml-migrates-to-yml", func(t *testing.T) {
		yamlPath := filepath.Join(dir, "a.yaml")
		if err := os.WriteFile(yamlPath, []byte("server:\n  port: \"1\"\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		got := migrateYamlToYml(yamlPath)
		want := filepath.Join(dir, "a.yml")
		if got != want {
			t.Errorf("migrateYamlToYml = %q, want %q", got, want)
		}
		if _, err := os.Stat(want); err != nil {
			t.Errorf(".yml file should exist after migration: %v", err)
		}
		if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
			t.Error(".yaml file should no longer exist after migration")
		}
	})

	t.Run("yaml-not-migrated-when-yml-exists", func(t *testing.T) {
		ymlPath := filepath.Join(dir, "b.yml")
		yamlPath := filepath.Join(dir, "b.yaml")
		if err := os.WriteFile(ymlPath, []byte("server:\n  port: \"existing\"\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(yamlPath, []byte("server:\n  port: \"stale\"\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		got := migrateYamlToYml(yamlPath)
		if got != ymlPath {
			t.Errorf("migrateYamlToYml = %q, want %q", got, ymlPath)
		}
		// Both files should still exist since .yml already existed.
		if _, err := os.Stat(yamlPath); err != nil {
			t.Error(".yaml should remain untouched when .yml already exists")
		}
	})

	t.Run("yml-passthrough-no-yaml-sibling", func(t *testing.T) {
		ymlPath := filepath.Join(dir, "c.yml")
		got := migrateYamlToYml(ymlPath)
		if got != ymlPath {
			t.Errorf("migrateYamlToYml = %q, want %q", got, ymlPath)
		}
	})

	t.Run("yml-migrated-from-legacy-yaml", func(t *testing.T) {
		yamlPath := filepath.Join(dir, "d.yaml")
		ymlPath := filepath.Join(dir, "d.yml")
		if err := os.WriteFile(yamlPath, []byte("server:\n  port: \"legacy\"\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		// Request the .yml path directly - since it doesn't exist but the
		// .yaml sibling does, it should be renamed into place.
		got := migrateYamlToYml(ymlPath)
		if got != ymlPath {
			t.Errorf("migrateYamlToYml = %q, want %q", got, ymlPath)
		}
		if _, err := os.Stat(ymlPath); err != nil {
			t.Errorf("expected %s to exist after migration: %v", ymlPath, err)
		}
	})
}

func TestGetReturnsDefaultWhenUnset(t *testing.T) {
	resetGlobalState(t)
	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("Get() default Mode = %q, want production", cfg.Server.Mode)
	}
}

func TestSaveNilConfig(t *testing.T) {
	resetGlobalState(t)
	if err := Save("/tmp/apimgr/should-not-be-created.yml", nil); err == nil {
		t.Error("Save(nil) expected error, got nil")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	cfg := DefaultConfig()
	cfg.Server.FQDN = "roundtrip.example.com"
	cfg.Server.Port = "64500"
	cfg.Server.GeoIP.AllowCountries = []string{"US", "CA"}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	resetGlobalState(t)
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Server.FQDN != "roundtrip.example.com" {
		t.Errorf("FQDN = %q, want roundtrip.example.com", loaded.Server.FQDN)
	}
	if loaded.Server.Port != "64500" {
		t.Errorf("Port = %q, want 64500", loaded.Server.Port)
	}
	if len(loaded.Server.GeoIP.AllowCountries) != 2 {
		t.Errorf("AllowCountries = %v, want 2 entries", loaded.Server.GeoIP.AllowCountries)
	}
}

func TestSaveCurrentNoConfigLoaded(t *testing.T) {
	resetGlobalState(t)
	if err := SaveCurrent(); err == nil {
		t.Error("SaveCurrent() with no config loaded expected error, got nil")
	}
}

func TestSaveCurrentPersists(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	mu.Lock()
	current.Server.FQDN = "savecurrent.example.com"
	mu.Unlock()

	if err := SaveCurrent(); err != nil {
		t.Fatalf("SaveCurrent: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "savecurrent.example.com") {
		t.Error("expected saved file to contain updated FQDN")
	}
}

func TestUpdate(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	// Update with no configPath set should not error and not write a file.
	cfg := DefaultConfig()
	cfg.Server.FQDN = "noupdatepath.example.com"
	if err := Update(cfg); err != nil {
		t.Fatalf("Update with no path: %v", err)
	}
	if Get().Server.FQDN != "noupdatepath.example.com" {
		t.Error("Update should set current config even without a path")
	}

	// Now load with a path, then Update should persist to disk.
	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg2 := DefaultConfig()
	cfg2.Server.FQDN = "updated.example.com"
	if err := Update(cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "updated.example.com") {
		t.Error("expected Update to persist FQDN to disk")
	}
}

func TestReloadNoConfigPath(t *testing.T) {
	resetGlobalState(t)
	if err := Reload(); err == nil {
		t.Error("Reload() with no config path expected error, got nil")
	}
}

func TestReload(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Mutate the file on disk directly, then reload should pick it up.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	mutated := strings.Replace(string(data), `fqdn: ""`, `fqdn: "reloaded.example.com"`, 1)
	if err := os.WriteFile(path, []byte(mutated), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if Get().Server.FQDN != "reloaded.example.com" {
		t.Errorf("Get().Server.FQDN = %q, want reloaded.example.com", Get().Server.FQDN)
	}
}

func TestGetPortSetPort(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	// SetPort with no config loaded should create a default config.
	resetGlobalState(t)
	if err := SetPort("64999"); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if GetPort() != "64999" {
		t.Errorf("GetPort() = %q, want 64999", GetPort())
	}

	// With a config path set, SetPort must persist to disk.
	resetGlobalState(t)
	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetPort("64001"); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "64001") {
		t.Error("expected SetPort to persist port to disk")
	}
}

func TestSetUpdateBranch(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetUpdateBranch("beta"); err != nil {
		t.Fatalf("SetUpdateBranch: %v", err)
	}
	if Get().Server.Update.Branch != "beta" {
		t.Errorf("Update.Branch = %q, want beta", Get().Server.Update.Branch)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "beta") {
		t.Error("expected SetUpdateBranch to persist branch to disk")
	}
}

func TestSetLastNotifiedVersion(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetLastNotifiedVersion("1.2.3"); err != nil {
		t.Fatalf("SetLastNotifiedVersion: %v", err)
	}
	if Get().Server.Update.LastNotifiedVersion != "1.2.3" {
		t.Errorf("Update.LastNotifiedVersion = %q, want 1.2.3", Get().Server.Update.LastNotifiedVersion)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "1.2.3") {
		t.Error("expected SetLastNotifiedVersion to persist version to disk")
	}

	// Reloading from disk must round-trip the persisted value.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	if reloaded.Server.Update.LastNotifiedVersion != "1.2.3" {
		t.Errorf("reloaded Update.LastNotifiedVersion = %q, want 1.2.3", reloaded.Server.Update.LastNotifiedVersion)
	}
}

func TestSetSSLLastExpiryWarningDays(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetSSLLastExpiryWarningDays(7); err != nil {
		t.Fatalf("SetSSLLastExpiryWarningDays: %v", err)
	}
	if Get().Server.SSL.LastExpiryWarningDays != 7 {
		t.Errorf("SSL.LastExpiryWarningDays = %d, want 7", Get().Server.SSL.LastExpiryWarningDays)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "last_expiry_warning_days: 7") {
		t.Error("expected SetSSLLastExpiryWarningDays to persist value to disk")
	}

	// Reloading from disk must round-trip the persisted value.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	if reloaded.Server.SSL.LastExpiryWarningDays != 7 {
		t.Errorf("reloaded SSL.LastExpiryWarningDays = %d, want 7", reloaded.Server.SSL.LastExpiryWarningDays)
	}

	// Resetting to 0 (e.g. after a successful renewal) must also persist.
	if err := SetSSLLastExpiryWarningDays(0); err != nil {
		t.Fatalf("SetSSLLastExpiryWarningDays(0): %v", err)
	}
	if Get().Server.SSL.LastExpiryWarningDays != 0 {
		t.Errorf("SSL.LastExpiryWarningDays = %d, want 0", Get().Server.SSL.LastExpiryWarningDays)
	}
}

func TestGetTheme(t *testing.T) {
	resetGlobalState(t)
	if GetTheme() != "dark" {
		t.Errorf("GetTheme() = %q, want dark", GetTheme())
	}
}

func TestGetCORS(t *testing.T) {
	resetGlobalState(t)
	if got := GetCORS(); len(got) != 1 || got[0] != "*" {
		t.Errorf("GetCORS() default = %v, want [*]", got)
	}

	cfg := DefaultConfig()
	cfg.Server.CORS.AllowedOrigins = []string{"https://example.com"}
	if err := Update(cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := GetCORS(); len(got) != 1 || got[0] != "https://example.com" {
		t.Errorf("GetCORS() = %v, want [https://example.com]", got)
	}

	// Empty CORS must fall back to "*".
	cfg2 := DefaultConfig()
	cfg2.Server.CORS.AllowedOrigins = nil
	if err := Update(cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := GetCORS(); len(got) != 1 || got[0] != "*" {
		t.Errorf("GetCORS() with empty CORS = %v, want [*]", got)
	}
}

func TestFormatStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty", []string{}, "[]"},
		{"nil", nil, "[]"},
		{"single", []string{"US"}, `["US"]`},
		{"multiple", []string{"US", "CA"}, `["US", "CA"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatStringSlice(tt.input); got != tt.want {
				t.Errorf("formatStringSlice(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatAnnouncementMessages(t *testing.T) {
	if got := formatAnnouncementMessages(nil); got != "[]" {
		t.Errorf("formatAnnouncementMessages(nil) = %q, want []", got)
	}
	if got := formatAnnouncementMessages([]AnnouncementMessage{}); got != "[]" {
		t.Errorf("formatAnnouncementMessages(empty) = %q, want []", got)
	}

	msgs := []AnnouncementMessage{
		{
			ID:          "maintenance-1",
			Type:        "warning",
			Title:       "Scheduled maintenance",
			Message:     "Downtime expected",
			Start:       "2026-01-01T00:00:00Z",
			End:         "2026-01-02T00:00:00Z",
			Dismissible: true,
		},
	}
	got := formatAnnouncementMessages(msgs)
	for _, want := range []string{
		`id: "maintenance-1"`,
		`type: "warning"`,
		`title: "Scheduled maintenance"`,
		`message: "Downtime expected"`,
		`start: "2026-01-01T00:00:00Z"`,
		`end: "2026-01-02T00:00:00Z"`,
		`dismissible: true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAnnouncementMessages() = %q, want it to contain %q", got, want)
		}
	}
}

func TestValidateConfigFixesInvalidAnnouncementType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Web.Announcements.Messages = []AnnouncementMessage{
		{ID: "a", Type: "bogus"},
		{ID: "b", Type: "success"},
	}

	validateConfig(cfg)

	if cfg.Web.Announcements.Messages[0].Type != "info" {
		t.Errorf("Messages[0].Type = %q, want default %q", cfg.Web.Announcements.Messages[0].Type, "info")
	}
	if cfg.Web.Announcements.Messages[1].Type != "success" {
		t.Errorf("Messages[1].Type = %q, want untouched %q", cfg.Web.Announcements.Messages[1].Type, "success")
	}
}

func TestDefaultConfigNotifications(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Notifications.WebUI.Position != "top-right" {
		t.Errorf("WebUI.Position = %q, want top-right", cfg.Server.Notifications.WebUI.Position)
	}
	if cfg.Server.Notifications.WebUI.Duration != 5 {
		t.Errorf("WebUI.Duration = %d, want 5", cfg.Server.Notifications.WebUI.Duration)
	}
	if cfg.Server.Notifications.Email.Enabled {
		t.Error("Email.Enabled = true, want false by default")
	}
	if cfg.Server.Notifications.Email.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.Server.Notifications.Email.SMTP.Port)
	}
	if cfg.Server.Notifications.Email.SMTP.TLS != "auto" {
		t.Errorf("SMTP.TLS = %q, want auto", cfg.Server.Notifications.Email.SMTP.TLS)
	}
	if cfg.Server.Notifications.Email.SMTP.Host != "" {
		t.Errorf("SMTP.Host = %q, want empty by default", cfg.Server.Notifications.Email.SMTP.Host)
	}

	wantEvents := map[string]bool{
		"startup":            false,
		"shutdown":           false,
		"backup_complete":    false,
		"backup_failed":      true,
		"ssl_expiring":       true,
		"ssl_renewed":        false,
		"ssl_renewal_failed": true,
		"security_alert":     true,
		"scheduler_error":    true,
		"update_available":   false,
		"update_installed":   true,
	}
	for event, want := range wantEvents {
		if got := cfg.Server.Notifications.Email.Events[event]; got != want {
			t.Errorf("Events[%q] = %v, want %v", event, got, want)
		}
	}
	if len(cfg.Server.Notifications.Email.Events) != len(wantEvents) {
		t.Errorf("Events has %d entries, want %d", len(cfg.Server.Notifications.Email.Events), len(wantEvents))
	}
}

func TestApplySMTPEnvOverrides(t *testing.T) {
	envVars := map[string]string{
		"SMTP_HOST":       "smtp.example.com",
		"SMTP_PORT":       "465",
		"SMTP_USERNAME":   "operator",
		"SMTP_PASSWORD":   "s3cret",
		"SMTP_TLS":        "tls",
		"SMTP_FROM_NAME":  "Airports Ops",
		"SMTP_FROM_EMAIL": "ops@example.com",
	}
	for k, v := range envVars {
		t.Setenv(k, v)
	}

	cfg := DefaultConfig()
	applySMTPEnvOverrides(cfg)

	smtp := cfg.Server.Notifications.Email.SMTP
	if smtp.Host != "smtp.example.com" {
		t.Errorf("SMTP.Host = %q, want smtp.example.com", smtp.Host)
	}
	if smtp.Port != 465 {
		t.Errorf("SMTP.Port = %d, want 465", smtp.Port)
	}
	if smtp.Username != "operator" {
		t.Errorf("SMTP.Username = %q, want operator", smtp.Username)
	}
	if smtp.Password != "s3cret" {
		t.Errorf("SMTP.Password = %q, want s3cret", smtp.Password)
	}
	if smtp.TLS != "tls" {
		t.Errorf("SMTP.TLS = %q, want tls", smtp.TLS)
	}
	if cfg.Server.Notifications.Email.From.Name != "Airports Ops" {
		t.Errorf("From.Name = %q, want Airports Ops", cfg.Server.Notifications.Email.From.Name)
	}
	if cfg.Server.Notifications.Email.From.Email != "ops@example.com" {
		t.Errorf("From.Email = %q, want ops@example.com", cfg.Server.Notifications.Email.From.Email)
	}
}

func TestApplySMTPEnvOverridesInvalidPortIgnored(t *testing.T) {
	t.Setenv("SMTP_PORT", "not-a-number")

	cfg := DefaultConfig()
	applySMTPEnvOverrides(cfg)

	if cfg.Server.Notifications.Email.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want unchanged default 587 on invalid env value", cfg.Server.Notifications.Email.SMTP.Port)
	}
}

func TestApplySMTPEnvOverridesNoneSet(t *testing.T) {
	cfg := DefaultConfig()
	applySMTPEnvOverrides(cfg)

	if cfg.Server.Notifications.Email.SMTP.Host != "" {
		t.Errorf("SMTP.Host = %q, want empty when no env vars set", cfg.Server.Notifications.Email.SMTP.Host)
	}
	if cfg.Server.Notifications.Email.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want default 587 when no env vars set", cfg.Server.Notifications.Email.SMTP.Port)
	}
}

func TestApplyCacheEnvOverridesURLImpliesValkeyType(t *testing.T) {
	t.Setenv("CACHE_URL", "valkey://airports-cache:6379")

	cfg := DefaultConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.URL != "valkey://airports-cache:6379" {
		t.Errorf("Cache.URL = %q, want valkey://airports-cache:6379", cfg.Server.Cache.URL)
	}
	if cfg.Server.Cache.Type != "valkey" {
		t.Errorf("Cache.Type = %q, want valkey inferred from CACHE_URL", cfg.Server.Cache.Type)
	}
}

func TestApplyCacheEnvOverridesExplicitTypeWins(t *testing.T) {
	t.Setenv("CACHE_URL", "redis://cache.example.com:6379/1")
	t.Setenv("CACHE_TYPE", "redis")

	cfg := DefaultConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.Type != "redis" {
		t.Errorf("Cache.Type = %q, want redis", cfg.Server.Cache.Type)
	}
	if cfg.Server.Cache.URL != "redis://cache.example.com:6379/1" {
		t.Errorf("Cache.URL = %q, want redis://cache.example.com:6379/1", cfg.Server.Cache.URL)
	}
}

func TestApplyCacheEnvOverridesTypeOnlyDoesNotForceURL(t *testing.T) {
	t.Setenv("CACHE_TYPE", "none")

	cfg := DefaultConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.Type != "none" {
		t.Errorf("Cache.Type = %q, want none", cfg.Server.Cache.Type)
	}
	if cfg.Server.Cache.URL != "" {
		t.Errorf("Cache.URL = %q, want empty when only CACHE_TYPE set", cfg.Server.Cache.URL)
	}
}

func TestApplyCacheEnvOverridesNoneSet(t *testing.T) {
	cfg := DefaultConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.Type != "memory" {
		t.Errorf("Cache.Type = %q, want unchanged default memory when no env vars set", cfg.Server.Cache.Type)
	}
	if cfg.Server.Cache.URL != "" {
		t.Errorf("Cache.URL = %q, want empty when no env vars set", cfg.Server.Cache.URL)
	}
}

func TestValidateConfigReplacesInvalidValuesWithDefaults(t *testing.T) {
	def := DefaultConfig()
	cfg := DefaultConfig()

	cfg.Server.RateLimit.Read = RateLimitBucket{Requests: -1, Window: 0}
	cfg.Server.RateLimit.Write = RateLimitBucket{Requests: 0, Window: -5}
	cfg.Server.RateLimit.Health = RateLimitBucket{Requests: 5, Window: 5}
	cfg.Server.RateLimit.GlobalBurst = 0
	cfg.Server.Database.Pool.MaxOpen = -1
	cfg.Server.Database.Pool.MaxIdle = 999
	cfg.Server.Cache.PoolSize = 0
	cfg.Server.Cache.MinIdle = -1
	cfg.Server.Maintenance.Cleanup.DiskThreshold = 150
	cfg.Server.Maintenance.Cleanup.LogRetentionDays = 0
	cfg.Server.Maintenance.Cleanup.BackupKeepCount = -3

	validateConfig(cfg)

	if cfg.Server.RateLimit.Read != def.Server.RateLimit.Read {
		t.Errorf("RateLimit.Read = %+v, want default %+v", cfg.Server.RateLimit.Read, def.Server.RateLimit.Read)
	}
	if cfg.Server.RateLimit.Write != def.Server.RateLimit.Write {
		t.Errorf("RateLimit.Write = %+v, want default %+v", cfg.Server.RateLimit.Write, def.Server.RateLimit.Write)
	}
	// Health bucket was already valid (5/5) - must be left untouched.
	if cfg.Server.RateLimit.Health != (RateLimitBucket{Requests: 5, Window: 5}) {
		t.Errorf("RateLimit.Health = %+v, want untouched {5 5}", cfg.Server.RateLimit.Health)
	}
	if cfg.Server.RateLimit.GlobalBurst != def.Server.RateLimit.GlobalBurst {
		t.Errorf("RateLimit.GlobalBurst = %d, want default %d", cfg.Server.RateLimit.GlobalBurst, def.Server.RateLimit.GlobalBurst)
	}
	if cfg.Server.Database.Pool.MaxOpen != def.Server.Database.Pool.MaxOpen {
		t.Errorf("Database.Pool.MaxOpen = %d, want default %d", cfg.Server.Database.Pool.MaxOpen, def.Server.Database.Pool.MaxOpen)
	}
	if cfg.Server.Database.Pool.MaxIdle != def.Server.Database.Pool.MaxIdle {
		t.Errorf("Database.Pool.MaxIdle = %d, want default %d (exceeded max_open)", cfg.Server.Database.Pool.MaxIdle, def.Server.Database.Pool.MaxIdle)
	}
	if cfg.Server.Cache.PoolSize != def.Server.Cache.PoolSize {
		t.Errorf("Cache.PoolSize = %d, want default %d", cfg.Server.Cache.PoolSize, def.Server.Cache.PoolSize)
	}
	if cfg.Server.Cache.MinIdle != def.Server.Cache.MinIdle {
		t.Errorf("Cache.MinIdle = %d, want default %d", cfg.Server.Cache.MinIdle, def.Server.Cache.MinIdle)
	}
	if cfg.Server.Maintenance.Cleanup.DiskThreshold != def.Server.Maintenance.Cleanup.DiskThreshold {
		t.Errorf("Cleanup.DiskThreshold = %d, want default %d", cfg.Server.Maintenance.Cleanup.DiskThreshold, def.Server.Maintenance.Cleanup.DiskThreshold)
	}
	if cfg.Server.Maintenance.Cleanup.LogRetentionDays != def.Server.Maintenance.Cleanup.LogRetentionDays {
		t.Errorf("Cleanup.LogRetentionDays = %d, want default %d", cfg.Server.Maintenance.Cleanup.LogRetentionDays, def.Server.Maintenance.Cleanup.LogRetentionDays)
	}
	if cfg.Server.Maintenance.Cleanup.BackupKeepCount != def.Server.Maintenance.Cleanup.BackupKeepCount {
		t.Errorf("Cleanup.BackupKeepCount = %d, want default %d", cfg.Server.Maintenance.Cleanup.BackupKeepCount, def.Server.Maintenance.Cleanup.BackupKeepCount)
	}
}

func TestValidateConfigLeavesValidValuesUntouched(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.RateLimit.GlobalBurst = 500
	cfg.Server.Database.Pool.MaxOpen = 20
	cfg.Server.Database.Pool.MaxIdle = 20
	cfg.Server.Cache.PoolSize = 50
	cfg.Server.Cache.MinIdle = 0
	cfg.Server.Maintenance.Cleanup.DiskThreshold = 1
	cfg.Server.Maintenance.Cleanup.BackupKeepCount = 0

	validateConfig(cfg)

	if cfg.Server.RateLimit.GlobalBurst != 500 {
		t.Errorf("RateLimit.GlobalBurst = %d, want untouched 500", cfg.Server.RateLimit.GlobalBurst)
	}
	if cfg.Server.Database.Pool.MaxOpen != 20 || cfg.Server.Database.Pool.MaxIdle != 20 {
		t.Errorf("Database.Pool = %+v, want untouched {MaxOpen:20 MaxIdle:20 ...}", cfg.Server.Database.Pool)
	}
	if cfg.Server.Cache.PoolSize != 50 || cfg.Server.Cache.MinIdle != 0 {
		t.Errorf("Cache pool = {%d %d}, want untouched {50 0}", cfg.Server.Cache.PoolSize, cfg.Server.Cache.MinIdle)
	}
	if cfg.Server.Maintenance.Cleanup.DiskThreshold != 1 {
		t.Errorf("Cleanup.DiskThreshold = %d, want untouched 1", cfg.Server.Maintenance.Cleanup.DiskThreshold)
	}
	if cfg.Server.Maintenance.Cleanup.BackupKeepCount != 0 {
		t.Errorf("Cleanup.BackupKeepCount = %d, want untouched 0", cfg.Server.Maintenance.Cleanup.BackupKeepCount)
	}
}

func TestSanitizedRedactsSMTPPassword(t *testing.T) {
	resetGlobalState(t)
	cfg := DefaultConfig()
	cfg.Server.Notifications.Email.SMTP.Password = "s3cret"
	mu.Lock()
	current = cfg
	mu.Unlock()

	sanitized := Sanitized()

	if sanitized.Server.Notifications.Email.SMTP.Password != redactedValue {
		t.Errorf("Sanitized SMTP.Password = %q, want %q", sanitized.Server.Notifications.Email.SMTP.Password, redactedValue)
	}
	if cfg.Server.Notifications.Email.SMTP.Password != "s3cret" {
		t.Error("Sanitized() must not mutate the original config's password")
	}
}
