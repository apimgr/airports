package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"gopkg.in/yaml.v3"
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
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove test temp directory: %v", err)
		}
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

func TestValidateConfigPreservesRequiredLanguages(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Server.I18n.Supported = []string{"ja"}
	cfg.Server.I18n.DefaultLanguage = "ja"
	validateConfig(cfg)
	want := []string{"ja", "en", "es", "fr", "ar", "de", "it"}
	if strings.Join(cfg.Server.I18n.Supported, ",") != strings.Join(want, ",") {
		t.Errorf("I18n.Supported = %v, want required languages added to requested locale", cfg.Server.I18n.Supported)
	}
	if cfg.Server.I18n.DefaultLanguage != "ja" {
		t.Errorf("I18n.DefaultLanguage = %q, want ja", cfg.Server.I18n.DefaultLanguage)
	}
}

func TestDefaultApplicationConfig(t *testing.T) {
	cfg := DefaultApplicationConfig()
	if cfg.Server.Privacy.Consent.Policy.Version != "1" {
		t.Errorf("Consent.Policy.Version = %q, want default version 1", cfg.Server.Privacy.Consent.Policy.Version)
	}
	antispam := cfg.Server.Pages.Contact.Antispam
	if !antispam.Honeypot || antispam.MinFillSeconds != 3 || antispam.MaxAgeSeconds != 43200 || antispam.PerIP != 5 || antispam.PerEmail != 3 || antispam.Global != 60 || antispam.Window != "1h" || antispam.MaxLinks != 10 || !antispam.RejectMarkup || antispam.Strikes != 3 || antispam.BanHours != 24 || antispam.ReviewMinSeconds != 2 || antispam.ReviewMaxSeconds != 900 || antispam.DuplicateWindowHours != 24 {
		t.Errorf("Contact.Antispam = %+v, want AI.md PART 16 defaults", antispam)
	}
	if cfg.Server.Limits != (LimitsConfig{MaxBodySize: "10MB", ReadTimeout: "30s", WriteTimeout: "30s", IdleTimeout: "120s"}) {
		t.Errorf("Server.Limits = %+v, want PART 12 defaults", cfg.Server.Limits)
	}
	if !cfg.Server.Compression.Enabled || cfg.Server.Compression.Level != 5 || strings.Join(cfg.Server.Compression.Types, ",") != strings.Join(defaultCompressionTypes, ",") {
		t.Errorf("Server.Compression = %+v, want enabled level 5 and default MIME types", cfg.Server.Compression)
	}
	if cfg.Server.Lang != "auto" {
		t.Errorf("Lang = %q, want auto", cfg.Server.Lang)
	}
	if cfg.Server.BaseURL != "/" {
		t.Errorf("BaseURL = %q, want /", cfg.Server.BaseURL)
	}
	if cfg.Server.URLDetection != (URLDetectionConfig{Learning: true, MinSamples: 3, SampleWindow: "5m", LogChanges: true, LiveReload: true}) {
		t.Errorf("URLDetection = %+v, want the AI.md defaults", cfg.Server.URLDetection)
	}
	if cfg.Server.Address != "[::]" {
		t.Errorf("Address = %q, want [::]", cfg.Server.Address)
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("Mode = %q, want production", cfg.Server.Mode)
	}
	if cfg.Server.I18n.DefaultLanguage != "en" {
		t.Errorf("I18n.DefaultLanguage = %q, want en", cfg.Server.I18n.DefaultLanguage)
	}
	wantLanguages := i18n.RegisteredLanguageCodes()
	if strings.Join(cfg.Server.I18n.Supported, ",") != strings.Join(wantLanguages, ",") {
		t.Errorf("I18n.Supported = %v, want %v", cfg.Server.I18n.Supported, wantLanguages)
	}
	if !cfg.Server.Privacy.Data.StoredOnServer {
		t.Error("Privacy.Data.StoredOnServer = false, want true")
	}
	if cfg.Server.Update.Branch != "stable" {
		t.Errorf("Update.Branch = %q, want stable", cfg.Server.Update.Branch)
	}
	if cfg.Server.Tokens != (OwnerTokensConfig{MaxLinked: 25, CookieMaxAge: "1y", IdlePurgeDays: 0}) {
		t.Errorf("Server.Tokens = %+v, want PART 8 defaults", cfg.Server.Tokens)
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

func TestLoadApplicationConfigDoesNotCreateAreaFiles(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	if _, err := LoadApplicationConfig(filepath.Join(dir, "server.yml")); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "yaml.d")); !os.IsNotExist(err) {
		t.Fatalf("default configuration created yaml.d unexpectedly: stat error = %v", err)
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	cfg, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to be created at %s: %v", path, err)
	}
	if GetApplicationConfig().Server.Mode != "production" {
		t.Errorf("GetApplicationConfig().Server.Mode = %q, want production", GetApplicationConfig().Server.Mode)
	}
	if !strings.HasPrefix(cfg.Server.Token, "adm_") || len(cfg.Server.Token) != len("adm_")+32 {
		t.Errorf("generated operator token has format %q, want adm_ plus 32 characters", cfg.Server.Token)
	}
	serverYAML, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated server.yml: %v", err)
	}
	serverInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated server.yml: %v", err)
	}
	if serverInfo.Mode().Perm() != 0600 {
		t.Errorf("server.yml permissions = %04o, want 0600", serverInfo.Mode().Perm())
	}
	configDirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat config directory: %v", err)
	}
	if configDirInfo.Mode().Perm() != 0700 {
		t.Errorf("config directory permissions = %04o, want 0700", configDirInfo.Mode().Perm())
	}
	if strings.Contains(string(serverYAML), cfg.Server.Token) {
		t.Error("server.yml contains the operator token")
	}
	if !strings.HasPrefix(string(serverYAML), "# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.") || strings.Contains(string(serverYAML), "  scheduler:") || strings.Contains(string(serverYAML), "  metrics:") || strings.Contains(string(serverYAML), "web:") {
		t.Errorf("generated server.yml contains non-node settings or lacks its required header: %s", serverYAML)
	}
	secretPath := filepath.Join(dir, "secrets.yml")
	secretInfo, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("stat generated secrets.yml: %v", err)
	}
	if secretInfo.Mode().Perm() != 0600 {
		t.Errorf("secrets.yml permissions = %04o, want 0600", secretInfo.Mode().Perm())
	}
	resetGlobalState(t)
	reloaded, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("reload config with secrets.yml: %v", err)
	}
	if reloaded.Server.Token != cfg.Server.Token {
		t.Error("operator token did not survive a reload from secrets.yml")
	}
}

