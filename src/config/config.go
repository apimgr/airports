package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/tor"
)

// Config represents the complete server configuration
type Config struct {
	Server ServerConfig `yaml:"server"`
	Web    WebConfig    `yaml:"web"`
	CLI    CLIConfig    `yaml:"cli,omitempty"`
	Output OutputConfig `yaml:"output,omitempty"`
}

// CLIConfig controls the server-side binary download endpoint per AI.md PART 32.
type CLIConfig struct {
	BinaryDownload CLIBinaryDownloadConfig `yaml:"binary_download"`
}

type CLIBinaryDownloadConfig struct {
	RequireAuth bool `yaml:"require_auth"`
}

// OutputConfig controls terminal colors and emoji output. Nil booleans mean
// the setting is unspecified, allowing NO_COLOR and terminal detection.
type OutputConfig struct {
	Color *bool `yaml:"color,omitempty"`
	Emoji *bool `yaml:"emoji,omitempty"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	Port    string `yaml:"port"`    // Single port (HTTP) or dual (8090,64453)
	FQDN    string `yaml:"fqdn"`    // Fully qualified domain name
	Address string `yaml:"address"` // Listen address
	Mode    string `yaml:"mode"`    // Application mode: production or development
	// Debug settings are only activated by the independent debug flag.
	Debug      DebugConfig `yaml:"debug"`
	APIVersion string      `yaml:"api_version"` // API version prefix used in /api/{api_version}/ routes
	// Lang selects the output language or auto-detects the system locale.
	Lang string `yaml:"lang"`
	// BaseURL is the external URL path prefix, such as /myproject.
	BaseURL      string             `yaml:"baseurl"`
	URLDetection URLDetectionConfig `yaml:"url_detection"`
	// I18n contains the default language and supported locale codes.
	I18n           I18nConfig           `yaml:"i18n"`
	Update         UpdateConfig         `yaml:"update"`          // Update channel/auto-install/defer settings (PART 22)
	Healthz        HealthzConfig        `yaml:"healthz"`         // /server/healthz behavior
	Branding       BrandingConfig       `yaml:"branding"`        // Branding text
	SEO            SEOConfig            `yaml:"seo"`             // SEO metadata
	User           string               `yaml:"user"`            // System user (auto-detected if empty)
	Group          string               `yaml:"group"`           // System group (auto-detected if empty)
	PIDFile        bool                 `yaml:"pidfile"`         // Write a PID file
	Daemonize      bool                 `yaml:"daemonize"`       // Detach from terminal on start
	SSL            SSLConfig            `yaml:"ssl"`             // SSL/TLS configuration
	Scheduler      SchedulerConfig      `yaml:"scheduler"`       // Background task scheduling
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`      // Rate limiting
	Database       DatabaseConfig       `yaml:"database"`        // Database connection
	CORS           CORSConfig           `yaml:"cors"`            // CORS settings
	TrustedProxies TrustedProxiesConfig `yaml:"trusted_proxies"` // Additional trusted proxy IPs/CIDRs (private ranges always trusted)
	// Limits configures HTTP request and connection limits (PART 12).
	Limits LimitsConfig `yaml:"limits"`
	// Compression controls response gzip settings (PART 12).
	Compression CompressionConfig `yaml:"compression"`
	Maintenance MaintenanceConfig `yaml:"maintenance"` // Self-healing maintenance mode
	GeoIP       GeoIPConfig       `yaml:"geoip"`       // GeoIP settings
	Metrics     MetricsConfig     `yaml:"metrics"`     // Metrics/observability
	Logging     LoggingConfig     `yaml:"logs"`        // Logging settings (server.logs.*)
	// Privacy contains cookie-consent and CCPA settings.
	Privacy PrivacyConfig `yaml:"privacy"`
	// Contact is the unified notification recipient tree defined in PART 12.
	Contact    ContactConfig     `yaml:"contact"`
	Pages      ServerPagesConfig `yaml:"pages"`
	Tracking   TrackingConfig    `yaml:"tracking"`
	Cache      CacheConfig       `yaml:"cache"`      // Cache backend (memory/valkey/redis)
	Security   SecurityConfig    `yaml:"security"`   // IP allowlist/blocklist management
	CVE        CVEConfig         `yaml:"cve"`        // NVD CVE feed settings
	Backup     BackupConfig      `yaml:"backup"`     // Backup encryption/retention settings
	Compliance ComplianceConfig  `yaml:"compliance"` // Compliance-standard enforcement
	Tor        tor.Config        `yaml:"tor"`        // Tor hidden service settings (PART 31)
	// I2P configures the optional eepsite.
	I2P           i2p.Config                `yaml:"i2p"`
	Notifications ServerNotificationsConfig `yaml:"notifications"` // WebUI toast + operator email settings (PART 17)
	CSRF          CSRFConfig                `yaml:"csrf"`          // Double-submit CSRF protection for browser forms (PART 16)
	Session       SessionConfig             `yaml:"session"`
	// Token is the global operator token (AI.md PART 8 "server.token"),
	// auto-generated on first run. It authorizes destructive operator-only
	// actions such as "--maintenance restore" when the process is neither
	// root nor facing an empty database (AI.md PART 21 "Restore
	// Authorization"). Never logged and never returned by any API surface.
	Token string `yaml:"token"`
}

// DebugConfig controls diagnostics available while the independent debug flag
// is enabled. Application mode alone never enables these features.
type DebugConfig struct {
	Pprof                bool   `yaml:"pprof"`
	LogQueries           bool   `yaml:"log_queries"`
	LogCache             bool   `yaml:"log_cache"`
	LogBodies            bool   `yaml:"log_bodies"`
	MaxBodyLogSize       string `yaml:"max_body_log_size"`
	BlockProfileRate     int    `yaml:"block_profile_rate"`
	MutexProfileFraction int    `yaml:"mutex_profile_fraction"`
	RuntimeEndpoints     bool   `yaml:"runtime_endpoints"`
}

// URLDetectionConfig controls trusted-proxy host learning and live URL refresh.
type URLDetectionConfig struct {
	Learning     bool   `yaml:"learning"`
	MinSamples   int    `yaml:"min_samples"`
	SampleWindow string `yaml:"sample_window"`
	LogChanges   bool   `yaml:"log_changes"`
	LiveReload   bool   `yaml:"live_reload"`
}

// LimitsConfig sets HTTP request-body and connection timeout limits.
type LimitsConfig struct {
	MaxBodySize  string `yaml:"max_body_size"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout"`
}

// CompressionConfig controls gzip response compression.
type CompressionConfig struct {
	Enabled bool     `yaml:"enabled"`
	Level   int      `yaml:"level"`
	Types   []string `yaml:"types"`
}

var defaultCompressionTypes = []string{
	"text/html",
	"text/css",
	"text/javascript",
	"application/json",
	"application/xml",
}

// MaxBodyBytes parses max_body_size using byte units such as KB, MB, and GB.
func (c LimitsConfig) MaxBodyBytes() int64 {
	bytes, err := parseConfigByteSize(c.MaxBodySize)
	if err != nil {
		return 10 * 1024 * 1024
	}
	return bytes
}

func parseConfigByteSize(raw string) (int64, error) {
	value := strings.TrimSpace(strings.ToUpper(raw))
	unit := int64(1)
	for _, suffix := range []struct {
		name       string
		multiplier int64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(value, suffix.name) {
			unit = suffix.multiplier
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix.name))
			break
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 || number > int64(^uint64(0)>>1)/unit {
		return 0, fmt.Errorf("invalid byte size %q", raw)
	}
	return number * unit, nil
}

func (c LimitsConfig) ReadTimeoutDuration() time.Duration {
	return parseLimitDuration(c.ReadTimeout, 30*time.Second)
}
func (c LimitsConfig) WriteTimeoutDuration() time.Duration {
	return parseLimitDuration(c.WriteTimeout, 30*time.Second)
}
func (c LimitsConfig) IdleTimeoutDuration() time.Duration {
	return parseLimitDuration(c.IdleTimeout, 120*time.Second)
}

