package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	WebUI       WebUIConfig       `yaml:"web-ui"`
	WebRobots   WebRobotsConfig   `yaml:"web-robots"`
	WebSecurity WebSecurityConfig `yaml:"web-security"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	Port         string         `yaml:"port"`          // Single port (HTTP) or dual (8090,64453)
	FQDN         string         `yaml:"fqdn"`          // Fully qualified domain name
	Address      string         `yaml:"address"`       // Listen address
	Mode         string         `yaml:"mode"`          // Application mode: production or development
	UpdateBranch string         `yaml:"update_branch"` // Update branch: stable, beta, or daily
	Schedule     ScheduleConfig `yaml:"schedule"`      // Task scheduling
	SSL          SSLConfig      `yaml:"ssl"`           // SSL/TLS configuration
	GeoIP        GeoIPConfig    `yaml:"geoip"`         // GeoIP settings
	Metrics      MetricsConfig  `yaml:"metrics"`       // Metrics/observability
	Logging      LoggingConfig  `yaml:"logging"`       // Logging settings
}

// ScheduleConfig contains scheduler settings
type ScheduleConfig struct {
	Enabled       bool   `yaml:"enabled"`        // Enable scheduler
	CertRenewal   string `yaml:"cert_renewal"`   // Certificate renewal schedule
	Notifications string `yaml:"notifications"`  // Notification check schedule
	GeoIPUpdate   string `yaml:"geoip_update"`   // GeoIP database update schedule
}

// SSLConfig contains SSL/TLS settings
type SSLConfig struct {
	Enabled     bool              `yaml:"enabled"`      // Enable SSL
	CertPath    string            `yaml:"cert_path"`    // Certificate path
	LetsEncrypt LetsEncryptConfig `yaml:"letsencrypt"`  // Let's Encrypt configuration
}

// LetsEncryptConfig contains Let's Encrypt settings
type LetsEncryptConfig struct {
	Enabled       bool   `yaml:"enabled"`        // Enable Let's Encrypt
	Email         string `yaml:"email"`          // Contact email
	Challenge     string `yaml:"challenge"`      // Challenge type: dns-01, tls-alpn-01, http-01
	DNSProvider   string `yaml:"dns_provider"`   // DNS provider for dns-01
	DNSAPIKey     string `yaml:"dns_api_key"`    // API key for DNS provider
	RFC2136Server string `yaml:"rfc2136_server"` // RFC2136 nameserver
	RFC2136Secret string `yaml:"rfc2136_secret"` // RFC2136 TSIG secret
}

// GeoIPConfig contains GeoIP settings
type GeoIPConfig struct {
	Enabled        bool     `yaml:"enabled"`         // Enable GeoIP
	Database       string   `yaml:"database"`        // Path to GeoIP database
	AllowCountries []string `yaml:"allow_countries"` // Allowed countries (empty = all)
	DenyCountries  []string `yaml:"deny_countries"`  // Denied countries
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`        // Enable metrics
	Endpoint      string `yaml:"endpoint"`       // Metrics endpoint
	IncludeSystem bool   `yaml:"include_system"` // Include system metrics
	IncludeApp    bool   `yaml:"include_app"`    // Include app metrics
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	AccessFormat string `yaml:"access_format"` // Access log format
	Level        string `yaml:"level"`         // Log level
}

// WebUIConfig contains web UI settings
type WebUIConfig struct {
	Theme         string              `yaml:"theme"`         // dark or light
	Logo          string              `yaml:"logo"`          // Logo URL
	Favicon       string              `yaml:"favicon"`       // Favicon path
	Notifications NotificationsConfig `yaml:"notifications"` // Notification settings
}

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	Enabled       bool     `yaml:"enabled"`       // Enable notifications
	Announcements []string `yaml:"announcements"` // Admin announcements
}

// WebRobotsConfig contains robots.txt settings
type WebRobotsConfig struct {
	Allow []string `yaml:"allow"` // Allowed paths
	Deny  []string `yaml:"deny"`  // Denied paths
}

// WebSecurityConfig contains security settings
type WebSecurityConfig struct {
	Admin string `yaml:"admin"` // Security contact email
	CORS  string `yaml:"cors"`  // CORS origin
}