func TestLoadGeneratesAndPersistsWebhookSigningSecret(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	const endpoint = "https://hooks.example.test/contact"
	if err := os.WriteFile(path, []byte("server:\n  contact:\n    abuse:\n      webhooks:\n        generic: "+endpoint+"\n"), 0600); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	first, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	secret := first.Server.Contact.Abuse.Webhooks["generic_secret"]
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("generated webhook signing secret has invalid encoding/size: len=%d err=%v", len(decoded), err)
	}
	if !strings.Contains(strings.Join(first.GeneratedSecretPaths(), "\n"), "server.contact.abuse.webhooks.generic_secret") {
		t.Fatalf("generated secret path missing from audit metadata: %v", first.GeneratedSecretPaths())
	}
	secrets, err := os.ReadFile(filepath.Join(dir, "secrets.yml"))
	if err != nil {
		t.Fatalf("read secrets.yml: %v", err)
	}
	if !strings.Contains(string(secrets), "generic_secret") || !strings.Contains(string(secrets), secret) || !strings.Contains(string(secrets), endpoint) {
		t.Fatalf("secrets.yml does not contain the webhook URL and signing key")
	}
	serverYAML, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read server.yml: %v", err)
	}
	if strings.Contains(string(serverYAML), secret) || strings.Contains(string(serverYAML), endpoint) {
		t.Fatal("server.yml contains webhook secrets or destination URLs")
	}

	resetGlobalState(t)
	second, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if second.Server.Contact.Abuse.Webhooks["generic_secret"] != secret {
		t.Fatal("webhook signing secret changed during reload")
	}
	if second.Server.Contact.ResolveWebhook("abuse", "generic") != endpoint || second.Server.Contact.ResolveWebhookSigningSecret("abuse", "generic") != secret {
		t.Fatal("webhook URL and signing secret did not resolve as a pair")
	}
}

func TestInitOnlyApplicationEnvironment(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	t.Setenv("APPLICATION_NAME", "Configured Airports")
	t.Setenv("APPLICATION_TAGLINE", "Airport data at a glance")

	first, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if first.Server.Branding.Title != "Configured Airports" || first.Server.Branding.Tagline != "Airport data at a glance" {
		t.Fatalf("first-run branding = %+v", first.Server.Branding)
	}

	resetGlobalState(t)
	t.Setenv("APPLICATION_NAME", "Ignored Name")
	t.Setenv("APPLICATION_TAGLINE", "Ignored Tagline")
	second, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if second.Server.Branding.Title != "Configured Airports" || second.Server.Branding.Tagline != "Airport data at a glance" {
		t.Fatalf("existing config was overridden by init-only environment: %+v", second.Server.Branding)
	}
}

func TestDatabaseEnvironmentOverridesAreRuntimeOnly(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  database:\n    driver: sqlite\n    url: ''\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("DATABASE_DRIVER", "SQLite3")
	t.Setenv("DATABASE_URL", "file:runtime.db")

	cfg, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Database.Driver != "sqlite" || cfg.Server.Database.URL != "file:runtime.db" {
		t.Errorf("database config = %+v, want runtime SQLite URL", cfg.Server.Database)
	}

	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(persisted), "runtime.db") || strings.Contains(string(persisted), "driver: sqlite3") {
		t.Errorf("runtime database override was persisted to server.yml: %s", persisted)
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

	cfg, err := LoadApplicationConfig(path)
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

	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load should ignore invalid YAML and use defaults: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "server: [this is not: valid: yaml" {
		t.Errorf("invalid config was modified: data=%q err=%v", data, err)
	}
}