func parseLimitDuration(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

// CSRFConfig controls the stateless double-submit CSRF protection applied to
// mutating browser form POSTs per AI.md PART 16 "CSRF Protection". API/CLI
// requests carrying a Bearer/API-Token Authorization header are never
// subject to this check (they can't be forged by a third-party page the way
// a cookie-backed browser session can).
type CSRFConfig struct {
	Enabled     bool     `yaml:"enabled"`      // Validate CSRF token on mutating (POST/PUT/PATCH/DELETE) browser requests
	TokenLength int      `yaml:"token_length"` // Random token size in bytes before hex-encoding
	CookieName  string   `yaml:"cookie_name"`  // Double-submit cookie name
	HeaderName  string   `yaml:"header_name"`  // Header name clients may submit the token in
	Secure      string   `yaml:"secure"`       // auto, true, or false — cookie Secure attribute
	ExemptPaths []string `yaml:"exempt_paths"` // Glob patterns bypassing CSRF validation (e.g. JSON API endpoints)
}

// SessionConfig defines defaults for cookies issued by browser-session
// features. A zero max_age creates a browser-session cookie.
type SessionConfig struct {
	CookieName string `yaml:"cookie_name"`
	MaxAge     string `yaml:"max_age"`
	Secure     string `yaml:"secure"`
	HTTPOnly   bool   `yaml:"http_only"`
	SameSite   string `yaml:"same_site"`
}

// ServerNotificationsConfig contains WebUI toast (visitor-facing, client
// side only) and operator email settings per AI.md PART 17 "EMAIL &
// NOTIFICATIONS". This is distinct from Web.UI.Notifications, which
// controls the server-rendered site announcement banner (yaml path
// web.ui.notifications) - the two never collide.
type ServerNotificationsConfig struct {
	WebUI NotifyWebUIConfig `yaml:"webui"` // Toast position/duration
	Email NotifyEmailConfig `yaml:"email"` // Operator email notifications
}

// NotifyWebUIConfig controls the public WebUI toast notification corner and
// auto-dismiss duration per AI.md PART 17 "Sane Defaults".
type NotifyWebUIConfig struct {
	Position string `yaml:"position"` // top-right, top-left, bottom-right, bottom-left
	Duration int    `yaml:"duration"` // Seconds (0 = manual dismiss)
}

// NotifyEmailConfig controls operator email delivery per AI.md PART 17
// "SMTP Configuration". Enabled is auto-managed by the server based on
// SMTP availability (autodetect success or startup connection test) - it
// is not a manual operator toggle, but the field is still yaml-persisted
// so the hand-rolled generateConfigYAML round-trips it across restarts.
type NotifyEmailConfig struct {
	Enabled bool             `yaml:"enabled"`
	SMTP    NotifySMTPConfig `yaml:"smtp"`
	From    NotifyFromConfig `yaml:"from"`
	ReplyTo string           `yaml:"reply_to"` // Reply-To address; omitted from templates if empty
	// Events maps template name to whether an email is sent for that event,
	// per AI.md PART 17 "Configuration". Absent key defaults to true.
	Events map[string]bool `yaml:"events"`
}

// NotifySMTPConfig holds SMTP connection settings, overridable via SMTP_*
// env vars per AI.md PART 17 "Environment Variable Priority".
type NotifySMTPConfig struct {
	Host     string `yaml:"host"`     // Empty = autodetect on startup
	Port     int    `yaml:"port"`     // Default 587
	Username string `yaml:"username"` // Optional SMTP auth username
	Password string `yaml:"password"` // Optional SMTP auth password, never exposed via Sanitized()
	TLS      string `yaml:"tls"`      // auto, starttls, tls, none
}

// NotifyFromConfig holds the notification sender identity.
type NotifyFromConfig struct {
	Name  string `yaml:"name"`  // Default: app title
	Email string `yaml:"email"` // Default: no-reply@{fqdn}
}

// notificationEventOrder is the fixed key order used when rendering
// server.notifications.email.events to YAML, since Go map iteration order
// is non-deterministic. Matches AI.md PART 17 "Configuration".
var notificationEventOrder = []string{
	"startup",
	"shutdown",
	"backup_complete",
	"backup_failed",
	"ssl_expiring",
	"ssl_renewed",
	"ssl_renewal_failed",
	"security_alert",
	"scheduler_error",
	"update_available",
	"update_installed",
}

// SecurityConfig contains IP Block Management settings per AI.md PART 11
// "IP Block Management". Allowlist entries always bypass blocklist/rate
// limit/GeoIP/auto-block (never CSRF/path-security/TLS); BlockedIPs are
// permanent, config-file-only blocks released only by editing the config.
type SecurityConfig struct {
	Allowlist      []AllowlistEntry     `yaml:"allowlist"`       // Trusted IPs/CIDRs, bypass blocklist/rate-limit/GeoIP/auto-block
	BlockedIPs     []BlockedIPEntry     `yaml:"blocked_ips"`     // Permanent config-file blocks
	EncryptionKey  string               `yaml:"encryption_key"`  // AES-256-GCM key for at-rest sensitive data (DNS-01 provider credentials, backups); never exposed via Sanitized()
	AbuseDetection AbuseDetectionConfig `yaml:"abuse_detection"` // Request-flood auto-block + auto-alert (PART 11 "Abuse Detection")
}

// AbuseDetectionConfig controls the built-in abuse-detection auto-block/
// auto-alert behavior per AI.md PART 11 "Abuse Detection". All-enabled by
// default; auto-blocking always respects server.security.allowlist (never
// auto-blocks a trusted IP).
type AbuseDetectionConfig struct {
	RequestFlood RequestFloodConfig `yaml:"request_flood"` // Request-flood detection thresholds
	AutoBlockIP  bool               `yaml:"auto_block_ip"` // Automatically add a temporary block for offending IPs
	AutoAlert    bool               `yaml:"auto_alert"`    // Automatically send a security_alert notification
}

// RequestFloodConfig controls the "10x rate limit in short burst from same
// IP" detection trigger (Medium severity, Block IP + alert via email) per
// AI.md PART 11 "Abuse Detection" table.
type RequestFloodConfig struct {
	Multiplier    int    `yaml:"multiplier"`     // Rate-limit rejections (within the window) that trigger auto-block
	BlockDuration string `yaml:"block_duration"` // Auto-block duration (Go duration string)
}

// CVEConfig contains NVD CVE 2.0 API feed settings for the scheduled
// cve_update task per AI.md PART 18/19.
type CVEConfig struct {
	APIKey string `yaml:"api_key"` // Optional NVD API key (raises rate limit from 5/30s to 50/30s)
}

// BackupConfig contains backup encryption and retention settings per AI.md
// PART 21 "Backup Encryption" / "Backup Retention".
type BackupConfig struct {
	// EncryptionPassword, when non-empty, AES-256-GCM-encrypts backups (key
	// derived via Argon2id). Required when server.compliance.enabled is
	// true. Never exposed via Sanitized() or logs.
	EncryptionPassword string          `yaml:"encryption_password"`
	Retention          BackupRetention `yaml:"retention"` // Retention limits
}

// BackupRetention controls how many backups of each cadence are kept, plus
// an optional hard size cap, per AI.md PART 21 "Backup Retention".
type BackupRetention struct {
	MaxBackups   int    `yaml:"max_backups"`    // 1-365: daily full backups to keep
	KeepWeekly   int    `yaml:"keep_weekly"`    // 0-52: Sunday backups (0 = disabled)
	KeepMonthly  int    `yaml:"keep_monthly"`   // 0-12: 1st-of-month backups (0 = disabled)
	KeepYearly   int    `yaml:"keep_yearly"`    // 0-10: January 1st backups (0 = disabled)
	MaxTotalSize string `yaml:"max_total_size"` // Percent ("10%") or absolute ("50G") hard cap; "0" = disabled
}

// ComplianceConfig controls compliance-standard enforcement per AI.md
// PART 11 "Compliance" / PART 21 "Compliance Mode Enforcement". When
// Any enabled standard activates compliance mode; backups require
// server.backup.encryption_password.
type ComplianceConfig struct {
	Enabled  bool `yaml:"enabled"`
	GDPR     bool `yaml:"gdpr"`
	CCPA     bool `yaml:"ccpa"`
	HIPAA    bool `yaml:"hipaa"`
	SOC2     bool `yaml:"soc2"`
	PCIDSS   bool `yaml:"pci_dss"`
	ISO27001 bool `yaml:"iso27001"`
	FedRAMP  bool `yaml:"fedramp"`
	LGPD     bool `yaml:"lgpd"`
	PIPEDA   bool `yaml:"pipeda"`
	APPI     bool `yaml:"appi"`
	PDPA     bool `yaml:"pdpa"`
}

func (c ComplianceConfig) IsEnabled() bool {
	return c.Enabled || c.GDPR || c.CCPA || c.HIPAA || c.SOC2 || c.PCIDSS || c.ISO27001 || c.FedRAMP || c.LGPD || c.PIPEDA || c.APPI || c.PDPA
}

// CacheConfig contains cache backend settings per AI.md PART 12 "Cache
// Configuration" — memory is the default; valkey/redis are opt-in for
// production. When URL is set it takes precedence over the discrete
// host/port/username/password/db fields.
type CacheConfig struct {
	Type          string `yaml:"type"`            // none, memory (default), valkey, redis
	URL           string `yaml:"url"`             // Connection URL, takes precedence over host/port fields
	Host          string `yaml:"host"`            // Cache host (used when URL is empty)
	Port          int    `yaml:"port"`            // Cache port (used when URL is empty)
	Username      string `yaml:"username"`        // Cache auth username (ACL, Redis 6+)
	Password      string `yaml:"password"`        // Cache auth password
	DB            int    `yaml:"db"`              // Cache logical DB index
	TLS           bool   `yaml:"tls"`             // Enable TLS for the connection
	TLSSkipVerify bool   `yaml:"tls_skip_verify"` // Skip TLS certificate verification
	PoolSize      int    `yaml:"pool_size"`       // Connection pool size
	MinIdle       int    `yaml:"min_idle"`        // Minimum idle connections
	Timeout       string `yaml:"timeout"`         // Connection/operation timeout (Go duration string)
	Prefix        string `yaml:"prefix"`          // Key prefix, defaults to "{project_name}:"
	TTL           string `yaml:"ttl"`             // Default entry TTL (Go duration string)
}

// HealthzConfig contains /server/healthz behavior settings
type HealthzConfig struct {
	Root HealthzRootConfig `yaml:"root"` // Optional /healthz root-alias behavior
}

// HealthzRootConfig controls the optional /healthz compatibility alias
type HealthzRootConfig struct {
	Enabled bool `yaml:"enabled"` // Mount /healthz to the same handler as /server/healthz
}

// I18nConfig controls the default and supported interface languages.
type I18nConfig struct {
	Enabled          bool     `yaml:"enabled"`
	DefaultLanguage  string   `yaml:"default_language"`
	Supported        []string `yaml:"available_languages"`
	FallbackLanguage string   `yaml:"fallback_language"`
	CookieName       string   `yaml:"cookie_name"`
	CookieMaxAge     string   `yaml:"cookie_max_age"`
}

// BrandingConfig contains project branding text
type BrandingConfig struct {
	Title       string `yaml:"title"`       // Displayed project title
	Tagline     string `yaml:"tagline"`     // Short tagline
	Description string `yaml:"description"` // Longer description
}

// SEOConfig contains SEO metadata
type SEOConfig struct {
	Keywords []string `yaml:"keywords"` // SEO keywords
}

// SSLConfig contains SSL/TLS settings
type SSLConfig struct {
	Enabled     bool              `yaml:"enabled"`     // Enable SSL
	Cert        string            `yaml:"cert"`        // Manual certificate override path
	Key         string            `yaml:"key"`         // Manual key override path
	MinVersion  string            `yaml:"min_version"` // TLS1.2 or TLS1.3
	LetsEncrypt LetsEncryptConfig `yaml:"letsencrypt"` // Let's Encrypt configuration
	// LastExpiryWarningDays is the smallest expiry-countdown threshold
	// (30/14/7/3/1) already warned about for the current certificate, so
	// the daily ssl_renewal task's expiry check fires each threshold's
	// log/email once rather than every day inside the window (PART 17
	// "Sent 7, 3, 1 days before expiry"). Reset to 0 once a renewal
	// succeeds and a fresh expiry date is in effect.
	LastExpiryWarningDays int `yaml:"last_expiry_warning_days,omitempty"`
}

// LetsEncryptConfig contains Let's Encrypt settings. DNS-01 provider
// selection is fully dynamic per AI.md PART 15 "DNS-01 Provider
// Configuration" / api-rules.md "NEVER let a project's DNS-01 provider
// list be limited" — DNSProvider accepts ANY lego-supported provider name
// (https://go-acme.github.io/lego/dns/), and DNSCredentials holds that
// provider's arbitrary field set (lego env var name -> value), never a
// fixed struct of named fields.
type LetsEncryptConfig struct {
	Enabled   bool   `yaml:"enabled"`   // Enable Let's Encrypt
	Email     string `yaml:"email"`     // Contact email
	Challenge string `yaml:"challenge"` // Challenge type: dns-01, tls-alpn-01, http-01
	Staging   bool   `yaml:"staging"`   // Use Let's Encrypt staging server for testing
	// DNSProvider is any lego-supported provider name (e.g. "cloudflare",
	// "route53", "digitalocean", "rfc2136", ...) — never a fixed enum.
	DNSProvider string `yaml:"dns_provider"`
	// DNSCredentials is an AES-256-GCM encrypted (base64) JSON blob of the
	// selected provider's credential fields, keyed by server.security.
	// encryption_key. Decrypt with ssl.DecryptDNSCredentials before use;
	// never stored or logged in plaintext. See PART 15 "Provider Credential
	// Storage".
	DNSCredentials string `yaml:"dns_credentials"`
	// DNSCredentialsValidatedAt is an RFC 3339 timestamp set after the last
	// successful DNS-01 provider credential validation, per PART 15
	// "validated_at".
	DNSCredentialsValidatedAt string `yaml:"dns_credentials_validated_at"`
}

// SchedulerConfig contains built-in scheduler settings — never external cron
type SchedulerConfig struct {
	Enabled       bool           `yaml:"enabled"`         // Enable the scheduler
	Timezone      string         `yaml:"timezone"`        // IANA timezone for schedule evaluation
	CatchUpWindow string         `yaml:"catch_up_window"` // Missed-task catch-up window (Go duration string)
	Tasks         SchedulerTasks `yaml:"tasks"`           // Built-in scheduled tasks
}

// SchedulerTasks contains every built-in scheduled task's configuration,
// per AI.md PART 18 "Built-in Tasks (Required)".
type SchedulerTasks struct {
	SSLRenewal      SSLRenewalConfig `yaml:"ssl_renewal"`
	GeoIPUpdate     TaskConfig       `yaml:"geoip_update"`
	BlocklistUpdate TaskConfig       `yaml:"blocklist_update"`
	CVEUpdate       TaskConfig       `yaml:"cve_update"`
	UpdateCheck     TaskConfig       `yaml:"update_check"`
	TokenCleanup    TaskConfig       `yaml:"token_cleanup"`
	LogRotation     TaskConfig       `yaml:"log_rotation"`
	Backup          BackupTaskConfig `yaml:"backup"`
	BackupHourly    TaskConfig       `yaml:"backup_hourly"`
	HealthCheck     TaskConfig       `yaml:"health_check"`
	TorHealth       TaskConfig       `yaml:"tor_health"`
}

// TaskConfig is the common shape shared by most scheduled tasks
type TaskConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Schedule    string `yaml:"schedule"`
	RetryOnFail bool   `yaml:"retry_on_fail"`
	RetryDelay  string `yaml:"retry_delay"`
}