var (
	current *Config
	mu      sync.RWMutex
	configPath string
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "",  // Will be set to random 64xxx port
			FQDN:         "",
			Address:      "0.0.0.0",
			Mode:         "production",
			UpdateBranch: "stable",
			Schedule: ScheduleConfig{
				Enabled:       true,
				CertRenewal:   "daily",
				Notifications: "hourly",
				GeoIPUpdate:   "weekly",
			},
			SSL: SSLConfig{
				Enabled:  false,
				CertPath: "",
				LetsEncrypt: LetsEncryptConfig{
					Enabled:   false,
					Challenge: "http-01",
				},
			},
			GeoIP: GeoIPConfig{
				Enabled:        true,
				AllowCountries: []string{},
				DenyCountries:  []string{},
			},
			Metrics: MetricsConfig{
				Enabled:       false,
				Endpoint:      "/metrics",
				IncludeSystem: true,
				IncludeApp:    true,
			},
			Logging: LoggingConfig{
				AccessFormat: "apache",
				Level:        "info",
			},
		},
		WebUI: WebUIConfig{
			Theme:   "dark",
			Logo:    "",
			Favicon: "",
			Notifications: NotificationsConfig{
				Enabled:       true,
				Announcements: []string{},
			},
		},
		WebRobots: WebRobotsConfig{
			Allow: []string{"/", "/api"},
			Deny:  []string{"/admin", "/debug"},
		},
		WebSecurity: WebSecurityConfig{
			Admin: "",
			CORS:  "*",
		},
	}
}

// Load loads configuration from a YAML file
// Auto-migrates from .yaml to .yml extension per BASE.md
func Load(path string) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	// Migrate .yaml to .yml if needed
	path = migrateYamlToYml(path)
	configPath = path

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultConfig()
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		current = cfg
		return cfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	current = cfg
	return cfg, nil
}

// migrateYamlToYml migrates config from .yaml to .yml extension
func migrateYamlToYml(path string) string {
	// If path ends with .yaml, check if we need to migrate
	if strings.HasSuffix(path, ".yaml") {
		ymlPath := strings.TrimSuffix(path, ".yaml") + ".yml"
		// If .yml doesn't exist but .yaml does, rename
		if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
			if _, err := os.Stat(path); err == nil {
				if err := os.Rename(path, ymlPath); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", path, ymlPath)
				}
			}
		}
		return ymlPath
	}
	// If path ends with .yml, check for .yaml to migrate
	if strings.HasSuffix(path, ".yml") {
		yamlPath := strings.TrimSuffix(path, ".yml") + ".yaml"
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if _, err := os.Stat(yamlPath); err == nil {
				if err := os.Rename(yamlPath, path); err == nil {
					fmt.Printf("Migrated config from %s to %s\n", yamlPath, path)
				}
			}
		}
	}
	return path
}

// Get returns the current configuration
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return DefaultConfig()
	}
	return current
}

// Save saves the specified configuration to the specified path
func Save(path string, cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	current = cfg
	if path != "" {
		configPath = path
	}
	return saveConfig(cfg, path)
}

// SaveCurrent saves the current configuration to file
func SaveCurrent() error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil || configPath == "" {
		return fmt.Errorf("no configuration loaded")
	}
	return saveConfig(current, configPath)
}

// Update updates the configuration and saves it
func Update(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	current = cfg
	if configPath != "" {
		return saveConfig(cfg, configPath)
	}
	return nil
}