func TestLoadInvalidYAMLPreservesLastGoodConfig(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  port: \"12345\"\n  fqdn: \"example.com\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("server:\n  port: [invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("Load invalid replacement: %v", err)
	}
	if cfg.Server.Port != "12345" || cfg.Server.FQDN != "example.com" {
		t.Fatalf("invalid reload lost last good config: port=%q fqdn=%q", cfg.Server.Port, cfg.Server.FQDN)
	}
}

func TestValidateYAMLDocumentRejectsNullAndDuplicateKeys(t *testing.T) {
	for _, input := range []string{"server:\n  port: null\n", "server:\n  port: 1\n  port: 2\n"} {
		if err := validateYAMLDocument([]byte(input)); err == nil {
			t.Errorf("validateYAMLDocument(%q) succeeded, want invalid document", input)
		}
	}
}

func TestApplyConfigYAMLDefaultsOnlyInvalidKeys(t *testing.T) {
	for _, content := range []string{
		"server:\n  port: null\n  fqdn: example.com\n",
		"server:\n  port: first\n  port: second\n  fqdn: example.com\n",
		"server:\n  port: [invalid]\n  fqdn: example.com\n",
	} {
		cfg := DefaultApplicationConfig()
		if err := applyConfigYAML(cfg, []byte(content), "test.yml"); err != nil {
			t.Fatalf("applyConfigYAML(%q): %v", content, err)
		}
		validateConfig(cfg)
		if cfg.Server.Port != DefaultApplicationConfig().Server.Port || cfg.Server.FQDN != "example.com" {
			t.Errorf("invalid value did not reset independently for %q: port=%q fqdn=%q", content, cfg.Server.Port, cfg.Server.FQDN)
		}
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
	cfg := GetApplicationConfig()
	if cfg == nil {
		t.Fatal("GetApplicationConfig() returned nil")
	}
	if cfg.Server.Mode != "production" {
		t.Errorf("GetApplicationConfig() default Mode = %q, want production", cfg.Server.Mode)
	}
}

func TestSaveNilConfig(t *testing.T) {
	resetGlobalState(t)
	if err := SaveApplicationConfig("/tmp/apimgr/should-not-be-created.yml", nil); err == nil {
		t.Error("SaveApplicationConfig(nil) expected error, got nil")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	cfg := DefaultApplicationConfig()
	cfg.Server.FQDN = "roundtrip.example.com"
	cfg.Server.Port = "64500"
	cfg.Server.BaseURL = "/myproject"
	cfg.Server.URLDetection = URLDetectionConfig{Learning: false, MinSamples: 5, SampleWindow: "10m", LogChanges: false, LiveReload: false}
	cfg.Server.GeoIP.AllowCountries = []string{"US", "CA"}

	if err := SaveApplicationConfig(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	resetGlobalState(t)
	loaded, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Server.FQDN != "roundtrip.example.com" {
		t.Errorf("FQDN = %q, want roundtrip.example.com", loaded.Server.FQDN)
	}
	if loaded.Server.Port != "64500" {
		t.Errorf("Port = %q, want 64500", loaded.Server.Port)
	}
	if loaded.Server.BaseURL != "/myproject" {
		t.Errorf("BaseURL = %q, want /myproject", loaded.Server.BaseURL)
	}
	if loaded.Server.URLDetection != cfg.Server.URLDetection {
		t.Errorf("URLDetection = %+v, want %+v", loaded.Server.URLDetection, cfg.Server.URLDetection)
	}
	if len(loaded.Server.GeoIP.AllowCountries) != 2 {
		t.Errorf("AllowCountries = %v, want 2 entries", loaded.Server.GeoIP.AllowCountries)
	}
}

func TestSaveCurrentNoConfigLoaded(t *testing.T) {
	resetGlobalState(t)
	if err := SaveCurrentApplicationConfig(); err == nil {
		t.Error("SaveCurrentApplicationConfig() with no config loaded expected error, got nil")
	}
}

func TestSaveCurrentPersists(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	mu.Lock()
	current.Server.FQDN = "savecurrent.example.com"
	mu.Unlock()

	if err := SaveCurrentApplicationConfig(); err != nil {
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
	cfg := DefaultApplicationConfig()
	cfg.Server.FQDN = "noupdatepath.example.com"
	if err := UpdateApplicationConfig(cfg); err != nil {
		t.Fatalf("Update with no path: %v", err)
	}
	if GetApplicationConfig().Server.FQDN != "noupdatepath.example.com" {
		t.Error("Update should set current config even without a path")
	}

	// Now load with a path, then Update should persist to disk.
	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg2 := DefaultApplicationConfig()
	cfg2.Server.FQDN = "updated.example.com"
	if err := UpdateApplicationConfig(cfg2); err != nil {
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
	if err := ReloadApplicationConfig(); err == nil {
		t.Error("ReloadApplicationConfig() with no config path expected error, got nil")
	}
}

func TestReload(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := LoadApplicationConfig(path); err != nil {
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

	if err := ReloadApplicationConfig(); err != nil {
		t.Fatalf("ReloadApplicationConfig: %v", err)
	}
	if GetApplicationConfig().Server.FQDN != "reloaded.example.com" {
		t.Errorf("GetApplicationConfig().Server.FQDN = %q, want reloaded.example.com", GetApplicationConfig().Server.FQDN)
	}
}

func TestGetConfiguredServerPortSetConfiguredServerPort(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	// SetConfiguredServerPort with no config loaded should create a default config.
	resetGlobalState(t)
	if err := SetConfiguredServerPort("64999"); err != nil {
		t.Fatalf("SetConfiguredServerPort: %v", err)
	}
	if GetConfiguredServerPort() != "64999" {
		t.Errorf("GetConfiguredServerPort() = %q, want 64999", GetConfiguredServerPort())
	}

	// With a config path set, SetConfiguredServerPort must persist to disk.
	resetGlobalState(t)
	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetConfiguredServerPort("64001"); err != nil {
		t.Fatalf("SetConfiguredServerPort: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "64001") {
		t.Error("expected SetConfiguredServerPort to persist port to disk")
	}
}

func TestSetUpdateBranch(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetUpdateBranch("beta"); err != nil {
		t.Fatalf("SetUpdateBranch: %v", err)
	}
	if GetApplicationConfig().Server.Update.Branch != "beta" {
		t.Errorf("Update.Branch = %q, want beta", GetApplicationConfig().Server.Update.Branch)
	}
	data, err := os.ReadFile(filepath.Join(dir, "yaml.d", "maintenance.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "beta") {
		t.Error("expected SetUpdateBranch to persist branch to disk")
	}
}

func TestUpdateBranchAliasesNormalizeToDevel(t *testing.T) {
	for _, alias := range []string{"devel", "daily", "rolling", "dev", "development"} {
		t.Run(alias, func(t *testing.T) {
			got, ok := NormalizeUpdateBranch(alias)
			if !ok || got != "devel" {
				t.Fatalf("NormalizeUpdateBranch(%q) = %q, %v; want devel, true", alias, got, ok)
			}
		})
	}
	if got, ok := NormalizeUpdateBranch("unknown"); ok || got != "" {
		t.Errorf("NormalizeUpdateBranch(unknown) = %q, %v; want empty, false", got, ok)
	}
}

func TestSetUpdateBranchPersistsCanonicalAlias(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetUpdateBranch("daily"); err != nil {
		t.Fatalf("SetUpdateBranch(daily): %v", err)
	}
	if got := GetApplicationConfig().Server.Update.Branch; got != "devel" {
		t.Fatalf("Update.Branch = %q, want devel", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "yaml.d", "maintenance.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "branch: devel") {
		t.Errorf("maintenance.yml did not persist canonical devel branch")
	}
}

func TestLoadCanonicalizesStoredUpdateBranchAlias(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("server:\n  update:\n    branch: rolling\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := LoadApplicationConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Update.Branch != "devel" {
		t.Fatalf("Update.Branch = %q, want devel", cfg.Server.Update.Branch)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "yaml.d", "maintenance.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(saved), "branch: devel") {
		t.Errorf("maintenance.yml did not persist canonical devel branch")
	}
}

func TestSetLastNotifiedVersion(t *testing.T) {
	resetGlobalState(t)
	dir := newTestDir(t)
	path := filepath.Join(dir, "server.yml")

	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetLastNotifiedVersion("1.2.3"); err != nil {
		t.Fatalf("SetLastNotifiedVersion: %v", err)
	}
	if GetApplicationConfig().Server.Update.LastNotifiedVersion != "1.2.3" {
		t.Errorf("Update.LastNotifiedVersion = %q, want 1.2.3", GetApplicationConfig().Server.Update.LastNotifiedVersion)
	}
	data, err := os.ReadFile(filepath.Join(dir, "yaml.d", "maintenance.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "1.2.3") {
		t.Error("expected SetLastNotifiedVersion to persist version to disk")
	}

	// Reloading from disk must round-trip the persisted value.
	reloaded, err := LoadApplicationConfig(path)
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

	if _, err := LoadApplicationConfig(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetSSLLastExpiryWarningDays(7); err != nil {
		t.Fatalf("SetSSLLastExpiryWarningDays: %v", err)
	}
	if GetApplicationConfig().Server.SSL.LastExpiryWarningDays != 7 {
		t.Errorf("SSL.LastExpiryWarningDays = %d, want 7", GetApplicationConfig().Server.SSL.LastExpiryWarningDays)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "last_expiry_warning_days: 7") {
		t.Error("expected SetSSLLastExpiryWarningDays to persist value to disk")
	}

	// Reloading from disk must round-trip the persisted value.
	reloaded, err := LoadApplicationConfig(path)
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
	if GetApplicationConfig().Server.SSL.LastExpiryWarningDays != 0 {
		t.Errorf("SSL.LastExpiryWarningDays = %d, want 0", GetApplicationConfig().Server.SSL.LastExpiryWarningDays)
	}
}

func TestGetTheme(t *testing.T) {
	resetGlobalState(t)
	if GetConfiguredWebTheme() != "dark" {
		t.Errorf("GetConfiguredWebTheme() = %q, want dark", GetConfiguredWebTheme())
	}
}

func TestGetCORS(t *testing.T) {
	resetGlobalState(t)
	if got := GetConfiguredCORSOrigins(); len(got) != 1 || got[0] != "*" {
		t.Errorf("GetConfiguredCORSOrigins() default = %v, want [*]", got)
	}

	cfg := DefaultApplicationConfig()
	cfg.Server.CORS.AllowedOrigins = []string{"https://example.com"}
	if err := UpdateApplicationConfig(cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := GetConfiguredCORSOrigins(); len(got) != 1 || got[0] != "https://example.com" {
		t.Errorf("GetConfiguredCORSOrigins() = %v, want [https://example.com]", got)
	}

	// Empty CORS must fall back to "*".
	cfg2 := DefaultApplicationConfig()
	cfg2.Server.CORS.AllowedOrigins = nil
	if err := UpdateApplicationConfig(cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := GetConfiguredCORSOrigins(); len(got) != 1 || got[0] != "*" {
		t.Errorf("GetConfiguredCORSOrigins() with empty CORS = %v, want [*]", got)
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
	cfg := DefaultApplicationConfig()
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

func TestDefaultApplicationConfigNotifications(t *testing.T) {
	cfg := DefaultApplicationConfig()

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
		"startup":                        false,
		"shutdown":                       false,
		"backup_complete":                false,
		"backup_failed":                  true,
		"ssl_expiring":                   true,
		"ssl_renewed":                    false,
		"ssl_renewal_failed":             true,
		"security_alert":                 true,
		"security_report_received":       true,
		"security_report_acknowledgment": true,
		"security_report_status":         true,
		"scheduler_error":                true,
		"update_available":               false,
		"update_installed":               true,
		"update_failed":                  true,
		"update_staged":                  true,
		"disk_low":                       true,
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

	cfg := DefaultApplicationConfig()
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

	cfg := DefaultApplicationConfig()
	applySMTPEnvOverrides(cfg)

	if cfg.Server.Notifications.Email.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want unchanged default 587 on invalid env value", cfg.Server.Notifications.Email.SMTP.Port)
	}
}

func TestApplySMTPEnvOverridesNoneSet(t *testing.T) {
	cfg := DefaultApplicationConfig()
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

	cfg := DefaultApplicationConfig()
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

	cfg := DefaultApplicationConfig()
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

	cfg := DefaultApplicationConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.Type != "none" {
		t.Errorf("Cache.Type = %q, want none", cfg.Server.Cache.Type)
	}
	if cfg.Server.Cache.URL != "" {
		t.Errorf("Cache.URL = %q, want empty when only CACHE_TYPE set", cfg.Server.Cache.URL)
	}
}

func TestApplyCacheEnvOverridesNoneSet(t *testing.T) {
	cfg := DefaultApplicationConfig()
	applyCacheEnvOverrides(cfg)

	if cfg.Server.Cache.Type != "memory" {
		t.Errorf("Cache.Type = %q, want unchanged default memory when no env vars set", cfg.Server.Cache.Type)
	}
	if cfg.Server.Cache.URL != "" {
		t.Errorf("Cache.URL = %q, want empty when no env vars set", cfg.Server.Cache.URL)
	}
}

func TestValidateConfigReplacesInvalidValuesWithDefaults(t *testing.T) {
	def := DefaultApplicationConfig()
	cfg := DefaultApplicationConfig()

	cfg.Server.RateLimit.Read = RateLimitBucket{Requests: -1, Window: 0}
	cfg.Server.RateLimit.Write = RateLimitBucket{Requests: 0, Window: -5}
	cfg.Server.RateLimit.Health = RateLimitBucket{Requests: 5, Window: 5}
	cfg.Server.RateLimit.Strict = RateLimitBucket{Requests: 0, Window: 0}
	cfg.Server.RateLimit.AuthFailures = RateLimitBucket{Requests: -1, Window: 0}
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
	if cfg.Server.RateLimit.Strict != def.Server.RateLimit.Strict {
		t.Errorf("RateLimit.Strict = %+v, want default %+v", cfg.Server.RateLimit.Strict, def.Server.RateLimit.Strict)
	}
	if cfg.Server.RateLimit.AuthFailures != def.Server.RateLimit.AuthFailures {
		t.Errorf("RateLimit.AuthFailures = %+v, want default %+v", cfg.Server.RateLimit.AuthFailures, def.Server.RateLimit.AuthFailures)
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

func TestValidateConfigReplacesInvalidServerValues(t *testing.T) {
	defaults := DefaultApplicationConfig()
	cfg := DefaultApplicationConfig()
	cfg.Server.Port = "80,443"
	cfg.Server.Mode = "invalid"
	cfg.Server.Address = "bad address"
	cfg.Server.APIVersion = "v1/../v2"
	cfg.Server.SSL.MinVersion = "TLS1.0"
	cfg.Web.UI.Theme = "blue"
	cfg.Server.Cache.Type = "memcached"
	cfg.Server.Compression.Level = 10
	cfg.Server.Compression.Types = []string{"application/json; invalid"}
	cfg.Server.Limits = LimitsConfig{MaxBodySize: "0MB", ReadTimeout: "never", WriteTimeout: "0s", IdleTimeout: "-1s"}
	cfg.Server.Scheduler.Tasks.SSLRenewal.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.GeoIPUpdate.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.BlocklistUpdate.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.CVEUpdate.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.UpdateCheck.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.TokenCleanup.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.LogRotation.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.Backup.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.BackupHourly.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.HealthCheck.Schedule = "invalid"
	cfg.Server.Scheduler.Tasks.TorHealth.Schedule = "invalid"
	cfg.Server.Tokens = OwnerTokensConfig{MaxLinked: -1, CookieMaxAge: "invalid", IdlePurgeDays: -1}

	validateConfig(cfg)

	if cfg.Server.Port != defaults.Server.Port {
		t.Errorf("Server.Port = %q, want default %q", cfg.Server.Port, defaults.Server.Port)
	}
	if cfg.Server.Mode != defaults.Server.Mode {
		t.Errorf("Server.Mode = %q, want default %q", cfg.Server.Mode, defaults.Server.Mode)
	}
	if cfg.Server.Tokens != defaults.Server.Tokens {
		t.Errorf("Server.Tokens = %+v, want defaults %+v", cfg.Server.Tokens, defaults.Server.Tokens)
	}
	if cfg.Server.Address != defaults.Server.Address {
		t.Errorf("Server.Address = %q, want default %q", cfg.Server.Address, defaults.Server.Address)
	}
	if cfg.Server.APIVersion != defaults.Server.APIVersion {
		t.Errorf("Server.APIVersion = %q, want default %q", cfg.Server.APIVersion, defaults.Server.APIVersion)
	}
	if cfg.Server.SSL.MinVersion != defaults.Server.SSL.MinVersion {
		t.Errorf("SSL.MinVersion = %q, want default %q", cfg.Server.SSL.MinVersion, defaults.Server.SSL.MinVersion)
	}
	if cfg.Web.UI.Theme != defaults.Web.UI.Theme {
		t.Errorf("Web.UI.Theme = %q, want default %q", cfg.Web.UI.Theme, defaults.Web.UI.Theme)
	}
	if cfg.Server.Cache.Type != defaults.Server.Cache.Type {
		t.Errorf("Cache.Type = %q, want default %q", cfg.Server.Cache.Type, defaults.Server.Cache.Type)
	}
	if cfg.Server.Compression.Level != defaults.Server.Compression.Level || strings.Join(cfg.Server.Compression.Types, ",") != strings.Join(defaults.Server.Compression.Types, ",") {
		t.Errorf("Compression = %+v, want defaults %+v", cfg.Server.Compression, defaults.Server.Compression)
	}
	if got, want := cfg.Server.Scheduler.Tasks, defaults.Server.Scheduler.Tasks; got != want {
		t.Errorf("Scheduler.Tasks schedules were not restored to defaults: got %+v, want %+v", got, want)
	}
	if cfg.Server.Limits != defaults.Server.Limits {
		t.Errorf("Server.Limits = %+v, want defaults %+v", cfg.Server.Limits, defaults.Server.Limits)
	}
}

func TestValidateConfigPreservesValidServerValues(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Server.Port = "8090,8443"
	cfg.Server.SSL.Enabled = true
	cfg.Server.Mode = "dev"
	cfg.Server.Address = "[::1]"
	cfg.Server.APIVersion = "v2"
	cfg.Server.SSL.MinVersion = "TLS1.3"
	cfg.Web.UI.Theme = "light"
	cfg.Server.Cache.Type = "valkey"
	cfg.Server.Compression = CompressionConfig{Enabled: true, Level: 3, Types: []string{"text/plain", "application/*"}}
	cfg.Server.Limits = LimitsConfig{MaxBodySize: "2MB", ReadTimeout: "4s", WriteTimeout: "5s", IdleTimeout: "90s"}
	cfg.Server.Scheduler.Tasks.TokenCleanup.Schedule = "@every 15m"

	validateConfig(cfg)

	if cfg.Server.Port != "8090,8443" || cfg.Server.Mode != "dev" || cfg.Server.Address != "[::1]" || cfg.Server.APIVersion != "v2" {
		t.Errorf("valid server values changed: port=%q mode=%q address=%q api_version=%q", cfg.Server.Port, cfg.Server.Mode, cfg.Server.Address, cfg.Server.APIVersion)
	}
	if cfg.Server.SSL.MinVersion != "TLS1.3" || cfg.Web.UI.Theme != "light" || cfg.Server.Cache.Type != "valkey" {
		t.Errorf("valid option changed: min_version=%q theme=%q cache.type=%q", cfg.Server.SSL.MinVersion, cfg.Web.UI.Theme, cfg.Server.Cache.Type)
	}
	if cfg.Server.Compression.Level != 3 || strings.Join(cfg.Server.Compression.Types, ",") != "text/plain,application/*" {
		t.Errorf("valid compression settings changed: %+v", cfg.Server.Compression)
	}
	if cfg.Server.Scheduler.Tasks.TokenCleanup.Schedule != "@every 15m" {
		t.Errorf("TokenCleanup.Schedule = %q, want valid descriptor unchanged", cfg.Server.Scheduler.Tasks.TokenCleanup.Schedule)
	}
	if cfg.Server.Limits.MaxBodyBytes() != 2*1024*1024 || cfg.Server.Limits.ReadTimeoutDuration() != 4*time.Second || cfg.Server.Limits.WriteTimeoutDuration() != 5*time.Second || cfg.Server.Limits.IdleTimeoutDuration() != 90*time.Second {
		t.Errorf("valid server limits parsed incorrectly: %+v", cfg.Server.Limits)
	}
}

func TestValidateConfigLeavesValidValuesUntouched(t *testing.T) {
	cfg := DefaultApplicationConfig()
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
	cfg := DefaultApplicationConfig()
	cfg.Server.Notifications.Email.SMTP.Password = "s3cret"
	cfg.Server.Pages.Contact.CaptchaSecretKey = "captcha-secret"
	cfg.Server.Token = "operator-token"
	cfg.Server.Metrics.Auth.Tokens = MetricsTokensConfig{Prometheus: "prom-token", Grafana: "graf-token", Loki: "loki-token"}
	mu.Lock()
	current = cfg
	mu.Unlock()

	sanitized := GetSanitizedApplicationConfig()

	if sanitized.Server.Notifications.Email.SMTP.Password != redactedValue {
		t.Errorf("Sanitized SMTP.Password = %q, want %q", sanitized.Server.Notifications.Email.SMTP.Password, redactedValue)
	}
	if sanitized.Server.Pages.Contact.CaptchaSecretKey != redactedValue {
		t.Errorf("Sanitized contact CAPTCHA secret = %q, want redacted", sanitized.Server.Pages.Contact.CaptchaSecretKey)
	}
	if sanitized.Server.Token != redactedValue ||
		sanitized.Server.Metrics.Auth.Tokens.Prometheus != redactedValue ||
		sanitized.Server.Metrics.Auth.Tokens.Grafana != redactedValue ||
		sanitized.Server.Metrics.Auth.Tokens.Loki != redactedValue {
		t.Errorf("GetSanitizedApplicationConfig() leaked an operator or metrics token: %+v", sanitized.Server.Metrics.Auth.Tokens)
	}
	if cfg.Server.Notifications.Email.SMTP.Password != "s3cret" || cfg.Server.Pages.Contact.CaptchaSecretKey != "captcha-secret" {
		t.Error("GetSanitizedApplicationConfig() must not mutate the original config's password")
	}
	if cfg.Server.Token != "operator-token" || cfg.Server.Metrics.Auth.Tokens.Prometheus != "prom-token" {
		t.Error("GetSanitizedApplicationConfig() must not mutate the original config's tokens")
	}
}

func TestValidateConfigFallsBackWhenExternalCaptchaKeysAreMissing(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Server.Pages.Contact.Captcha = "recaptcha"
	validateConfig(cfg)
	if cfg.Server.Pages.Contact.Captcha != DefaultApplicationConfig().Server.Pages.Contact.Captcha {
		t.Fatalf("validateConfig() captcha = %q, want default", cfg.Server.Pages.Contact.Captcha)
	}
	cfg.Server.Pages.Contact.Captcha = "recaptcha"
	cfg.Server.Pages.Contact.CaptchaSiteKey = "site-key"
	cfg.Server.Pages.Contact.CaptchaSecretKey = "secret-key"
	validateConfig(cfg)
	if cfg.Server.Pages.Contact.Captcha != "recaptcha" {
		t.Fatalf("validateConfig() captcha = %q, want recaptcha", cfg.Server.Pages.Contact.Captcha)
	}
}

func TestValidateContactAntispamKeepsZeroDisableAndResetsInvalidValues(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Server.Pages.Contact.Antispam.PerIP = 0
	cfg.Server.Pages.Contact.Antispam.Strikes = 0
	cfg.Server.Pages.Contact.Antispam.MaxLinks = -1
	cfg.Server.Pages.Contact.Antispam.Window = "not-a-duration"
	validateConfig(cfg)
	if cfg.Server.Pages.Contact.Antispam.PerIP != 0 || cfg.Server.Pages.Contact.Antispam.Strikes != 0 {
		t.Fatalf("zero contact antispam settings should disable their rules: %+v", cfg.Server.Pages.Contact.Antispam)
	}
	if cfg.Server.Pages.Contact.Antispam.MaxLinks != DefaultApplicationConfig().Server.Pages.Contact.Antispam.MaxLinks || cfg.Server.Pages.Contact.Antispam.Window != "1h" {
		t.Fatalf("invalid contact antispam values were not reset: %+v", cfg.Server.Pages.Contact.Antispam)
	}
}

func TestValidateConfigClearsInvalidAccent(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Web.UI.Accent = "url(javascript:alert(1))"
	validateConfig(cfg)
	if cfg.Web.UI.Accent != "" {
		t.Fatalf("invalid accent = %q, want empty built-in default", cfg.Web.UI.Accent)
	}
}

func TestGenerateConfigYAMLContainsOnlyNodeSettings(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.CLI.BinaryDownload.RequireAuth = true
	cfg.Server.Update.CLIMinVersion = "1.2.3"
	cfg.Server.Metrics.Root.Enabled = false
	cfg.Server.Metrics.Auth.AllowUnauthenticated = true
	cfg.Server.Metrics.Auth.Tokens = MetricsTokensConfig{Prometheus: "prom-token", Grafana: "graf-token", Loki: "loki-token"}
	cfg.Server.Metrics.IncludeRuntime = false
	cfg.Server.Metrics.Loki = MetricsLokiConfig{MaxEntries: 47, MaxAge: "15m"}

	yaml := generateConfigYAML(cfg)
	if strings.Contains(yaml, "compliance:\n    enabled:") {
		t.Error("generated server.yml contains a separate compliance-mode switch")
	}
	for _, expected := range []string{"baseurl: \"/\"", "database:", "cache:", "ssl:", "trusted_proxies:"} {
		if !strings.Contains(yaml, expected) {
			t.Errorf("generated server.yml missing node setting %q", expected)
		}
	}
	for _, key := range []string{"metrics:", "scheduler:", "i18n:", "session:", "cli:", "output:", "web:"} {
		if strings.Contains(yaml, key) {
			t.Errorf("generated server.yml contains non-node setting %q", key)
		}
	}
	if strings.Contains(yaml, "prom-token") || strings.Contains(yaml, "graf-token") || strings.Contains(yaml, "loki-token") {
		t.Error("generated server.yml contains metrics secrets")
	}
}

func TestComplianceModeRequiresAtLeastOneStandard(t *testing.T) {
	if (ComplianceConfig{}).IsEnabled() {
		t.Fatal("compliance mode is enabled with no standards")
	}
	standards := []ComplianceConfig{
		{GDPR: true}, {CCPA: true}, {HIPAA: true}, {SOC2: true}, {PCIDSS: true},
		{ISO27001: true}, {FedRAMP: true}, {LGPD: true}, {PIPEDA: true}, {APPI: true}, {PDPA: true},
	}
	for i, standard := range standards {
		if !standard.IsEnabled() {
			t.Errorf("compliance standard %d did not enable compliance mode", i)
		}
	}
}

func TestSessionConfigDefaultsAndValidation(t *testing.T) {
	defaults := DefaultApplicationConfig().Server.Session
	if defaults.CookieName != "session" || defaults.MaxAge != "0s" || defaults.Secure != "auto" || !defaults.HTTPOnly || defaults.SameSite != "lax" {
		t.Fatalf("session defaults = %+v, want secure browser-session defaults", defaults)
	}
	cfg := DefaultApplicationConfig()
	cfg.Server.Session.MaxAge = "invalid"
	cfg.Server.Session.Secure = "sometimes"
	cfg.Server.Session.SameSite = "cross-site"
	validateConfig(cfg)
	if cfg.Server.Session.MaxAge != defaults.MaxAge || cfg.Server.Session.Secure != defaults.Secure || cfg.Server.Session.SameSite != defaults.SameSite {
		t.Fatalf("invalid session settings were not reset to defaults: %+v", cfg.Server.Session)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
		valid bool
	}{
		{input: "", want: "/", valid: true},
		{input: "/", want: "/", valid: true},
		{input: "/myproject/", want: "/myproject", valid: true},
		{input: "/my//project", want: "/my/project", valid: true},
		{input: "myproject", valid: false},
		{input: "//example.com/path", valid: false},
		{input: "/myproject?x=1", valid: false},
		{input: "/my/../project", valid: false},
		{input: "/my/%2e%2e/project", valid: false},
	}
	for _, test := range tests {
		got, valid := NormalizeBaseURL(test.input)
		if got != test.want || valid != test.valid {
			t.Errorf("NormalizeBaseURL(%q) = (%q, %t), want (%q, %t)", test.input, got, valid, test.want, test.valid)
		}
	}
}

func TestValidateConfigReplacesInvalidBaseURL(t *testing.T) {
	cfg := DefaultApplicationConfig()
	cfg.Server.BaseURL = "https://example.com/path"
	validateConfig(cfg)
	if cfg.Server.BaseURL != "/" {
		t.Errorf("BaseURL = %q, want default /", cfg.Server.BaseURL)
	}
}

func TestGenerateConfigYAMLDoesNotExposeTorEnableToggle(t *testing.T) {
	generated := generateConfigYAML(DefaultApplicationConfig())
	var document map[string]any
	if err := yaml.Unmarshal([]byte(generated), &document); err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}
	serverConfig, ok := document["server"].(map[string]any)
	if !ok {
		t.Fatal("generated YAML is missing the server configuration section")
	}
	if _, ok := serverConfig["tor"]; ok {
		t.Error("generated server.yml contains the Tor area section")
	}
}

func TestOutputConfigLoadsAndPersists(t *testing.T) {
	var cfg ApplicationConfig
	if err := yaml.Unmarshal([]byte("output:\n  color: false\n  emoji: true\n"), &cfg); err != nil {
		t.Fatalf("unmarshal output settings: %v", err)
	}
	if cfg.Output.Color == nil || *cfg.Output.Color {
		t.Fatalf("output.color = %v, want explicit false", cfg.Output.Color)
	}
	if cfg.Output.Emoji == nil || !*cfg.Output.Emoji {
		t.Fatalf("output.emoji = %v, want explicit true", cfg.Output.Emoji)
	}
	generated := generateConfigYAML(&cfg)
	if strings.Contains(generated, "output:") || strings.Contains(generated, "color: false") || strings.Contains(generated, "emoji: true") {
		t.Fatal("generated server.yml contains non-node output settings")
	}
}