// BackupTaskConfig is the scheduled backup task's configuration
type BackupTaskConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Schedule  string `yaml:"schedule"`
	Retention int    `yaml:"retention"`
}

// SSLRenewalConfig is the scheduled SSL renewal task's configuration
type SSLRenewalConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Schedule    string `yaml:"schedule"`
	RenewBefore string `yaml:"renew_before"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled     bool            `yaml:"enabled"`
	Read        RateLimitBucket `yaml:"read"`
	Write       RateLimitBucket `yaml:"write"`
	Health      RateLimitBucket `yaml:"health"`
	GlobalBurst int             `yaml:"global_burst"`
}

// RateLimitBucket is a single requests-per-window rate limit rule
type RateLimitBucket struct {
	Requests int `yaml:"requests"` // Requests allowed per window
	Window   int `yaml:"window"`   // Window length in seconds
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	Driver string             `yaml:"driver"` // sqlite or libsql
	URL    string             `yaml:"url"`    // Connection URL (auto-created path for sqlite)
	Pool   DatabasePoolConfig `yaml:"pool"`   // Connection pool tuning (AI.md PART 10 "Connection Pooling")
}

// DatabasePoolConfig contains connection pool tuning settings (AI.md PART 10
// "Pool Configuration"). SQLite is single-writer, so the shipped defaults
// stay small (4/4) regardless of the guideline table's libsql-oriented
// numbers.
type DatabasePoolConfig struct {
	MaxOpen     int    `yaml:"max_open"`      // Max open connections
	MaxIdle     int    `yaml:"max_idle"`      // Max idle connections
	MaxLifetime string `yaml:"max_lifetime"`  // Max connection lifetime (e.g. "5m")
	MaxIdleTime string `yaml:"max_idle_time"` // Max idle time before close (e.g. "1m")
}

// CORSConfig contains CORS settings
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// MaintenanceConfig contains self-healing maintenance mode settings
type MaintenanceConfig struct {
	SelfHealing SelfHealingConfig `yaml:"self_healing"`
	Cleanup     CleanupConfig     `yaml:"cleanup"`
	Notify      MaintenanceNotify `yaml:"notify"`
}

// SelfHealingConfig controls maintenance-mode self-healing retries
type SelfHealingConfig struct {
	Enabled       bool   `yaml:"enabled"`
	RetryInterval string `yaml:"retry_interval"`
	MaxAttempts   int    `yaml:"max_attempts"` // 0 = unlimited
}

// CleanupConfig controls maintenance-mode auto-cleanup thresholds
type CleanupConfig struct {
	DiskThreshold    int `yaml:"disk_threshold"`
	LogRetentionDays int `yaml:"log_retention_days"`
	BackupKeepCount  int `yaml:"backup_keep_count"`
}

// MaintenanceNotify controls maintenance mode enter/exit notifications
type MaintenanceNotify struct {
	OnEnter bool `yaml:"on_enter"`
	OnExit  bool `yaml:"on_exit"`
}

// GeoIPConfig contains GeoIP settings
type GeoIPConfig struct {
	Enabled        bool     `yaml:"enabled"`         // Enable GeoIP
	Database       string   `yaml:"database"`        // Path to GeoIP database
	AllowCountries []string `yaml:"allow_countries"` // Allowed countries (empty = all)
	DenyCountries  []string `yaml:"deny_countries"`  // Denied countries
	// Presets holds named, operator-authored country lists for reuse across
	// allow_countries/deny_countries, per AI.md PART 19 "Country Blocking
	// Presets". Ships empty; a preset is never auto-applied.
	Presets map[string][]string `yaml:"presets"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled        bool              `yaml:"enabled"`        // Enable metrics
	IncludeSystem  bool              `yaml:"include_system"` // Include system metrics
	Auth           MetricsAuthConfig `yaml:"auth"`
	Root           MetricsRootConfig `yaml:"root"`
	IncludeRuntime bool              `yaml:"include_runtime"`
	Loki           MetricsLokiConfig `yaml:"loki"`
	Token          string            `yaml:"token,omitempty"`
	// DurationBuckets overrides the histogram buckets (seconds) used for
	// *_http_request_duration_seconds per AI.md PART 20 "Configuration".
	DurationBuckets []float64 `yaml:"duration_buckets"`
	// SizeBuckets overrides the histogram buckets (bytes) used for
	// *_http_request_size_bytes / *_http_response_size_bytes per AI.md
	// PART 20 "Configuration".
	SizeBuckets []float64 `yaml:"size_buckets"`
}

type MetricsRootConfig struct {
	Enabled bool `yaml:"enabled"`
}

type MetricsLokiConfig struct {
	MaxEntries int    `yaml:"max_entries"`
	MaxAge     string `yaml:"max_age"`
}

// MetricsAuthConfig controls access to each metrics service.
type MetricsAuthConfig struct {
	AllowUnauthenticated bool                `yaml:"allow_unauthenticated"`
	Tokens               MetricsTokensConfig `yaml:"tokens"`
}

