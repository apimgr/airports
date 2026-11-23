package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadBrandingDropIn(t *testing.T) {
	configDir := newTestDir(t)
	dropInDir := filepath.Join(configDir, "yaml.d")
	if err := os.Mkdir(dropInDir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dropInDir, "branding.yml")
	if err := os.WriteFile(path, []byte("server:\n  branding:\n    title: Custom airports\n    content:\n      about:\n        format: md\n        body:\n          default: About text\n          fr: Texte à propos\nweb:\n  ui:\n    theme: light\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultApplicationConfig()
	if err := loadBrandingDropIn(cfg, configDir); err != nil {
		t.Fatalf("loadBrandingDropIn() error = %v", err)
	}
	if cfg.Server.Branding.Title != "Custom airports" || cfg.Web.UI.Theme != "light" {
		t.Fatalf("branding drop-in values were not loaded: %+v / %+v", cfg.Server.Branding, cfg.Web.UI)
	}
	content, ok := cfg.Server.Branding.Content["about"]
	if !ok || content.Format != "md" {
		t.Fatalf("about page content was not loaded: %+v", cfg.Server.Branding.Content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("branding drop-in mode = %04o, want 0600", got)
	}
}

func TestLoadBrandingDropInMergesEveryOrderedAreaFile(t *testing.T) {
	configDir := newTestDir(t)
	dropInDir := filepath.Join(configDir, "yaml.d")
	if err := os.Mkdir(dropInDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"10-first.yml":      "server:\n  branding:\n    title: First\n    tagline: First tagline\n  seo:\n    keywords: [one, two]\n",
		"20-second.yaml":    "server:\n  branding:\n    title: Second\n  seo:\n    keywords: [later]\n",
		".hidden.yml":       "server:\n  branding:\n    title: Hidden\n",
		"30-ignore.yml.bak": "server:\n  branding:\n    title: Backup\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dropInDir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dropInDir, "nested.yml"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultApplicationConfig()
	if err := loadBrandingDropIn(cfg, configDir); err != nil {
		t.Fatalf("loadBrandingDropIn() error = %v", err)
	}
	if cfg.Server.Branding.Title != "Second" || cfg.Server.Branding.Tagline != "First tagline" {
		t.Fatalf("ordered map merge = title %q, tagline %q", cfg.Server.Branding.Title, cfg.Server.Branding.Tagline)
	}
	if len(cfg.Server.SEO.Keywords) != 1 || cfg.Server.SEO.Keywords[0] != "later" {
		t.Fatalf("later list = %#v, want complete replacement", cfg.Server.SEO.Keywords)
	}
}

func TestLoadBrandingDropInIgnoresInvalidFileWithoutPartialChanges(t *testing.T) {
	configDir := newTestDir(t)
	dropInDir := filepath.Join(configDir, "yaml.d")
	if err := os.Mkdir(dropInDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dropInDir, "10-good.yml"), []byte("server:\n  branding:\n    title: Good\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dropInDir, "20-invalid.yml"), []byte("server:\n  branding:\n    title: Partial\n    description: [bad\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultApplicationConfig()
	if err := loadBrandingDropIn(cfg, configDir); err != nil {
		t.Fatalf("loadBrandingDropIn() error = %v", err)
	}
	if cfg.Server.Branding.Title != "Good" || cfg.Server.Branding.Description != "" {
		t.Fatalf("invalid drop-in partially changed config: %+v", cfg.Server.Branding)
	}
}

func TestMigrateLegacyBranding(t *testing.T) {
	configDir := newTestDir(t)
	serverPath := filepath.Join(configDir, "server.yml")
	data := []byte("server:\n  title: Legacy title\n  branding:\n    description: Legacy description\n    logo_url: https://example.com/logo.svg\n    primary_color: '#123456'\n    footer_text: 'A & B'\n  seo:\n    keywords: [airports]\n    opengraph:\n      image: https://example.com/share.png\n    twitter:\n      site: '@airports'\n  pages:\n    about:\n      content: About text\nweb:\n  ui:\n    theme: light\n    logo: old-logo.svg\n  footer:\n    custom_html: '<p>Legacy footer</p>'\n")
	if err := os.WriteFile(serverPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	dropInDir := filepath.Join(configDir, "yaml.d")
	if err := os.Mkdir(dropInDir, 0700); err != nil {
		t.Fatal(err)
	}
	existing := []byte("server:\n  branding:\n    title: Drop-in title\n    content:\n      about:\n        format: html\n        body: '<p>Drop-in about</p>'\nweb:\n  ui:\n    theme: dark\n")
	if err := os.WriteFile(filepath.Join(dropInDir, "branding.yml"), existing, 0600); err != nil {
		t.Fatal(err)
	}
	updated, err := migrateLegacyBranding(serverPath, data)
	if err != nil {
		t.Fatalf("migrateLegacyBranding() error = %v", err)
	}
	if strings.Contains(string(updated), "branding:") || strings.Contains(string(updated), "seo:") || strings.Contains(string(updated), "theme:") || strings.Contains(string(updated), "accent:") {
		t.Fatalf("server.yml retained branding keys: %s", updated)
	}
	var migrated map[string]interface{}
	if err := yaml.Unmarshal(updated, &migrated); err != nil {
		t.Fatalf("migrated server.yml is invalid: %v", err)
	}
	var branding map[string]interface{}
	brandingBytes, err := os.ReadFile(filepath.Join(dropInDir, "branding.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(brandingBytes, &branding); err != nil {
		t.Fatalf("migrated branding.yml is invalid: %v", err)
	}
	var cfg ApplicationConfig
	if err := yaml.Unmarshal(brandingBytes, &cfg); err != nil {
		t.Fatalf("branding.yml does not decode to ApplicationConfig: %v", err)
	}
	if cfg.Server.Branding.Title != "Drop-in title" || cfg.Web.UI.Theme != "dark" {
		t.Fatalf("existing drop-in values did not take precedence: %+v / %+v", cfg.Server.Branding, cfg.Web.UI)
	}
	if cfg.Server.Branding.Description != "Legacy description" || cfg.Server.Branding.Logo != "https://example.com/logo.svg" || cfg.Server.SEO.OGImage != "https://example.com/share.png" || cfg.Server.SEO.TwitterHandle != "@airports" || cfg.Web.UI.Accent != "#123456" {
		t.Fatalf("legacy branding aliases were not migrated: %+v / %+v", cfg.Server.Branding, cfg.Server.SEO)
	}
	if len(cfg.Server.SEO.Keywords) != 1 || cfg.Server.SEO.Keywords[0] != "airports" {
		t.Fatalf("SEO keywords were not migrated: %+v", cfg.Server.SEO.Keywords)
	}
	footer, ok := cfg.Server.Branding.Content["footer"]
	if !ok || footer.Format != "html" || footer.Body != "<p>A &amp; B</p>" {
		t.Fatalf("legacy footer text was not safely migrated: %+v", footer)
	}
	if cfg.Server.Branding.Content["about"].Format != "html" {
		t.Fatalf("existing page content did not take precedence: %+v", cfg.Server.Branding.Content["about"])
	}
	if _, err := os.Stat(serverPath + ".bak"); err != nil {
		t.Fatalf("migration backup missing: %v", err)
	}
}

func TestMigrateLegacyAreaSectionsAndSecrets(t *testing.T) {
	configDir := newTestDir(t)
	serverPath := filepath.Join(configDir, "server.yml")
	input := []byte("server:\n  port: 8080\n  token: legacy-token\n  pages:\n    contact:\n      enabled: true\n      captcha_secret_key: captcha-secret\n  notifications:\n    email:\n      smtp:\n        host: mail.example.test\n        password: smtp-secret\nweb:\n  robots:\n    enabled: true\ndata:\n  cve:\n    api_key: cve-secret\n")
	if err := os.WriteFile(serverPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	migrated, err := migrateLegacyBranding(serverPath, input)
	if err != nil {
		t.Fatalf("migrateLegacyBranding() error = %v", err)
	}
	if strings.Contains(string(migrated), "pages:") || strings.Contains(string(migrated), "notifications:") || strings.Contains(string(migrated), "robots:") || strings.Contains(string(migrated), "legacy-token") {
		t.Fatalf("server.yml retained area or secret configuration: %s", migrated)
	}
	for _, item := range []struct {
		file string
		want string
	}{
		{file: "pages.yml", want: "enabled"},
		{file: "notifications.yml", want: "smtp"},
		{file: "web.yml", want: "robots"},
	} {
		contents, err := os.ReadFile(filepath.Join(configDir, "yaml.d", item.file))
		if err != nil {
			t.Fatalf("read %s: %v", item.file, err)
		}
		if !strings.Contains(string(contents), item.want) {
			t.Errorf("%s missing %q: %s", item.file, item.want, contents)
		}
		if strings.Contains(string(contents), "captcha-secret") || strings.Contains(string(contents), "smtp-secret") {
			t.Errorf("%s contains a plaintext secret: %s", item.file, contents)
		}
	}
	secrets, err := os.ReadFile(filepath.Join(configDir, "secrets.yml"))
	if err != nil {
		t.Fatalf("read secrets.yml: %v", err)
	}
	for _, secret := range []string{"legacy-token", "captcha-secret", "smtp-secret", "cve-secret"} {
		if !strings.Contains(string(secrets), secret) {
			t.Errorf("secrets.yml missing %q: %s", secret, secrets)
		}
	}
}

func TestGenerateConfigYAMLOmitsBrandingAreaKeys(t *testing.T) {
	generated := generateConfigYAML(DefaultApplicationConfig())
	var document map[string]interface{}
	if err := yaml.Unmarshal([]byte(generated), &document); err != nil {
		t.Fatalf("generated config is invalid YAML: %v", err)
	}
	server := document["server"].(map[string]interface{})
	if _, ok := server["branding"]; ok {
		t.Fatal("generated server.yml contains server.branding")
	}
	if _, ok := server["seo"]; ok {
		t.Fatal("generated server.yml contains server.seo")
	}
	if _, ok := document["web"]; ok {
		t.Fatal("generated server.yml contains the web area section")
	}
}