// saveConfig writes configuration to a YAML file with comments
func saveConfig(cfg *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate YAML with comments
	content := generateConfigYAML(cfg)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// generateConfigYAML generates YAML content with helpful comments
func generateConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# Airports API Server Configuration
# Documentation: https://airports.apimgr.us/docs

server:
  port: "%s"                    # Single port (HTTP) or dual port (8090,64453 - second is HTTPS)
  fqdn: "%s"                    # Fully qualified domain name
  address: "%s"                 # Listen address (0.0.0.0 for all interfaces)
  mode: "%s"                    # Application mode: production or development
  update_branch: "%s"           # Update branch: stable, beta, or daily

  schedule:                     # Task scheduling
    enabled: %t
    cert_renewal: "%s"          # Certificate renewal check (daily, weekly)
    notifications: "%s"         # Notification checks (hourly, daily)
    geoip_update: "%s"          # GeoIP database update (daily, weekly)

  ssl:                          # SSL/TLS configuration
    enabled: %t
    cert_path: "%s"             # Certificate storage path
    letsencrypt:                # Let's Encrypt configuration
      enabled: %t
      email: "%s"               # Contact email for Let's Encrypt
      challenge: "%s"           # Challenge type: dns-01, tls-alpn-01, http-01
      dns_provider: "%s"        # For dns-01: cloudflare, route53, digitalocean, rfc2136
      dns_api_key: "%s"         # API key for DNS provider
      rfc2136_server: "%s"      # RFC2136 nameserver (if using rfc2136)
      rfc2136_secret: "%s"      # RFC2136 TSIG secret

  geoip:                        # GeoIP settings
    enabled: %t
    database: "%s"              # Path to GeoIP database
    allow_countries: %s         # Allowed countries (empty = allow all)
    deny_countries: %s          # Blocked countries

  metrics:                      # Metrics/observability
    enabled: %t
    endpoint: "%s"              # Prometheus-compatible metrics endpoint
    include_system: %t          # Include CPU, memory, disk metrics
    include_app: %t             # Include request, error, latency metrics

  logging:                      # Logging configuration
    access_format: "%s"         # Access log format (apache, json, combined)
    level: "%s"                 # Log level (debug, info, warn, error)

web-ui:
  theme: "%s"                   # Theme: dark (dracula) or light
  logo: "%s"                    # Logo URL (local file or remote)
  favicon: "%s"                 # Favicon path (local file or remote)
  notifications:                # Announcement system
    enabled: %t
    announcements: %s           # Admin announcements list

web-robots:                     # robots.txt configuration
  allow: %s                     # Allowed paths
  deny: %s                      # Denied paths

web-security:
  admin: "%s"                   # security.txt contact email
  cors: "%s"                    # CORS origin (default: *)
`,
		cfg.Server.Port,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Mode,
		cfg.Server.UpdateBranch,
		cfg.Server.Schedule.Enabled,
		cfg.Server.Schedule.CertRenewal,
		cfg.Server.Schedule.Notifications,
		cfg.Server.Schedule.GeoIPUpdate,
		cfg.Server.SSL.Enabled,
		cfg.Server.SSL.CertPath,
		cfg.Server.SSL.LetsEncrypt.Enabled,
		cfg.Server.SSL.LetsEncrypt.Email,
		cfg.Server.SSL.LetsEncrypt.Challenge,
		cfg.Server.SSL.LetsEncrypt.DNSProvider,
		cfg.Server.SSL.LetsEncrypt.DNSAPIKey,
		cfg.Server.SSL.LetsEncrypt.RFC2136Server,
		cfg.Server.SSL.LetsEncrypt.RFC2136Secret,
		cfg.Server.GeoIP.Enabled,
		cfg.Server.GeoIP.Database,
		formatStringSlice(cfg.Server.GeoIP.AllowCountries),
		formatStringSlice(cfg.Server.GeoIP.DenyCountries),
		cfg.Server.Metrics.Enabled,
		cfg.Server.Metrics.Endpoint,
		cfg.Server.Metrics.IncludeSystem,
		cfg.Server.Metrics.IncludeApp,
		cfg.Server.Logging.AccessFormat,
		cfg.Server.Logging.Level,
		cfg.WebUI.Theme,
		cfg.WebUI.Logo,
		cfg.WebUI.Favicon,
		cfg.WebUI.Notifications.Enabled,
		formatStringSlice(cfg.WebUI.Notifications.Announcements),
		formatStringSlice(cfg.WebRobots.Allow),
		formatStringSlice(cfg.WebRobots.Deny),
		cfg.WebSecurity.Admin,
		cfg.WebSecurity.CORS,
	)
}

// formatStringSlice formats a string slice for YAML output
func formatStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	result := "["
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("\"%s\"", v)
	}
	result += "]"
	return result
}

// Reload reloads configuration from file
func Reload() error {
	if configPath == "" {
		return fmt.Errorf("no config path set")
	}
	_, err := Load(configPath)
	return err
}

// GetPort returns the configured port or empty string
func GetPort() string {
	cfg := Get()
	return cfg.Server.Port
}

// SetPort updates the port in configuration
func SetPort(port string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Port = port
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// GetTheme returns the current theme
func GetTheme() string {
	cfg := Get()
	return cfg.WebUI.Theme
}

// GetCORS returns the CORS setting
func GetCORS() string {
	cfg := Get()
	if cfg.WebSecurity.CORS == "" {
		return "*"
	}
	return cfg.WebSecurity.CORS
}