// MetricsTokensConfig stores independent bearer tokens for metrics services.
type MetricsTokensConfig struct {
	Prometheus string `yaml:"prometheus"`
	Grafana    string `yaml:"grafana"`
	Loki       string `yaml:"loki"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string          `yaml:"level"`  // Global minimum log level: debug, info, warn, error
	Access LogAccessConfig `yaml:"access"` // access.log
	Server LogFileConfig   `yaml:"server"` // server.log
	Error  LogFileConfig   `yaml:"error"`  // error.log
	// App is app.log (or "{project_name}.log") — general application
	// events (info/warn). Per AI.md's "Log Files" table; the table is
	// the more complete source than the main config block example, which
	// omits it (an AI.md self-inconsistency).
	App LogFileConfig `yaml:"app"`
	// Auth is auth.log — authentication events (token issue/revoke,
	// failures). Same table-vs-block-example inconsistency in AI.md.
	Auth     LogFileConfig  `yaml:"auth"`
	Audit    LogAuditConfig `yaml:"audit"`    // audit.log
	Security LogFileConfig  `yaml:"security"` // security.log
	Debug    LogDebugConfig `yaml:"debug"`    // debug.log
}

// LogFileConfig is the shared shape of every entry under server.logs.* per
// AI.md PART 11 "Logging" § Configuration.
type LogFileConfig struct {
	Filename string `yaml:"filename"` // Log file name relative to the logs directory
	Format   string `yaml:"format"`   // Output format (varies per log type)
	Custom   string `yaml:"custom"`   // Custom format string, used when format == "custom"
	Rotate   string `yaml:"rotate"`   // Rotation policy, e.g. "weekly,50MB"
	Keep     string `yaml:"keep"`     // Retention policy, e.g. "none", "30d", "4", "forever"
}

// LogAccessConfig extends LogFileConfig with the access log's health-check
// suppression toggle.
type LogAccessConfig struct {
	LogFileConfig   `yaml:",inline"`
	LogHealthChecks bool `yaml:"log_health_checks"` // Log successful (2xx) health-check requests
}

// LogAuditConfig extends LogFileConfig with the audit log's enable/compress
// toggles.
type LogAuditConfig struct {
	LogFileConfig `yaml:",inline"`
	Enabled       bool `yaml:"enabled"`  // Whether the audit log is written at all
	Compress      bool `yaml:"compress"` // Gzip-compress rotated backups before retention

	// Events toggles which audit event categories are logged.
	Events LogAuditEventsConfig `yaml:"events"`
	// MaskEmails masks email addresses in audit log output. Defaults to
	// true per AI.md "Sane Defaults".
	MaskEmails bool `yaml:"mask_emails"`
}

// LogAuditEventsConfig toggles which audit event categories are written,
// per AI.md "Audit Log Configuration" § events. All categories default to
// true.
type LogAuditEventsConfig struct {
	Configuration bool `yaml:"configuration"` // config.* events
	Security      bool `yaml:"security"`      // security.* events
	Backup        bool `yaml:"backup"`        // backup.* events
	Server        bool `yaml:"server"`        // server.*/scheduler.* events
}

// LogDebugConfig extends LogFileConfig with the debug log's enable toggle
// (debug.log is the only log type disabled by default).
type LogDebugConfig struct {
	LogFileConfig `yaml:",inline"`
	Enabled       bool `yaml:"enabled"` // Whether the debug log is written at all
}

// WebConfig contains frontend/web-facing settings
type WebConfig struct {
	UI            WebUIConfig         `yaml:"ui"`
	Robots        WebRobotsConfig     `yaml:"robots"`
	Security      WebSecurityConfig   `yaml:"security"`
	Announcements AnnouncementsConfig `yaml:"announcements"` // Site banner announcements (PART 16 "Site Banner")
}

// WebUIConfig contains web UI settings
type WebUIConfig struct {
	Theme         string              `yaml:"theme"`         // dark, light, or auto
	Logo          string              `yaml:"logo"`          // Logo URL
	Favicon       string              `yaml:"favicon"`       // Favicon path
	Notifications NotificationsConfig `yaml:"notifications"` // Notification settings
}

// NotificationsConfig contains notification settings
type NotificationsConfig struct {
	Enabled bool `yaml:"enabled"` // Enable notifications
}

// AnnouncementsConfig contains the site-wide announcement banner settings
// per AI.md PART 16 "Site Banner" / "Announcements".
type AnnouncementsConfig struct {
	Enabled  bool                  `yaml:"enabled"`  // Turn announcements on/off
	Messages []AnnouncementMessage `yaml:"messages"` // List of announcement messages
}

// AnnouncementMessage is a single site banner announcement per AI.md PART 16
// "Announcement Structure".
type AnnouncementMessage struct {
	ID          string `yaml:"id"`          // Stable id; changing it resets everyone's dismissal
	Type        string `yaml:"type"`        // warning, info, error, success
	Title       string `yaml:"title"`       // Short title
	Message     string `yaml:"message"`     // Full message content
	Start       string `yaml:"start"`       // ISO 8601 UTC - when to start showing
	End         string `yaml:"end"`         // ISO 8601 UTC - when to stop showing
	Dismissible bool   `yaml:"dismissible"` // Allow visitors to dismiss
}

// WebRobotsConfig contains robots.txt settings
type WebRobotsConfig struct {
	Allow []string `yaml:"allow"` // Allowed paths
	Deny  []string `yaml:"deny"`  // Denied paths
	// AIBots controls per-AI-crawler access, per AI.md PART 11 "AI Crawler
	// Rules". Default posture is allow; only bots resolving to deny get
	// their own robots.txt stanza.
	AIBots WebRobotsAIBotsConfig `yaml:"ai_bots"`
}

// WebRobotsAIBotsConfig contains per-AI-crawler robots.txt access control.
// Default applies to any bot named in Bots without an explicit value;
// explicit per-bot entries always win over Default.
type WebRobotsAIBotsConfig struct {
	// Default is "allow" or "deny" and applies to any listed bot whose own
	// value is empty. Empty/unrecognized values resolve to "allow".
	Default string `yaml:"default"`
	// Bots maps a crawler User-agent token to "allow" or "deny".
	Bots map[string]string `yaml:"bots"`
}

// WebSecurityConfig contains security.txt / contact settings
type WebSecurityConfig struct {
	Admin string `yaml:"admin"` // security.txt contact email
}

// PrivacyConfig controls cookie-consent and CCPA behavior. Dynamic messaging
// switches on Data.Sold.
type PrivacyConfig struct {
	Data       DataPolicy       `yaml:"data"`
	Retention  RetentionPolicy  `yaml:"retention"`
	Consent    ConsentConfig    `yaml:"consent"`
	Cookies    CookieCategories `yaml:"cookies"`
	ThirdParty ThirdPartyConfig `yaml:"third_party"`
	Content    PrivacyContent   `yaml:"content"`
	// Contact retains legacy privacy.contact values for migration.
	Contact LegacyContactConfig `yaml:"contact,omitempty"`
}

// LegacyContactConfig retains the old privacy.contact shape for loading older
// config files; new configuration uses the canonical server.contact tree.
type LegacyContactConfig struct {
	General string `yaml:"general"`
	Abuse   string `yaml:"abuse"`
}

// DataPolicy controls CCPA "Do Not Sell" messaging.
type DataPolicy struct {
	Sold           bool               `yaml:"sold"`
	StoredOnServer bool               `yaml:"stored_on_server"`
	Sharing        []SharingCondition `yaml:"sharing"`
}

type SharingCondition struct {
	Condition string `yaml:"condition"`
	When      string `yaml:"when"`
	Data      string `yaml:"data"`
}

type RetentionPolicy struct {
	Period            string `yaml:"period"`
	ExportAvailable   bool   `yaml:"export_available"`
	DeletionAvailable bool   `yaml:"deletion_available"`
}

// ConsentConfig configures the cookie consent banner per AI.md PART 16.
type ConsentConfig struct {
	Message               string `yaml:"message"`         // Shown when Data.Sold = false
	MessageIfSold         string `yaml:"message_if_sold"` // Shown when Data.Sold = true
	ShowUntilAcknowledged bool   `yaml:"show_until_acknowledged"`
	DefaultEnabled        bool   `yaml:"default_enabled"`
	Policy                struct {
		Text string `yaml:"text"`
		URL  string `yaml:"url"`
	} `yaml:"policy"`
	Buttons struct {
		Decline string `yaml:"decline"`
		Accept  string `yaml:"accept"`
	} `yaml:"buttons"`
	Position        string `yaml:"position"` // "top" or "bottom"
	ShowPreferences bool   `yaml:"show_preferences"`
	PreferencesText string `yaml:"preferences_text"`
}

type CookieCategories struct {
	Essential   CookieCategory  `yaml:"essential"`
	Preferences CookieCategory  `yaml:"preferences"`
	Analytics   AnalyticsCookie `yaml:"analytics"`
}

type CookieCategory struct {
	Enabled     bool   `yaml:"enabled"`
	Description string `yaml:"description"`
}

type AnalyticsCookie struct {
	CookieCategory           `yaml:",inline"`
	DescriptionSuffixNotSold string `yaml:"description_suffix_not_sold"`
	DescriptionSuffixSold    string `yaml:"description_suffix_sold"`
}

type ThirdPartyConfig struct {
	Services []ThirdPartyService `yaml:"services"`
}

type ThirdPartyService struct {
	Name      string `yaml:"name"`
	Purpose   string `yaml:"purpose"`
	DataSent  string `yaml:"data_sent"`
	PolicyURL string `yaml:"policy_url"`
}

type PrivacyContent struct {
	DataCollection  string `yaml:"data_collection"`
	DataUsage       string `yaml:"data_usage"`
	DataUsageIfSold string `yaml:"data_usage_if_sold"`
	DataSecurity    string `yaml:"data_security"`
}

// TrackingConfig selects a supported analytics provider and its identifiers.
type TrackingConfig struct {
	Type string `yaml:"type"`
	ID   string `yaml:"id"`
	URL  string `yaml:"url"`
}

// GetConsentMessage returns MessageIfSold when Data.Sold is true, otherwise
// Message, per AI.md PART 16 "Dynamic Message Selection".
func (p *PrivacyConfig) GetConsentMessage() string {
	if p.Data.Sold {
		return p.Consent.MessageIfSold
	}
	return p.Consent.Message
}

func (p *PrivacyConfig) GetAnalyticsDescription() string {
	base := p.Cookies.Analytics.Description
	if p.Data.Sold {
		return base + " " + p.Cookies.Analytics.DescriptionSuffixSold
	}
	return base + " " + p.Cookies.Analytics.DescriptionSuffixNotSold
}

func (p *PrivacyConfig) GetDataUsageContent() string {
	if p.Data.Sold {
		return p.Content.DataUsageIfSold
	}
	return p.Content.DataUsage
}

func (p *PrivacyConfig) IsCCPAApplicable() bool {
	return p.Data.Sold
}

// ContactConfig is the unified destination tree for server notifications,
// security reports, abuse reports, and contact-form submissions per PART 12.
type ContactConfig struct {
	Admin    ContactRoleConfig `yaml:"admin"`
	Security ContactRoleConfig `yaml:"security"`
	Abuse    ContactRoleConfig `yaml:"abuse"`
	General  ContactRoleConfig `yaml:"general"`
}

// ContactRoleConfig contains one role's email and named webhook destinations.
type ContactRoleConfig struct {
	Email    string            `yaml:"email"`
	Webhooks map[string]string `yaml:"webhooks"`
}

// ServerPagesConfig contains configurable public-page behavior.
type ServerPagesConfig struct {
	Contact ContactPageConfig `yaml:"contact"`
}

// ContactPageConfig controls the public contact form per PART 16.
type ContactPageConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Captcha        string `yaml:"captcha"`
	SuccessMessage string `yaml:"success_message"`
}

// ResolveEmail returns the role's configured email using PART 12 fallback
// ordering. Admin has no fallback; its empty value is a configuration issue.
func (c ContactConfig) ResolveEmail(role string) string {
	switch role {
	case "admin":
		return strings.TrimSpace(c.Admin.Email)
	case "security":
		return firstContactEmail(c.Security.Email, c.Admin.Email)
	case "abuse":
		return firstContactEmail(c.Abuse.Email, c.General.Email, c.Admin.Email)
	case "general":
		return firstContactEmail(c.General.Email, c.Admin.Email)
	default:
		return ""
	}
}

// ResolveWebhook returns the role's configured transport URL using PART 12
// fallback ordering.
func (c ContactConfig) ResolveWebhook(role, transport string) string {
	var candidates []string
	switch role {
	case "admin":
		candidates = []string{c.Admin.Webhooks[transport]}
	case "security":
		candidates = []string{c.Security.Webhooks[transport], c.Admin.Webhooks[transport]}
	case "abuse":
		candidates = []string{c.Abuse.Webhooks[transport], c.General.Webhooks[transport], c.Admin.Webhooks[transport]}
	case "general":
		candidates = []string{c.General.Webhooks[transport], c.Admin.Webhooks[transport]}
	}
	return firstContactEmail(candidates...)
}

func firstContactEmail(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// TrustedProxiesConfig lists additional IPs/CIDRs/DNS names trusted to set
// X-Forwarded-* headers, on top of the always-trusted private ranges
// (loopback, RFC1918, RFC4193, link-local) per AI.md PART 12 "Trusted Proxies".
type TrustedProxiesConfig struct {
	Additional []string `yaml:"additional"` // Additional IPs/CIDRs/DNS names to trust
}

// UpdateConfig controls the self-update channel, auto-install behavior, and
// defer window used by `--update`/`--maintenance update` and the scheduled
// `update_check` task per AI.md PART 22 "Update Configuration".
type UpdateConfig struct {
	// Release channel: stable, beta, or devel.
	Branch      string `yaml:"branch"`
	AutoInstall bool   `yaml:"auto_install"` // Auto-install updates found by update_check (default off)
	DeferDays   int    `yaml:"defer_days"`   // Days a release must age before update_check adopts it

	// LastNotifiedVersion is internal state (not operator-set) recording the
	// newest version update_check has already notified about, so the
	// WARN log + update_available email fire once per version, not on
	// every scheduler run, per PART 22 "Surfacing rules".
	LastNotifiedVersion string `yaml:"last_notified_version,omitempty"`
	// CLIMinVersion is the oldest client accepted; empty disables this minimum.
	CLIMinVersion string `yaml:"cli_min_version"`
}

var (
	current    *Config
	mu         sync.RWMutex
	configPath string
)

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
		applyInitOnlyEnvOverrides(cfg)
		applySMTPEnvOverrides(cfg)
		applyCacheEnvOverrides(cfg)
		applyI2PEnvOverrides(cfg)
		ensureEncryptionKey(cfg)
		ensureOperatorToken(cfg)
		validateConfig(cfg)
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		applyDatabaseEnvOverrides(cfg)
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
	originalUpdateBranch := cfg.Server.Update.Branch

	applySMTPEnvOverrides(cfg)
	applyCacheEnvOverrides(cfg)
	applyI2PEnvOverrides(cfg)
	validateConfig(cfg)

	needsPersist := ensureEncryptionKey(cfg)
	if cfg.Server.Update.Branch != originalUpdateBranch {
		needsPersist = true
	}
	if cfg.Server.Metrics.Token != "" && cfg.Server.Metrics.Auth.Tokens.Prometheus == "" {
		cfg.Server.Metrics.Auth.Tokens.Prometheus = cfg.Server.Metrics.Token
		cfg.Server.Metrics.Token = ""
		needsPersist = true
	}
	if ensureOperatorToken(cfg) {
		needsPersist = true
	}
	if needsPersist {
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to persist generated server secrets: %w", err)
		}
	}
	applyDatabaseEnvOverrides(cfg)

	current = cfg
	return cfg, nil
}

func applyI2PEnvOverrides(cfg *Config) {
	if value, ok := os.LookupEnv("I2P_ENABLED"); ok {
		cfg.Server.I2P.Enabled = strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
	}
}

// applyInitOnlyEnvOverrides applies environment values used only when the
// first server.yml is created. Existing config always takes precedence.
func applyInitOnlyEnvOverrides(cfg *Config) {
	if value := os.Getenv("APPLICATION_NAME"); value != "" {
		cfg.Server.Branding.Title = value
	}
	if value := os.Getenv("APPLICATION_TAGLINE"); value != "" {
		cfg.Server.Branding.Tagline = value
	}
}

// applyDatabaseEnvOverrides applies runtime database environment variables
// after config persistence so ephemeral values never rewrite server.yml.
func applyDatabaseEnvOverrides(cfg *Config) {
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("DATABASE_DRIVER"))); value != "" {
		switch value {
		case "sqlite2", "sqlite3":
			value = "sqlite"
		case "turso":
			value = "libsql"
		}
		cfg.Server.Database.Driver = value
	}
	if value := os.Getenv("DATABASE_URL"); value != "" {
		cfg.Server.Database.URL = value
	}
}

// ensureEncryptionKey generates and assigns a random AES-256-GCM key
// (server.security.encryption_key) the first time server.yml is created or
// loaded without one already set, per PART 15 "Provider Credential
// Storage" (DNS-01 credentials must be encrypted at rest) and
// backend-rules.md's AES-256-GCM at-rest requirement. Returns true if a key
// was generated, so the caller knows to persist it.
func ensureEncryptionKey(cfg *Config) bool {
	if cfg.Server.Security.EncryptionKey != "" {
		return false
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return false
	}
	cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(key)
	return true
}

// ensureOperatorToken generates and assigns the global operator token
// (server.token) the first time server.yml is created or loaded without one
// already set, per AI.md PART 8 "server.token". The token gates destructive
// operator-only actions (AI.md PART 21 "Restore Authorization"). Returns
// true if a token was generated, so the caller knows to persist it.
func ensureOperatorToken(cfg *Config) bool {
	if cfg.Server.Token != "" {
		return false
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Rejection-free mapping is not used here: 62 does not divide 256, so a
	// plain modulo would bias the alphabet. Draw one byte at a time and
	// discard values in the biased tail instead.
	out := make([]byte, 0, 32)
	buf := make([]byte, 1)
	for len(out) < 32 {
		if _, err := rand.Read(buf); err != nil {
			return false
		}
		if buf[0] >= 248 {
			continue
		}
		out = append(out, alphabet[buf[0]%62])
	}
	cfg.Server.Token = "tok_" + string(out)
	return true
}

// applySMTPEnvOverrides applies SMTP_* environment variable overrides on
// top of whatever was loaded from server.yml, per AI.md PART 17
// "Environment Variable Priority" - useful for containers. Only variables
// that are actually set override the config value.
func applySMTPEnvOverrides(cfg *Config) {
	if v := os.Getenv("SMTP_HOST"); v != "" {
		cfg.Server.Notifications.Email.SMTP.Host = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Notifications.Email.SMTP.Port = port
		}
	}
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		cfg.Server.Notifications.Email.SMTP.Username = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		cfg.Server.Notifications.Email.SMTP.Password = v
	}
	if v := os.Getenv("SMTP_TLS"); v != "" {
		cfg.Server.Notifications.Email.SMTP.TLS = v
	}
	if v := os.Getenv("SMTP_FROM_NAME"); v != "" {
		cfg.Server.Notifications.Email.From.Name = v
	}
	if v := os.Getenv("SMTP_FROM_EMAIL"); v != "" {
		cfg.Server.Notifications.Email.From.Email = v
	}
}

// applyCacheEnvOverrides applies CACHE_URL/CACHE_TYPE environment variable
// overrides on top of whatever was loaded from server.yml, per AI.md PART 12
// "Cache Configuration" (`url: ${CACHE_URL}` container-friendly override) -
// useful for containers such as docker-compose.yml, which sets CACHE_URL but
// otherwise relies on server.yml for cache settings. Only variables that are
// actually set override the config value. Setting CACHE_URL alone (without
// CACHE_TYPE) implies type "valkey" when the current type is "none"/"memory",
// since a URL with no consumer would otherwise remain dead config.
func applyCacheEnvOverrides(cfg *Config) {
	if v := os.Getenv("CACHE_TYPE"); v != "" {
		cfg.Server.Cache.Type = v
	}
	if v := os.Getenv("CACHE_URL"); v != "" {
		cfg.Server.Cache.URL = v
		if cfg.Server.Cache.Type == "" || cfg.Server.Cache.Type == "none" || cfg.Server.Cache.Type == "memory" {
			cfg.Server.Cache.Type = "valkey"
		}
	}
}

// NormalizeBaseURL validates and canonicalizes an external URL path prefix.
// Empty input and the root path both resolve to "/"; other valid paths have
// no trailing slash.
func NormalizeBaseURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/", true
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "?#\\") {
		return "", false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return "", false
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
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

// Sanitized returns a copy of the config with sensitive values redacted,
// safe to expose via /debug/config. Only debug-gated callers should use this.
func Sanitized() *Config {
	cfg := *Get()
	if cfg.Server.SSL.LetsEncrypt.DNSCredentials != "" {
		cfg.Server.SSL.LetsEncrypt.DNSCredentials = redactedValue
	}
	if cfg.Server.Security.EncryptionKey != "" {
		cfg.Server.Security.EncryptionKey = redactedValue
	}
	if cfg.Server.Database.URL != "" {
		cfg.Server.Database.URL = redactedValue
	}
	if cfg.Server.Backup.EncryptionPassword != "" {
		cfg.Server.Backup.EncryptionPassword = redactedValue
	}
	if cfg.Server.Notifications.Email.SMTP.Password != "" {
		cfg.Server.Notifications.Email.SMTP.Password = redactedValue
	}
	if cfg.Server.Token != "" {
		cfg.Server.Token = redactedValue
	}
	if cfg.Server.Metrics.Token != "" {
		cfg.Server.Metrics.Token = redactedValue
	}
	if cfg.Server.Metrics.Auth.Tokens.Prometheus != "" {
		cfg.Server.Metrics.Auth.Tokens.Prometheus = redactedValue
	}
	if cfg.Server.Metrics.Auth.Tokens.Grafana != "" {
		cfg.Server.Metrics.Auth.Tokens.Grafana = redactedValue
	}
	if cfg.Server.Metrics.Auth.Tokens.Loki != "" {
		cfg.Server.Metrics.Auth.Tokens.Loki = redactedValue
	}
	redactContactWebhooks(&cfg.Server.Contact)
	return &cfg
}

func redactContactWebhooks(contact *ContactConfig) {
	if contact.Admin.Email != "" {
		contact.Admin.Email = redactedValue
	}
	for _, role := range []*ContactRoleConfig{
		&contact.Admin, &contact.Security, &contact.Abuse, &contact.General,
	} {
		if len(role.Webhooks) == 0 {
			continue
		}
		redacted := make(map[string]string, len(role.Webhooks))
		for name, value := range role.Webhooks {
			if value != "" {
				redacted[name] = redactedValue
			} else {
				redacted[name] = ""
			}
		}
		role.Webhooks = redacted
	}
}

// redactedValue replaces a sensitive config value in sanitized output.
const redactedValue = "xxxxx"

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
	outputColor := "  # color: false\n"
	if cfg.Output.Color != nil {
		outputColor = fmt.Sprintf("  color: %t\n", *cfg.Output.Color)
	}
	outputEmoji := "  # emoji: true\n"
	if cfg.Output.Emoji != nil {
		outputEmoji = fmt.Sprintf("  emoji: %t\n", *cfg.Output.Emoji)
	}
	return fmt.Sprintf(`# Airports API Server Configuration
# Documentation: /docs

output:
  # Set true/false to override terminal color detection; omit for automatic behavior
%s  # Set true to keep emojis enabled when NO_COLOR is set
%s

cli:
  binary_download:
    # Require the server operator token to download CLI release assets
    require_auth: %t

server:
  # Single port (HTTP) or dual port (8090,64453 - second is HTTPS)
  port: "%s"
  # Fully qualified domain name
  fqdn: "%s"
  # Listen address ([::] for all interfaces IPv4/IPv6)
  address: "%s"
  # Application mode: production or development
  mode: "%s"
  # Debug diagnostics are gated by --debug / DEBUG, not by application mode
  debug:
    pprof: %t
    log_queries: %t
    log_cache: %t
    log_bodies: %t
    max_body_log_size: "%s"
    block_profile_rate: %d
    mutex_profile_fraction: %d
    runtime_endpoints: %t
  # Language for server output: auto detects LC_ALL/LANG, or set a language code
  lang: "%s"
  # API version prefix used in /api/{api_version}/ routes
  api_version: "%s"
  # External URL path prefix; reverse proxy headers take precedence
  baseurl: "%s"
  url_detection:
    learning: %t
    min_samples: %d
    sample_window: "%s"
    log_changes: %t
    live_reload: %t
  # Interface language settings
  i18n:
    enabled: %t
    default_language: "%s"
    available_languages: %s
    fallback_language: "%s"
    cookie_name: "%s"
    cookie_max_age: "%s"
  # Browser-session cookie defaults; max_age 0s means browser-session lifetime
  session:
    cookie_name: "%s"
    max_age: "%s"
    secure: "%s"
    http_only: %t
    same_site: "%s"
  # Global operator token, auto-generated on first run. Grants full access to
  # every server management operation, including "--maintenance restore" when
  # not running as root. Keep secret; clear this value to rotate it.
  token: "%s"
  update:
    # Release channel: stable | beta | devel (aliases normalize to devel)
    branch: "%s"
    # Auto-install updates found by the update_check task (default off)
    auto_install: %t
    # Defer window in days (0-365): a release is only eligible once this old
    defer_days: %d
    # Minimum CLI version accepted by the server; empty disables the minimum
    cli_min_version: "%s"
    # Internal state: newest version update_check has already notified about
    # (not operator-set; prevents re-notifying every scheduler run)
    last_notified_version: "%s"

  healthz:
    root:
      # Mount /healthz as an alias for /server/healthz
      enabled: %t

  # Branding text
  branding:
    title: "%s"
    tagline: "%s"
    description: "%s"
  seo:
    # SEO keywords
    keywords: %s

  # System user (auto-detected if empty)
  user: "%s"
  # System group (auto-detected if empty)
  group: "%s"
  # Write a PID file
  pidfile: %t
  # Detach from terminal on start
  daemonize: %t

  # SSL/TLS configuration
  ssl:
    enabled: %t
    # Manual certificate override path (leave empty for auto-detection)
    cert: "%s"
    # Manual key override path (leave empty for auto-detection)
    key: "%s"
    # TLS1.2 or TLS1.3
    min_version: "%s"
    # Let's Encrypt configuration
    letsencrypt:
      enabled: %t
      # Contact email for Let's Encrypt
      email: "%s"
      # Challenge type: dns-01, tls-alpn-01, http-01
      challenge: "%s"
      # Use Let's Encrypt staging server for testing
      staging: %t
      # For dns-01: ANY lego-supported provider name, e.g. cloudflare,
      # route53, digitalocean, godaddy, namecheap, rfc2136 - full list:
      # https://go-acme.github.io/lego/dns/
      dns_provider: "%s"
      # Provider credentials, AES-256-GCM encrypted (base64 JSON) using
      # server.security.encryption_key - never plaintext on disk.
      dns_credentials: "%s"
      # RFC3339 timestamp of the last successful credential validation
      dns_credentials_validated_at: "%s"
    # Smallest expiry-countdown threshold (30/14/7/3/1) already warned
    # about for the current certificate — internal state, do not edit.
    last_expiry_warning_days: %d

  # Built-in scheduler — never external cron
  scheduler:
    enabled: %t
    # IANA timezone used to evaluate cron schedules
    timezone: "%s"
    # How long after restart a missed task may still run
    catch_up_window: "%s"
    tasks:
      geoip_update:
        enabled: %t
        schedule: "%s"
        retry_on_fail: %t
        retry_delay: "%s"
      blocklist_update:
        enabled: %t
        schedule: "%s"
        retry_on_fail: %t
        retry_delay: "%s"
      cve_update:
        enabled: %t
        schedule: "%s"
        retry_on_fail: %t
        retry_delay: "%s"
      update_check:
        enabled: %t
        schedule: "%s"
      log_rotation:
        enabled: %t
        schedule: "%s"
      token_cleanup:
        enabled: %t
        schedule: "%s"
      backup:
        enabled: %t
        schedule: "%s"
        retention: %d
      backup_hourly:
        enabled: %t
        schedule: "%s"
      ssl_renewal:
        enabled: %t
        schedule: "%s"
        renew_before: "%s"
      health_check:
        enabled: %t
        schedule: "%s"
      tor_health:
        enabled: %t
        schedule: "%s"

  # Rate limiting
  rate_limit:
    enabled: %t
    read:
      requests: %d
      window: %d
    write:
      requests: %d
      window: %d
    health:
      requests: %d
      window: %d
    global_burst: %d

  # Database connection
  database:
    driver: "%s"
    url: "%s"

  # CORS
  cors:
    allowed_origins: %s

  # Trusted proxies (private ranges always trusted)
  trusted_proxies:
    # Additional IPs/CIDRs/DNS names to trust for X-Forwarded-* headers
    additional: %s

  # Request body and connection limits
  limits:
    max_body_size: "10MB"
    read_timeout: "30s"
    write_timeout: "30s"
    idle_timeout: "120s"

  # Compress configured MIME types when the client accepts gzip
  compression:
    enabled: true
    level: 5
    types:
      - text/html
      - text/css
      - text/javascript
      - application/json
      - application/xml

  # Self-healing maintenance mode
  maintenance:
    self_healing:
      enabled: %t
      retry_interval: "%s"
      # 0 = unlimited
      max_attempts: %d
    cleanup:
      # Start cleanup when disk usage exceeds this percent
      disk_threshold: %d
      log_retention_days: %d
      backup_keep_count: %d
    notify:
      on_enter: %t
      on_exit: %t

  # GeoIP settings
  geoip:
    enabled: %t
    # Path to GeoIP database
    database: "%s"
    # Allowed countries (empty = allow all)
    allow_countries: %s
    # Blocked countries
    deny_countries: %s
    # Named, operator-authored country lists for reuse across the allow/deny
    # fields above. Ships empty and is never auto-applied.
    presets: %s

  # Metrics/observability
  metrics:
    enabled: %t
    # Include CPU, memory, disk metrics
    include_system: %t
    # Metrics authentication is mandatory unless the server is firewalled.
    root:
      enabled: %t
    auth:
      allow_unauthenticated: %t
      tokens:
        prometheus: "%s"
        grafana: "%s"
        loki: "%s"
    include_runtime: %t
    loki:
      max_entries: %d
      max_age: "%s"
    # Histogram buckets for request duration (seconds)
    duration_buckets: %s
    # Histogram buckets for request size (bytes)
    size_buckets: %s

  # Logging configuration
  logs:
    # Global log level: debug, info, warn, error
    level: "%s"

    access:
      filename: "%s"
      # Format: apache, nginx, json, custom
      format: "%s"
      custom: "%s"
      rotate: "%s"
      keep: "%s"
      # Log successful health-check requests (failures always log)
      log_health_checks: %t

    server:
      filename: "%s"
      # Format: text, json
      format: "%s"
      custom: "%s"
      rotate: "%s"
      keep: "%s"

    error:
      filename: "%s"
      # Format: text, json
      format: "%s"
      custom: "%s"
      rotate: "%s"
      keep: "%s"

    audit:
      enabled: %t
      filename: "%s"
      # Format: json only (must be machine-parseable)
      format: "%s"
      rotate: "%s"
      keep: "%s"
      compress: %t

    security:
      filename: "%s"
      # Format: fail2ban, syslog, cef, json, text
      format: "%s"
      custom: "%s"
      rotate: "%s"
      keep: "%s"

    debug:
      enabled: %t
      filename: "%s"
      # Format: text, json
      format: "%s"
      custom: "%s"
      rotate: "%s"
      keep: "%s"

  # IP Block Management (PART 11): allowlist bypasses blocklist/rate-limit/
  # GeoIP/auto-block (never CSRF/path-security/TLS); blocked_ips are
  # permanent config-file-only blocks, released only by editing this file.
  security:
    allowlist: %s
    blocked_ips: %s
    # AES-256-GCM key for at-rest sensitive data (DNS-01 provider
    # credentials, backups). Auto-generated on first run if empty; never
    # exposed via /debug/config or logs.
    encryption_key: "%s"
    # Abuse detection: auto-block + auto-alert on request floods (all
    # enabled by default). Auto-blocking never applies to allowlisted IPs.
    abuse_detection:
      request_flood:
        # Rate-limit rejections (within the window) that trigger auto-block
        multiplier: %d
        block_duration: "%s"
      auto_block_ip: %t
      auto_alert: %t

  # Backup encryption and retention (PART 21)
  backup:
    # AES-256-GCM backup encryption password (Argon2id-derived key). Never
    # logged or exposed via /debug/config. Required when
    # server.compliance.enabled is true.
    encryption_password: "%s"
    retention:
      # 1-365: daily full backups to keep
      max_backups: %d
      # 0-52: Sunday backups (0 = disabled)
      keep_weekly: %d
      # 0-12: 1st-of-month backups (0 = disabled)
      keep_monthly: %d
      # 0-10: January 1st backups (0 = disabled)
      keep_yearly: %d
      # Hard size cap: percent of backup volume or absolute size; 0 = disabled
      max_total_size: "%s"

  # Compliance standards (PART 11): all disabled by default. Enabling any
  # standard turns on compliance backup enforcement (PART 21).
  compliance:
    enabled: %t
    gdpr: false
    ccpa: false
    hipaa: false
    soc2: false
    pci_dss: false
    iso27001: false
    fedramp: false
    lgpd: false
    pipeda: false
    appi: false
    pdpa: false

  # NVD CVE 2.0 feed settings for the scheduled cve_update task
  cve:
    # Optional NVD API key (raises rate limit from 5/30s to 50/30s)
    api_key: "%s"

  # WebUI toast + operator email notifications (PART 17). WebUI toasts are
  # visitor-facing (client-side only); email is operator-facing and
  # requires a working SMTP connection - no SMTP means no email, ever.
  notifications:
    webui:
      # top-right, top-left, bottom-right, bottom-left
      position: "%s"
      # seconds (0 = manual dismiss)
      duration: %d
    email:
      # Auto-managed: true only when SMTP is detected/configured and
      # verified working. Not a manual operator toggle.
      enabled: %t
      smtp:
        # If empty: autodetect local SMTP on startup. If set: test connection.
        host: "%s"
        port: %d
        username: "%s"
        password: "%s"
        # TLS mode: auto, starttls, tls, none
        tls: "%s"
      from:
        # Default: app title
        name: "%s"
        # Default: no-reply@{fqdn}
        email: "%s"
      # Optional Reply-To address, omitted from templates if empty
      reply_to: "%s"
      # Per-event email settings (override defaults). Missing key = true.
      events:%s

  # Optional I2P eepsite, disabled by default.
  i2p:
    enabled: false
    binary: ""
    sam_address: "127.0.0.1:7656"
    virtual_port: 80
    inbound_length: 3
    outbound_length: 3
    inbound_quantity: 5
    outbound_quantity: 5
    signature_type: 7
    bootstrap_timeout: "5m"

  # Tor hidden service (PART 31): starts automatically when a tor binary is found.
  # There is no enable/disable toggle. Never use default Tor ports - this app
  # always runs its own isolated Tor process with an auto-assigned control port.
  tor:
    # Path to Tor binary (auto-detected if empty)
    binary: "%s"

    # --- Outbound Network Settings ---
    # Use Tor network for outbound connections (server-wide setting)
    use_network: %t

    # --- Performance Settings ---
    # Maximum circuits to keep open (higher = faster but more memory)
    max_circuits: %d

    # Circuit timeout in seconds (how long before giving up)
    circuit_timeout: %d

    # Bootstrap timeout in seconds (wait for Tor network connection)
    bootstrap_timeout: %d

    # --- Security Settings ---
    # Scrub sensitive info from Tor logs
    safe_logging: %t

    # Maximum concurrent streams per circuit
    max_streams_per_circuit: %d

    # Close circuit when stream limit exceeded
    close_circuit_on_stream_limit: %t

    # --- Bandwidth Settings ---
    # Maximum bandwidth rate per second (e.g., "1 MB", "500 KB")
    bandwidth_rate: "%s"

    # Maximum bandwidth burst per second (e.g., "2 MB", "1 MB")
    bandwidth_burst: "%s"

    # Maximum monthly bandwidth (e.g., "100 GB", "50 GB", "unlimited")
    max_monthly_bandwidth: "%s"

    # --- Hidden Service Settings ---
    # Number of introduction points (3-10, more = resilient but more traffic)
    num_intro_points: %d

    # Virtual port for hidden service (what users connect to)
    virtual_port: %d

%s
web:
  ui:
    # Theme: dark, light, or auto
    theme: "%s"
    # Logo URL (local file or remote)
    logo: "%s"
    # Favicon path (local file or remote)
    favicon: "%s"
    notifications:
      enabled: %t
  # Site-wide announcement banners (see PART 16 -> "Site Banner")
  announcements:
    enabled: %t
    # List of announcement messages
    messages: %s
  # robots.txt configuration
  robots:
    # Allowed paths
    allow: %s
    # Denied paths
    deny: %s
    # Per-AI-crawler access control (default: allow all - no AI blocking)
    ai_bots:
      # Applies to any listed AI bot without an explicit value below
      default: "%s"
      # Per-bot overrides: allow | deny
      bots: %s
  security:
    # security.txt contact email
    admin: "%s"
`,
		outputColor,
		outputEmoji,
		cfg.CLI.BinaryDownload.RequireAuth,
		cfg.Server.Port,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Mode,
		cfg.Server.Debug.Pprof,
		cfg.Server.Debug.LogQueries,
		cfg.Server.Debug.LogCache,
		cfg.Server.Debug.LogBodies,
		cfg.Server.Debug.MaxBodyLogSize,
		cfg.Server.Debug.BlockProfileRate,
		cfg.Server.Debug.MutexProfileFraction,
		cfg.Server.Debug.RuntimeEndpoints,
		cfg.Server.Lang,
		cfg.Server.APIVersion,
		cfg.Server.BaseURL,
		cfg.Server.URLDetection.Learning,
		cfg.Server.URLDetection.MinSamples,
		cfg.Server.URLDetection.SampleWindow,
		cfg.Server.URLDetection.LogChanges,
		cfg.Server.URLDetection.LiveReload,
		cfg.Server.I18n.Enabled,
		cfg.Server.I18n.DefaultLanguage,
		formatStringSlice(cfg.Server.I18n.Supported),
		cfg.Server.I18n.FallbackLanguage,
		cfg.Server.I18n.CookieName,
		cfg.Server.I18n.CookieMaxAge,
		cfg.Server.Session.CookieName,
		cfg.Server.Session.MaxAge,
		cfg.Server.Session.Secure,
		cfg.Server.Session.HTTPOnly,
		cfg.Server.Session.SameSite,
		cfg.Server.Token,
		cfg.Server.Update.Branch,
		cfg.Server.Update.AutoInstall,
		cfg.Server.Update.DeferDays,
		cfg.Server.Update.CLIMinVersion,
		cfg.Server.Update.LastNotifiedVersion,
		cfg.Server.Healthz.Root.Enabled,
		cfg.Server.Branding.Title,
		cfg.Server.Branding.Tagline,
		cfg.Server.Branding.Description,
		formatStringSlice(cfg.Server.SEO.Keywords),
		cfg.Server.User,
		cfg.Server.Group,
		cfg.Server.PIDFile,
		cfg.Server.Daemonize,
		cfg.Server.SSL.Enabled,
		cfg.Server.SSL.Cert,
		cfg.Server.SSL.Key,
		cfg.Server.SSL.MinVersion,
		cfg.Server.SSL.LetsEncrypt.Enabled,
		cfg.Server.SSL.LetsEncrypt.Email,
		cfg.Server.SSL.LetsEncrypt.Challenge,
		cfg.Server.SSL.LetsEncrypt.Staging,
		cfg.Server.SSL.LetsEncrypt.DNSProvider,
		cfg.Server.SSL.LetsEncrypt.DNSCredentials,
		cfg.Server.SSL.LetsEncrypt.DNSCredentialsValidatedAt,
		cfg.Server.SSL.LastExpiryWarningDays,
		cfg.Server.Scheduler.Enabled,
		cfg.Server.Scheduler.Timezone,
		cfg.Server.Scheduler.CatchUpWindow,
		cfg.Server.Scheduler.Tasks.GeoIPUpdate.Enabled,
		cfg.Server.Scheduler.Tasks.GeoIPUpdate.Schedule,
		cfg.Server.Scheduler.Tasks.GeoIPUpdate.RetryOnFail,
		cfg.Server.Scheduler.Tasks.GeoIPUpdate.RetryDelay,
		cfg.Server.Scheduler.Tasks.BlocklistUpdate.Enabled,
		cfg.Server.Scheduler.Tasks.BlocklistUpdate.Schedule,
		cfg.Server.Scheduler.Tasks.BlocklistUpdate.RetryOnFail,
		cfg.Server.Scheduler.Tasks.BlocklistUpdate.RetryDelay,
		cfg.Server.Scheduler.Tasks.CVEUpdate.Enabled,
		cfg.Server.Scheduler.Tasks.CVEUpdate.Schedule,
		cfg.Server.Scheduler.Tasks.CVEUpdate.RetryOnFail,
		cfg.Server.Scheduler.Tasks.CVEUpdate.RetryDelay,
		cfg.Server.Scheduler.Tasks.UpdateCheck.Enabled,
		cfg.Server.Scheduler.Tasks.UpdateCheck.Schedule,
		cfg.Server.Scheduler.Tasks.LogRotation.Enabled,
		cfg.Server.Scheduler.Tasks.LogRotation.Schedule,
		cfg.Server.Scheduler.Tasks.TokenCleanup.Enabled,
		cfg.Server.Scheduler.Tasks.TokenCleanup.Schedule,
		cfg.Server.Scheduler.Tasks.Backup.Enabled,
		cfg.Server.Scheduler.Tasks.Backup.Schedule,
		cfg.Server.Scheduler.Tasks.Backup.Retention,
		cfg.Server.Scheduler.Tasks.BackupHourly.Enabled,
		cfg.Server.Scheduler.Tasks.BackupHourly.Schedule,
		cfg.Server.Scheduler.Tasks.SSLRenewal.Enabled,
		cfg.Server.Scheduler.Tasks.SSLRenewal.Schedule,
		cfg.Server.Scheduler.Tasks.SSLRenewal.RenewBefore,
		cfg.Server.Scheduler.Tasks.HealthCheck.Enabled,
		cfg.Server.Scheduler.Tasks.HealthCheck.Schedule,
		cfg.Server.Scheduler.Tasks.TorHealth.Enabled,
		cfg.Server.Scheduler.Tasks.TorHealth.Schedule,
		cfg.Server.RateLimit.Enabled,
		cfg.Server.RateLimit.Read.Requests,
		cfg.Server.RateLimit.Read.Window,
		cfg.Server.RateLimit.Write.Requests,
		cfg.Server.RateLimit.Write.Window,
		cfg.Server.RateLimit.Health.Requests,
		cfg.Server.RateLimit.Health.Window,
		cfg.Server.RateLimit.GlobalBurst,
		cfg.Server.Database.Driver,
		cfg.Server.Database.URL,
		formatStringSlice(cfg.Server.CORS.AllowedOrigins),
		formatStringSlice(cfg.Server.TrustedProxies.Additional),
		cfg.Server.Maintenance.SelfHealing.Enabled,
		cfg.Server.Maintenance.SelfHealing.RetryInterval,
		cfg.Server.Maintenance.SelfHealing.MaxAttempts,
		cfg.Server.Maintenance.Cleanup.DiskThreshold,
		cfg.Server.Maintenance.Cleanup.LogRetentionDays,
		cfg.Server.Maintenance.Cleanup.BackupKeepCount,
		cfg.Server.Maintenance.Notify.OnEnter,
		cfg.Server.Maintenance.Notify.OnExit,
		cfg.Server.GeoIP.Enabled,
		cfg.Server.GeoIP.Database,
		formatStringSlice(cfg.Server.GeoIP.AllowCountries),
		formatStringSlice(cfg.Server.GeoIP.DenyCountries),
		formatCountryPresets(cfg.Server.GeoIP.Presets),
		cfg.Server.Metrics.Enabled,
		cfg.Server.Metrics.IncludeSystem,
		cfg.Server.Metrics.Root.Enabled,
		cfg.Server.Metrics.Auth.AllowUnauthenticated,
		cfg.Server.Metrics.Auth.Tokens.Prometheus,
		cfg.Server.Metrics.Auth.Tokens.Grafana,
		cfg.Server.Metrics.Auth.Tokens.Loki,
		cfg.Server.Metrics.IncludeRuntime,
		cfg.Server.Metrics.Loki.MaxEntries,
		cfg.Server.Metrics.Loki.MaxAge,
		formatFloatSlice(cfg.Server.Metrics.DurationBuckets),
		formatFloatSlice(cfg.Server.Metrics.SizeBuckets),
		cfg.Server.Logging.Level,
		cfg.Server.Logging.Access.Filename,
		cfg.Server.Logging.Access.Format,
		cfg.Server.Logging.Access.Custom,
		cfg.Server.Logging.Access.Rotate,
		cfg.Server.Logging.Access.Keep,
		cfg.Server.Logging.Access.LogHealthChecks,
		cfg.Server.Logging.Server.Filename,
		cfg.Server.Logging.Server.Format,
		cfg.Server.Logging.Server.Custom,
		cfg.Server.Logging.Server.Rotate,
		cfg.Server.Logging.Server.Keep,
		cfg.Server.Logging.Error.Filename,
		cfg.Server.Logging.Error.Format,
		cfg.Server.Logging.Error.Custom,
		cfg.Server.Logging.Error.Rotate,
		cfg.Server.Logging.Error.Keep,
		cfg.Server.Logging.Audit.Enabled,
		cfg.Server.Logging.Audit.Filename,
		cfg.Server.Logging.Audit.Format,
		cfg.Server.Logging.Audit.Rotate,
		cfg.Server.Logging.Audit.Keep,
		cfg.Server.Logging.Audit.Compress,
		cfg.Server.Logging.Security.Filename,
		cfg.Server.Logging.Security.Format,
		cfg.Server.Logging.Security.Custom,
		cfg.Server.Logging.Security.Rotate,
		cfg.Server.Logging.Security.Keep,
		cfg.Server.Logging.Debug.Enabled,
		cfg.Server.Logging.Debug.Filename,
		cfg.Server.Logging.Debug.Format,
		cfg.Server.Logging.Debug.Custom,
		cfg.Server.Logging.Debug.Rotate,
		cfg.Server.Logging.Debug.Keep,
		formatAllowlist(cfg.Server.Security.Allowlist),
		formatBlockedIPs(cfg.Server.Security.BlockedIPs),
		cfg.Server.Security.EncryptionKey,
		cfg.Server.Security.AbuseDetection.RequestFlood.Multiplier,
		cfg.Server.Security.AbuseDetection.RequestFlood.BlockDuration,
		cfg.Server.Security.AbuseDetection.AutoBlockIP,
		cfg.Server.Security.AbuseDetection.AutoAlert,
		cfg.Server.Backup.EncryptionPassword,
		cfg.Server.Backup.Retention.MaxBackups,
		cfg.Server.Backup.Retention.KeepWeekly,
		cfg.Server.Backup.Retention.KeepMonthly,
		cfg.Server.Backup.Retention.KeepYearly,
		cfg.Server.Backup.Retention.MaxTotalSize,
		cfg.Server.Compliance.IsEnabled(),
		cfg.Server.CVE.APIKey,
		cfg.Server.Notifications.WebUI.Position,
		cfg.Server.Notifications.WebUI.Duration,
		cfg.Server.Notifications.Email.Enabled,
		cfg.Server.Notifications.Email.SMTP.Host,
		cfg.Server.Notifications.Email.SMTP.Port,
		cfg.Server.Notifications.Email.SMTP.Username,
		cfg.Server.Notifications.Email.SMTP.Password,
		cfg.Server.Notifications.Email.SMTP.TLS,
		cfg.Server.Notifications.Email.From.Name,
		cfg.Server.Notifications.Email.From.Email,
		cfg.Server.Notifications.Email.ReplyTo,
		formatNotificationEvents(cfg.Server.Notifications.Email.Events),
		cfg.Server.Tor.Binary,
		cfg.Server.Tor.UseNetwork,
		cfg.Server.Tor.MaxCircuits,
		cfg.Server.Tor.CircuitTimeout,
		cfg.Server.Tor.BootstrapTimeout,
		cfg.Server.Tor.SafeLogging,
		cfg.Server.Tor.MaxStreamsPerCircuit,
		cfg.Server.Tor.CloseCircuitOnStreamLimit,
		cfg.Server.Tor.BandwidthRate,
		cfg.Server.Tor.BandwidthBurst,
		cfg.Server.Tor.MaxMonthlyBandwidth,
		cfg.Server.Tor.NumIntroPoints,
		cfg.Server.Tor.VirtualPort,
		formatServerPublicSettings(cfg.Server.Contact, cfg.Server.Pages.Contact, cfg.Server.Tracking, cfg.Server.Privacy),
		cfg.Web.UI.Theme,
		cfg.Web.UI.Logo,
		cfg.Web.UI.Favicon,
		cfg.Web.UI.Notifications.Enabled,
		cfg.Web.Announcements.Enabled,
		formatAnnouncementMessages(cfg.Web.Announcements.Messages),
		formatStringSlice(cfg.Web.Robots.Allow),
		formatStringSlice(cfg.Web.Robots.Deny),
		cfg.Web.Robots.AIBots.Default,
		formatAIBots(cfg.Web.Robots.AIBots.Bots),
		cfg.Web.Security.Admin,
	)
}

// formatFloatSlice formats a float64 slice (e.g. histogram buckets) for YAML
// output, trimming trailing zeros so whole numbers render as "100" rather
// than "100.000000".
func formatFloatSlice(s []float64) string {
	if len(s) == 0 {
		return "[]"
	}
	result := "["
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += strconv.FormatFloat(v, 'g', -1, 64)
	}
	result += "]"
	return result
}

func formatServerPublicSettings(cfg ContactConfig, page ContactPageConfig, tracking TrackingConfig, privacy PrivacyConfig) string {
	var out strings.Builder
	out.WriteString("  contact:\n")
	roles := []struct {
		name string
		cfg  ContactRoleConfig
	}{
		{"admin", cfg.Admin},
		{"security", cfg.Security},
		{"abuse", cfg.Abuse},
		{"general", cfg.General},
	}
	for _, role := range roles {
		fmt.Fprintf(&out, "    %s:\n      email: %s\n      webhooks:\n", role.name, strconv.Quote(role.cfg.Email))
		keys := []string{"telegram", "discord", "slack", "mattermost", "pushover", "gotify", "generic"}
		seen := make(map[string]struct{}, len(keys)+len(role.cfg.Webhooks))
		for _, key := range keys {
			fmt.Fprintf(&out, "        %s: %s\n", key, strconv.Quote(role.cfg.Webhooks[key]))
			seen[key] = struct{}{}
		}
		extra := make([]string, 0, len(role.cfg.Webhooks))
		for key := range role.cfg.Webhooks {
			if _, ok := seen[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(extra)
		for _, key := range extra {
			fmt.Fprintf(&out, "        %s: %s\n", strconv.Quote(key), strconv.Quote(role.cfg.Webhooks[key]))
		}
	}
	fmt.Fprintf(&out, "  pages:\n    contact:\n      enabled: %t\n      captcha: %s\n      success_message: %s\n",
		page.Enabled, strconv.Quote(page.Captcha), strconv.Quote(page.SuccessMessage))
	fmt.Fprintf(&out, "  tracking:\n    type: %s\n    id: %s\n    url: %s\n",
		strconv.Quote(tracking.Type), strconv.Quote(tracking.ID), strconv.Quote(tracking.URL))
	fmt.Fprintf(&out, "  privacy:\n    data:\n      sold: %t\n      stored_on_server: %t\n", privacy.Data.Sold, privacy.Data.StoredOnServer)
	if len(privacy.Data.Sharing) == 0 {
		out.WriteString("      sharing: []\n")
	} else {
		out.WriteString("      sharing:\n")
	}
	for _, condition := range privacy.Data.Sharing {
		fmt.Fprintf(&out, "        - condition: %s\n          when: %s\n          data: %s\n", strconv.Quote(condition.Condition), strconv.Quote(condition.When), strconv.Quote(condition.Data))
	}
	fmt.Fprintf(&out, "    retention:\n      period: %s\n      export_available: %t\n      deletion_available: %t\n",
		strconv.Quote(privacy.Retention.Period), privacy.Retention.ExportAvailable, privacy.Retention.DeletionAvailable)
	consent := privacy.Consent
	fmt.Fprintf(&out, "    consent:\n      show_until_acknowledged: %t\n      default_enabled: %t\n      message: %s\n      message_if_sold: %s\n      policy:\n        text: %s\n        url: %s\n      buttons:\n        decline: %s\n        accept: %s\n      position: %s\n      show_preferences: %t\n      preferences_text: %s\n",
		consent.ShowUntilAcknowledged, consent.DefaultEnabled,
		strconv.Quote(consent.Message), strconv.Quote(consent.MessageIfSold),
		strconv.Quote(consent.Policy.Text), strconv.Quote(consent.Policy.URL),
		strconv.Quote(consent.Buttons.Decline), strconv.Quote(consent.Buttons.Accept),
		strconv.Quote(consent.Position), consent.ShowPreferences, strconv.Quote(consent.PreferencesText))
	fmt.Fprintf(&out, "    cookies:\n      essential:\n        enabled: %t\n        description: %s\n      preferences:\n        enabled: %t\n        description: %s\n      analytics:\n        enabled: %t\n        description: %s\n        description_suffix_not_sold: %s\n        description_suffix_sold: %s\n",
		privacy.Cookies.Essential.Enabled, strconv.Quote(privacy.Cookies.Essential.Description),
		privacy.Cookies.Preferences.Enabled, strconv.Quote(privacy.Cookies.Preferences.Description),
		privacy.Cookies.Analytics.Enabled, strconv.Quote(privacy.Cookies.Analytics.Description),
		strconv.Quote(privacy.Cookies.Analytics.DescriptionSuffixNotSold), strconv.Quote(privacy.Cookies.Analytics.DescriptionSuffixSold))
	out.WriteString("    third_party:\n")
	if len(privacy.ThirdParty.Services) == 0 {
		out.WriteString("      services: []\n")
	} else {
		out.WriteString("      services:\n")
	}
	for _, service := range privacy.ThirdParty.Services {
		fmt.Fprintf(&out, "        - name: %s\n          purpose: %s\n          data_sent: %s\n          policy_url: %s\n",
			strconv.Quote(service.Name), strconv.Quote(service.Purpose), strconv.Quote(service.DataSent), strconv.Quote(service.PolicyURL))
	}
	fmt.Fprintf(&out, "    content:\n      data_collection: %s\n      data_usage: %s\n      data_usage_if_sold: %s\n      data_security: %s\n",
		strconv.Quote(privacy.Content.DataCollection), strconv.Quote(privacy.Content.DataUsage),
		strconv.Quote(privacy.Content.DataUsageIfSold), strconv.Quote(privacy.Content.DataSecurity))
	return strings.TrimSuffix(out.String(), "\n")
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

// formatAllowlist renders server.security.allowlist as a YAML block sequence
func formatAllowlist(entries []AllowlistEntry) string {
	if len(entries) == 0 {
		return "[]"
	}
	var b strings.Builder
	for _, e := range entries {
		_, _ = fmt.Fprintf(&b, "\n      - cidr: %q\n        description: %q", e.CIDR, e.Description)
	}
	return b.String()
}

// formatBlockedIPs renders server.security.blocked_ips as a YAML block sequence
func formatBlockedIPs(entries []BlockedIPEntry) string {
	if len(entries) == 0 {
		return "[]"
	}
	var b strings.Builder
	for _, e := range entries {
		_, _ = fmt.Fprintf(&b, "\n      - cidr: %q\n        reason: %q", e.CIDR, e.Reason)
	}
	return b.String()
}

// formatAnnouncementMessages renders web.announcements.messages as a YAML
// block sequence, per AI.md PART 16 "Announcement Structure".
func formatAnnouncementMessages(messages []AnnouncementMessage) string {
	if len(messages) == 0 {
		return "[]"
	}
	var b strings.Builder
	for _, m := range messages {
		_, _ = fmt.Fprintf(&b,
			"\n      - id: %q\n        type: %q\n        title: %q\n        message: %q\n        start: %q\n        end: %q\n        dismissible: %t",
			m.ID, m.Type, m.Title, m.Message, m.Start, m.End, m.Dismissible)
	}
	return b.String()
}

// formatNotificationEvents renders server.notifications.email.events as a
// YAML block mapping in the fixed key order defined by
// notificationEventOrder, since Go map iteration order is non-deterministic.
func formatNotificationEvents(events map[string]bool) string {
	var b strings.Builder
	for _, key := range notificationEventOrder {
		_, _ = fmt.Fprintf(&b, "\n        %s: %t", key, events[key])
	}
	return b.String()
}

// formatCountryPresets renders server.geoip.presets as a YAML block mapping of
// preset name to an inline list of ISO 3166-1 alpha-2 codes, per AI.md PART 19
// "Country Blocking Presets". Keys are sorted so the file is stable.
func formatCountryPresets(presets map[string][]string) string {
	if len(presets) == 0 {
		return "{}"
	}
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		_, _ = fmt.Fprintf(&b, "\n      %s: %s", name, formatStringSlice(presets[name]))
	}
	return b.String()
}

// formatAIBots renders web.robots.ai_bots.bots as a YAML block mapping of
// crawler User-agent token to allow/deny, per AI.md PART 11 "AI Crawler
// Rules". Keys are sorted so the file is stable.
func formatAIBots(bots map[string]string) string {
	if len(bots) == 0 {
		return "{}"
	}
	names := make([]string, 0, len(bots))
	for name := range bots {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		_, _ = fmt.Fprintf(&b, "\n        %s: %q", name, bots[name])
	}
	return b.String()
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

// SetUpdateBranch updates the canonical update channel in configuration.
func SetUpdateBranch(branch string) error {
	mu.Lock()
	defer mu.Unlock()
	normalized, ok := NormalizeUpdateBranch(branch)
	if !ok {
		return fmt.Errorf("invalid update branch %q (expected stable, beta, or devel)", branch)
	}
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Update.Branch = normalized
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// SetLastNotifiedVersion persists the newest update version the update_check
// scheduled task has already notified about, so the WARN log + email fire
// once per version rather than on every scheduler run (PART 22).
func SetLastNotifiedVersion(version string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Update.LastNotifiedVersion = version
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// SetSSLLastExpiryWarningDays persists the smallest expiry-countdown
// threshold (30/14/7/3/1) already warned about for the current certificate,
// so the daily ssl_renewal task's expiry check fires each threshold once
// (PART 17). Pass 0 to clear the state after a successful renewal.
func SetSSLLastExpiryWarningDays(days int) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.SSL.LastExpiryWarningDays = days
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// GetTheme returns the current theme
func GetTheme() string {
	cfg := Get()
	return cfg.Web.UI.Theme
}

// GetCORS returns the configured CORS allowed origins, defaulting to "*"
func GetCORS() []string {
	cfg := Get()
	if len(cfg.Server.CORS.AllowedOrigins) == 0 {
		return []string{"*"}
	}
	return cfg.Server.CORS.AllowedOrigins
}
