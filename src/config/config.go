package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"log"
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

// ApplicationConfig represents the complete server configuration
type ApplicationConfig struct {
	Server               ServerConfig `yaml:"server"`
	Web                  WebConfig    `yaml:"web"`
	Data                 DataConfig   `yaml:"data,omitempty"`
	CLI                  CLIConfig    `yaml:"cli,omitempty"`
	Output               OutputConfig `yaml:"output,omitempty"`
	generatedSecretPaths []string
}

// GeneratedSecretPaths returns the names of secrets generated during this
// load. Callers use the names to write config.secret_generated audit events;
// secret values are never returned.
func (cfg *ApplicationConfig) GeneratedSecretPaths() []string {
	if cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.generatedSecretPaths...)
}

// DataConfig contains project data-service credentials, per AI.md PART 8.
type DataConfig struct {
	CVE CVEConfig `yaml:"cve"`
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
	// Tokens controls owner token issuance and retention (PART 8).
	Tokens OwnerTokensConfig `yaml:"tokens"`
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
	Cache      CacheConfig       `yaml:"cache"`         // Cache backend (memory/valkey/redis)
	Security   SecurityConfig    `yaml:"security"`      // IP allowlist/blocklist management
	CVE        CVEConfig         `yaml:"cve,omitempty"` // Legacy alias; canonical key is data.cve
	Backup     BackupConfig      `yaml:"backup"`        // Backup encryption/retention settings
	Compliance ComplianceConfig  `yaml:"compliance"`    // Compliance-standard enforcement
	Tor        tor.TorConfig     `yaml:"tor"`           // Tor hidden service settings (PART 31)
	// I2P configures the optional eepsite.
	I2P           i2p.I2PConfig             `yaml:"i2p"`
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

// OwnerTokensConfig controls linked token limits and owner-token cookies.
type OwnerTokensConfig struct {
	MaxLinked     int    `yaml:"max_linked"`
	CookieMaxAge  string `yaml:"cookie_max_age"`
	IdlePurgeDays int    `yaml:"idle_purge_days"`
}

// ServerNotificationsConfig contains WebUI toast (visitor-facing, client
// side only) and operator email settings per AI.md PART 17 "EMAIL &
// NOTIFICATIONS". This is distinct from Web.UI.Notifications, which
// controls the server-rendered site announcement banner (yaml path
// web.ui.notifications) - the two never collide.
type ServerNotificationsConfig struct {
	WebUI   NotifyWebUIConfig `yaml:"webui"` // Toast position/duration
	Email   NotifyEmailConfig `yaml:"email"` // Operator email notifications
	DiskLow DiskLowConfig     `yaml:"disk_low"`
}

// DiskLowConfig controls per-volume low-disk warnings and repeat cadence.
type DiskLowConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ThresholdPercent int    `yaml:"threshold_percent"`
	CheckInterval    string `yaml:"check_interval"`
	Repeat           string `yaml:"repeat"`
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
	Password string `yaml:"password"` // Optional SMTP auth password, never exposed via GetSanitizedApplicationConfig()
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
	"security_report_received",
	"security_report_acknowledgment",
	"security_report_status",
	"scheduler_error",
	"update_available",
	"update_installed",
	"update_failed",
	"update_staged",
	"disk_low",
}

// SecurityConfig contains IP Block Management settings per AI.md PART 11
// "IP Block Management". Allowlist entries always bypass blocklist/rate
// limit/GeoIP/auto-block (never CSRF/path-security/TLS); BlockedIPs are
// permanent, config-file-only blocks released only by editing the config.
type SecurityConfig struct {
	Allowlist            []AllowlistEntry      `yaml:"allowlist"`      // Trusted IPs/CIDRs, bypass blocklist/rate-limit/GeoIP/auto-block
	BlockedIPs           []BlockedIPEntry      `yaml:"blocked_ips"`    // Permanent config-file blocks
	EncryptionKey        string                `yaml:"encryption_key"` // AES-256-GCM key for at-rest sensitive data (DNS-01 provider credentials, backups); never exposed via GetSanitizedApplicationConfig()
	EncryptionKeyVersion int                   `yaml:"encryption_key_version,omitempty"`
	Reports              SecurityReportsConfig `yaml:"reports"`
	AbuseDetection       AbuseDetectionConfig  `yaml:"abuse_detection"` // Request-flood auto-block + auto-alert (PART 11 "Abuse Detection")
}

// SecurityReportsConfig controls security-report retention per AI.md PART 11.
type SecurityReportsConfig struct {
	RetentionDays int `yaml:"retention_days"`
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
	// derived via Argon2id). Required when any compliance standard is enabled.
	// Never exposed via GetSanitizedApplicationConfig() or logs.
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
	// GDPRAllowedRegions records operator-selected data residency regions.
	GDPRAllowedRegions []string `yaml:"gdpr_allowed_regions"`
	// FedRAMPImpactLevel is Low, Moderate, or High when FedRAMP is enabled.
	FedRAMPImpactLevel string `yaml:"fedramp_impact_level"`
}

func (c ComplianceConfig) IsEnabled() bool {
	return c.GDPR || c.CCPA || c.HIPAA || c.SOC2 || c.PCIDSS || c.ISO27001 || c.FedRAMP || c.LGPD || c.PIPEDA || c.APPI || c.PDPA
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
	DefaultLanguage string   `yaml:"default_language"`
	Supported       []string `yaml:"available_languages"`
}

// BrandingConfig contains project branding text
type BrandingConfig struct {
	Title       string                     `yaml:"title"`       // Displayed project title
	Tagline     string                     `yaml:"tagline"`     // Short tagline
	Description string                     `yaml:"description"` // Longer description
	Favicon     string                     `yaml:"favicon"`
	Logo        string                     `yaml:"logo"`
	Content     map[string]BrandingContent `yaml:"content"`
}

// BrandingContent is an operator-managed public page extension.
type BrandingContent struct {
	Format string      `yaml:"format"`
	Body   interface{} `yaml:"body"`
}

// SEOConfig contains SEO metadata
type SEOConfig struct {
	Keywords       []string              `yaml:"keywords"`
	Author         string                `yaml:"author"`
	OGImage        string                `yaml:"og_image"`
	TwitterHandle  string                `yaml:"twitter_handle"`
	TitleSeparator string                `yaml:"title_separator"`
	MetaTags       map[string]string     `yaml:"meta_tags"`
	Canonical      string                `yaml:"canonical"`
	NoIndex        bool                  `yaml:"noindex"`
	OpenGraph      SEOOpenGraphConfig    `yaml:"opengraph"`
	Twitter        SEOTwitterConfig      `yaml:"twitter"`
	Verification   SEOVerificationConfig `yaml:"verification"`
	Sitemap        SEOSitemapConfig      `yaml:"sitemap"`
	Feed           SEOFeedConfig         `yaml:"feed"`
}

type SEOOpenGraphConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`
	SiteName string `yaml:"site_name"`
}

type SEOTwitterConfig struct {
	Enabled bool   `yaml:"enabled"`
	Card    string `yaml:"card"`
	Creator string `yaml:"creator"`
}

type SEOSitemapConfig struct {
	MaxURLs       int  `yaml:"max_urls"`
	IncludeImages bool `yaml:"include_images"`
}

type SEOFeedConfig struct {
	MaxItems int    `yaml:"max_items"`
	Title    string `yaml:"title"`
}

type SEOVerificationConfig struct {
	Google    string             `yaml:"google"`
	Bing      string             `yaml:"bing"`
	Yandex    string             `yaml:"yandex"`
	Baidu     string             `yaml:"baidu"`
	Pinterest string             `yaml:"pinterest"`
	Facebook  string             `yaml:"facebook"`
	Custom    []SEOCustomMetaTag `yaml:"custom"`
}

type SEOCustomMetaTag struct {
	Name     string `yaml:"name"`
	Property string `yaml:"property"`
	Content  string `yaml:"content"`
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
	DiskCheck       TaskConfig       `yaml:"disk_check"`
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
	Read         RateLimitBucket `yaml:"read"`
	Write        RateLimitBucket `yaml:"write"`
	Health       RateLimitBucket `yaml:"health"`
	Strict       RateLimitBucket `yaml:"strict"`
	AuthFailures RateLimitBucket `yaml:"auth_failures"`
	GlobalBurst  int             `yaml:"global_burst"`
}

// RateLimitBucket is a single requests-per-window rate limit rule
type RateLimitBucket struct {
	Requests int `yaml:"requests"` // Requests allowed per window
	Window   int `yaml:"window"`   // Window length in seconds
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	Driver string             `yaml:"driver"`          // sqlite or libsql
	URL    string             `yaml:"url"`             // Connection URL (auto-created path for sqlite)
	Token  string             `yaml:"token,omitempty"` // libsql authentication token; stored in secrets.yml
	Pool   DatabasePoolConfig `yaml:"pool"`            // Connection pool tuning (AI.md PART 10 "Connection Pooling")
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
	LLMS          WebLLMSConfig       `yaml:"llms"`
	Security      WebSecurityConfig   `yaml:"security"`
	Announcements AnnouncementsConfig `yaml:"announcements"` // Site banner announcements (PART 16 "Site Banner")
	APIContact    string              `yaml:"api_contact"`
}

// WebLLMSConfig controls the generated AI-agent discovery document.
type WebLLMSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	IncludeEndpoints bool     `yaml:"include_endpoints"`
	IncludeSchemas   bool     `yaml:"include_schemas"`
	CustomSections   []string `yaml:"custom_sections"`
}

// WebUIConfig contains web UI settings
type WebUIConfig struct {
	Theme         string              `yaml:"theme"` // dark, light, or auto
	Accent        string              `yaml:"accent"`
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
}

// WebSecurityConfig contains security.txt / contact settings
type WebSecurityConfig struct {
	Admin         string   `yaml:"admin"`
	ReportURL     string   `yaml:"report_url"`
	Contact       string   `yaml:"contact"`
	Expires       string   `yaml:"expires"`
	Policy        string   `yaml:"policy"`
	Keyservers    []string `yaml:"keyservers"`
	PublishPGPKey bool     `yaml:"publish_pgp_key"`
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
		Text    string `yaml:"text"`
		URL     string `yaml:"url"`
		Version string `yaml:"version"`
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
	DPO      DPOContactConfig  `yaml:"dpo"`
	General  ContactRoleConfig `yaml:"general"`
}

// DPOContactConfig defines the public data protection officer contact.
type DPOContactConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// ContactRoleConfig contains one role's email and named webhook destinations.
type ContactRoleConfig struct {
	Email    string            `yaml:"email"`
	Webhooks map[string]string `yaml:"webhooks"`
}

// ServerPagesConfig contains configurable public-page behavior.
type ServerPagesConfig struct {
	Contact ContactPageConfig `yaml:"contact"`
	Help    HelpPagesConfig   `yaml:"help"`
}

type HelpPagesConfig struct {
	Content string `yaml:"content"`
	MCP     string `yaml:"mcp"`
}

// ContactPageConfig controls the public contact form per PART 16.
type ContactPageConfig struct {
	Enabled          bool                  `yaml:"enabled"`
	Captcha          string                `yaml:"captcha"`
	CaptchaSiteKey   string                `yaml:"captcha_site_key"`
	CaptchaSecretKey string                `yaml:"captcha_secret_key"`
	Antispam         ContactAntispamConfig `yaml:"antispam"`
	SuccessMessage   string                `yaml:"success_message"`
}

// ContactAntispamConfig controls the server-side contact-form protections
// described in AI.md PART 16 "Contact antispam".
type ContactAntispamConfig struct {
	Honeypot             bool     `yaml:"honeypot"`
	MinFillSeconds       int      `yaml:"min_fill_seconds"`
	MaxAgeSeconds        int      `yaml:"max_age_seconds"`
	PerIP                int      `yaml:"per_ip"`
	PerEmail             int      `yaml:"per_email"`
	Global               int      `yaml:"global"`
	Window               string   `yaml:"window"`
	MaxLinks             int      `yaml:"max_links"`
	RejectMarkup         bool     `yaml:"reject_markup"`
	BlockedWords         []string `yaml:"blocked_words"`
	BlockedEmailDomains  []string `yaml:"blocked_email_domains"`
	Strikes              int      `yaml:"strikes"`
	BanHours             int      `yaml:"ban_hours"`
	ReviewMinSeconds     int      `yaml:"review_min_seconds"`
	ReviewMaxSeconds     int      `yaml:"review_max_seconds"`
	DuplicateWindowHours int      `yaml:"duplicate_window_hours"`
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
	case "dpo":
		return firstContactEmail(c.DPO.Email, c.General.Email)
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

// ResolveWebhookSigningSecret returns the signing secret that belongs to the
// effective URL selected by ResolveWebhook's role fallback order.
func (c ContactConfig) ResolveWebhookSigningSecret(role, transport string) string {
	roles := []*ContactRoleConfig{}
	switch role {
	case "admin":
		roles = []*ContactRoleConfig{&c.Admin}
	case "security":
		roles = []*ContactRoleConfig{&c.Security, &c.Admin}
	case "abuse":
		roles = []*ContactRoleConfig{&c.Abuse, &c.General, &c.Admin}
	case "general":
		roles = []*ContactRoleConfig{&c.General, &c.Admin}
	}
	for _, candidate := range roles {
		if strings.TrimSpace(candidate.Webhooks[transport]) != "" {
			return strings.TrimSpace(candidate.Webhooks[transport+"_secret"])
		}
	}
	return ""
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
	current    *ApplicationConfig
	mu         sync.RWMutex
	configPath string
)

func LoadApplicationConfig(path string) (*ApplicationConfig, error) {
	mu.Lock()
	defer mu.Unlock()

	// Migrate .yaml to .yml if needed
	path = migrateYamlToYml(path)
	configPath = path
	if err := secureConfigDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultApplicationConfig()
		if err := loadBrandingDropIn(cfg, filepath.Dir(path)); err != nil {
			return nil, err
		}
		applyInitOnlyEnvOverrides(cfg)
		applySMTPEnvOverrides(cfg)
		applyCacheEnvOverrides(cfg)
		applyI2PEnvOverrides(cfg)
		if _, err := loadSecrets(filepath.Join(filepath.Dir(path), "secrets.yml"), cfg); err != nil {
			return nil, err
		}
		cfg.Server.CVE = cfg.Data.CVE
		generated := make([]string, 0, 8)
		if changed, err := ensureEncryptionKey(cfg); err != nil {
			return nil, err
		} else if changed {
			generated = append(generated, "server.security.encryption_key")
		}
		if changed, err := ensureOperatorToken(cfg); err != nil {
			return nil, err
		} else if changed {
			generated = append(generated, "server.token")
		}
		metricsMissing := missingMetricsTokenPaths(cfg)
		if _, err := ensureMetricsTokens(cfg); err != nil {
			return nil, err
		}
		generated = append(generated, metricsMissing...)
		webhookSecrets, err := ensureWebhookSigningSecrets(cfg)
		if err != nil {
			return nil, err
		}
		generated = append(generated, webhookSecrets...)
		cfg.generatedSecretPaths = generated
		validateConfig(cfg)
		if err := saveInitialApplicationConfig(cfg, path); err != nil {
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
	if err := os.Chmod(path, 0600); err != nil {
		return nil, fmt.Errorf("failed to secure config file: %w", err)
	}
	previousConfig := current
	if migrated, migrationErr := migrateLegacyBranding(path, data); migrationErr != nil {
		log.Printf("Warning: skipped legacy branding migration: %v", migrationErr)
	} else {
		data = migrated
	}

	cfg := DefaultApplicationConfig()
	configFileValid := true
	if err := applyConfigYAML(cfg, data, path); err != nil {
		log.Printf("Warning: ignoring invalid config file %s: %v", path, err)
		configFileValid = false
		cfg = previousConfigOrDefaults(previousConfig)
	}
	if err := loadBrandingDropIn(cfg, filepath.Dir(path)); err != nil {
		return nil, err
	}
	legacyCVEKey := cfg.Server.CVE.APIKey
	if cfg.Data.CVE.APIKey == "" {
		cfg.Data.CVE.APIKey = legacyCVEKey
	}
	cfg.Server.CVE = cfg.Data.CVE
	fileSecrets := secretConfigValues(cfg)
	secretPath := filepath.Join(filepath.Dir(path), "secrets.yml")
	_, err = loadSecrets(secretPath, cfg)
	if err != nil {
		return nil, err
	}
	cfg.Server.CVE = cfg.Data.CVE
	originalUpdateBranch := cfg.Server.Update.Branch

	applySMTPEnvOverrides(cfg)
	applyCacheEnvOverrides(cfg)
	applyI2PEnvOverrides(cfg)
	validateConfig(cfg)

	needsPersist := len(fileSecrets) > 0
	generated := make([]string, 0, 8)
	if changed, err := ensureEncryptionKey(cfg); err != nil {
		return nil, err
	} else if changed {
		needsPersist = true
		generated = append(generated, "server.security.encryption_key")
	}
	if cfg.Server.Update.Branch != originalUpdateBranch {
		needsPersist = true
	}
	if cfg.Server.Metrics.Token != "" && cfg.Server.Metrics.Auth.Tokens.Prometheus == "" {
		cfg.Server.Metrics.Auth.Tokens.Prometheus = cfg.Server.Metrics.Token
		cfg.Server.Metrics.Token = ""
		needsPersist = true
	}
	if legacyCVEKey != "" {
		needsPersist = true
	}
	operatorTokenGenerated, err := ensureOperatorToken(cfg)
	if err != nil {
		return nil, err
	} else if operatorTokenGenerated {
		needsPersist = true
		generated = append(generated, "server.token")
	}
	metricsMissing := missingMetricsTokenPaths(cfg)
	if changed, err := ensureMetricsTokens(cfg); err != nil {
		return nil, err
	} else if changed {
		needsPersist = true
		generated = append(generated, metricsMissing...)
	}
	webhookSecrets, err := ensureWebhookSigningSecrets(cfg)
	if err != nil {
		return nil, err
	}
	if len(webhookSecrets) > 0 {
		needsPersist = true
		generated = append(generated, webhookSecrets...)
	}
	cfg.generatedSecretPaths = generated
	if needsPersist {
		var persistErr error
		if configFileValid {
			persistErr = saveConfig(cfg, path)
		} else {
			persistErr = saveSecrets(filepath.Join(filepath.Dir(path), "secrets.yml"), cfg)
		}
		if persistErr != nil {
			return nil, fmt.Errorf("failed to persist generated server secrets: %w", persistErr)
		}
	}
	applyDatabaseEnvOverrides(cfg)

	current = cfg
	return cfg, nil
}

func previousConfigOrDefaults(previous *ApplicationConfig) *ApplicationConfig {
	if previous == nil {
		return DefaultApplicationConfig()
	}
	data, err := yaml.Marshal(previous)
	if err != nil {
		return DefaultApplicationConfig()
	}
	copy := DefaultApplicationConfig()
	if err := yaml.Unmarshal(data, copy); err != nil {
		return DefaultApplicationConfig()
	}
	return copy
}

func validateYAMLDocument(data []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	var visit func(*yaml.Node) error
	visit = func(node *yaml.Node) error {
		if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
			return fmt.Errorf("explicit null at line %d", node.Line)
		}
		if node.Kind == yaml.MappingNode {
			keys := make(map[string]int, len(node.Content)/2)
			for index := 0; index+1 < len(node.Content); index += 2 {
				key := node.Content[index]
				if key.Kind == yaml.ScalarNode {
					if first, exists := keys[key.Value]; exists {
						return fmt.Errorf("duplicate key %q at lines %d and %d", key.Value, first, key.Line)
					}
					keys[key.Value] = key.Line
				}
			}
		}
		for _, child := range node.Content {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(&document)
}

type invalidYAMLValue struct {
	path []string
	line int
}

func applyConfigYAML(cfg *ApplicationConfig, data []byte, source string) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	invalid := make([]invalidYAMLValue, 0)
	var sanitize func(*yaml.Node, []string)
	sanitize = func(node *yaml.Node, parent []string) {
		if node.Kind == yaml.MappingNode {
			counts := make(map[string]int, len(node.Content)/2)
			for index := 0; index+1 < len(node.Content); index += 2 {
				if node.Content[index].Kind == yaml.ScalarNode {
					counts[node.Content[index].Value]++
				}
			}
			filtered := make([]*yaml.Node, 0, len(node.Content))
			for index := 0; index+1 < len(node.Content); index += 2 {
				key, value := node.Content[index], node.Content[index+1]
				if key.Kind != yaml.ScalarNode {
					filtered = append(filtered, key, value)
					continue
				}
				path := append(append([]string(nil), parent...), key.Value)
				if counts[key.Value] > 1 {
					if counts[key.Value] == 2 {
						invalid = append(invalid, invalidYAMLValue{path: path, line: key.Line})
					}
					continue
				}
				if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
					invalid = append(invalid, invalidYAMLValue{path: path, line: value.Line})
					continue
				}
				sanitize(value, path)
				filtered = append(filtered, key, value)
			}
			node.Content = filtered
			return
		}
		if node.Kind == yaml.SequenceNode {
			for _, child := range node.Content {
				if child.Kind == yaml.ScalarNode && child.Tag == "!!null" {
					invalid = append(invalid, invalidYAMLValue{path: parent, line: child.Line})
					continue
				}
				sanitize(child, parent)
			}
		}
	}
	if len(document.Content) > 0 {
		sanitize(document.Content[0], nil)
	}
	cleaned, err := yaml.Marshal(&document)
	if err != nil {
		return err
	}
	var incoming map[string]interface{}
	if err := yaml.Unmarshal(cleaned, &incoming); err != nil {
		return err
	}
	currentBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	var currentMap map[string]interface{}
	if err := yaml.Unmarshal(currentBytes, &currentMap); err != nil {
		return err
	}
	defaultBytes, err := yaml.Marshal(DefaultApplicationConfig())
	if err != nil {
		return err
	}
	var defaultMap map[string]interface{}
	if err := yaml.Unmarshal(defaultBytes, &defaultMap); err != nil {
		return err
	}
	merged := mergeConfigMaps(currentMap, incoming)
	for _, bad := range invalid {
		path := bad.path
		if value, exists := getConfigPath(defaultMap, path...); exists {
			setConfigPath(merged, value, path...)
		} else {
			deleteConfigPath(merged, path...)
		}
		log.Printf("Warning: invalid config value file=%s line=%d key=%s; using default", source, bad.line, strings.Join(path, "."))
	}
	for attempts := 0; attempts < 32; attempts++ {
		result, err := yaml.Marshal(merged)
		if err != nil {
			return err
		}
		candidate := previousConfigOrDefaults(cfg)
		if err := yaml.Unmarshal(result, candidate); err == nil {
			*cfg = *candidate
			return nil
		} else {
			var typeError *yaml.TypeError
			if !errors.As(err, &typeError) || len(typeError.Errors) == 0 {
				return err
			}
			line := yamlErrorLine(typeError.Errors[0])
			var resultDocument yaml.Node
			if parseErr := yaml.Unmarshal(result, &resultDocument); parseErr != nil {
				return err
			}
			path := yamlPathAtLine(resultDocument.Content, line, nil)
			if len(path) == 0 {
				return err
			}
			if value, exists := getConfigPath(defaultMap, path...); exists {
				setConfigPath(merged, value, path...)
			} else {
				deleteConfigPath(merged, path...)
			}
			sourceLine := yamlPathLine(&document, path)
			if sourceLine == 0 {
				sourceLine = line
			}
			log.Printf("Warning: invalid config value file=%s line=%d key=%s; using default", source, sourceLine, strings.Join(path, "."))
		}
	}
	return fmt.Errorf("too many invalid configuration values in %s", source)
}

func yamlErrorLine(value string) int {
	var line int
	_, _ = fmt.Sscanf(value, "line %d:", &line)
	return line
}

func yamlPathAtLine(nodes []*yaml.Node, line int, parent []string) []string {
	for _, node := range nodes {
		if node.Kind == yaml.MappingNode {
			for index := 0; index+1 < len(node.Content); index += 2 {
				key, value := node.Content[index], node.Content[index+1]
				path := append(append([]string(nil), parent...), key.Value)
				if key.Line == line || value.Line == line {
					return path
				}
				if found := yamlPathAtLine([]*yaml.Node{value}, line, path); len(found) != 0 {
					return found
				}
			}
		} else if node.Kind == yaml.SequenceNode {
			for _, child := range node.Content {
				if found := yamlPathAtLine([]*yaml.Node{child}, line, parent); len(found) != 0 {
					return found
				}
				if child.Line == line {
					return parent
				}
			}
		} else if node.Line == line {
			return parent
		}
	}
	return nil
}

func yamlPathLine(document *yaml.Node, path []string) int {
	if document == nil || len(document.Content) == 0 || len(path) == 0 {
		return 0
	}
	var find func(*yaml.Node, []string) int
	find = func(node *yaml.Node, remaining []string) int {
		if node.Kind != yaml.MappingNode || len(remaining) == 0 {
			return 0
		}
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if key.Value != remaining[0] {
				continue
			}
			if len(remaining) == 1 {
				return value.Line
			}
			return find(value, remaining[1:])
		}
		return 0
	}
	return find(document.Content[0], path)
}

func deleteConfigPath(root map[string]interface{}, path ...string) {
	if len(path) == 0 {
		return
	}
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return
		}
		current = next
	}
	delete(current, path[len(path)-1])
}

func loadBrandingDropIn(cfg *ApplicationConfig, configDir string) error {
	dropInDir := filepath.Join(configDir, "yaml.d")
	dirInfo, err := os.Lstat(dropInDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		log.Printf("Warning: ignoring config drop-ins because yaml.d cannot be inspected: %v", err)
		return nil
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		log.Printf("Warning: ignoring config drop-ins because yaml.d is not a real directory")
		return nil
	}
	if err := os.Chmod(dropInDir, 0700); err != nil {
		log.Printf("Warning: ignoring config drop-ins because yaml.d permissions could not be secured: %v", err)
		return nil
	}
	entries, err := os.ReadDir(dropInDir)
	if err != nil {
		log.Printf("Warning: ignoring config drop-ins because yaml.d cannot be read: %v", err)
		return nil
	}
	resolvedDir, err := filepath.EvalSymlinks(dropInDir)
	if err != nil {
		log.Printf("Warning: ignoring config drop-ins because yaml.d cannot be resolved: %v", err)
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".orig") {
			continue
		}
		path := filepath.Join(dropInDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			log.Printf("Warning: ignoring config drop-in %s: %v", path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr != nil {
				log.Printf("Warning: ignoring config drop-in %s: %v", path, resolveErr)
				continue
			}
			relative, relErr := filepath.Rel(resolvedDir, resolved)
			if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				log.Printf("Warning: ignoring config drop-in %s because its symlink escapes yaml.d", path)
				continue
			}
			path = resolved
			info, err = os.Stat(path)
			if err != nil {
				log.Printf("Warning: ignoring config drop-in %s: %v", path, err)
				continue
			}
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := os.Chmod(path, 0600); err != nil {
			log.Printf("Warning: ignoring config drop-in %s because permissions could not be secured: %v", path, err)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: ignoring config drop-in %s: %v", path, err)
			continue
		}
		candidateData, err := yaml.Marshal(cfg)
		if err != nil {
			log.Printf("Warning: ignoring config drop-in %s because current config cannot be copied: %v", path, err)
			continue
		}
		candidate := DefaultApplicationConfig()
		if err := yaml.Unmarshal(candidateData, candidate); err != nil {
			log.Printf("Warning: ignoring config drop-in %s because current config cannot be copied: %v", path, err)
			continue
		}
		if err := applyConfigYAML(candidate, data, path); err != nil {
			log.Printf("Warning: ignoring invalid config drop-in %s: %v", path, err)
			continue
		}
		*cfg = *candidate
		log.Printf("Loaded config drop-in: %s", filepath.Join(dropInDir, name))
	}
	return nil
}

func migrateLegacyBranding(serverPath string, data []byte) ([]byte, error) {
	migrated, err := migrateLegacyBrandingOnly(serverPath, data)
	if err != nil {
		return data, err
	}
	return migrateLegacyAreaSections(serverPath, migrated)
}

func migrateLegacyBrandingOnly(serverPath string, data []byte) ([]byte, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return data, nil
	}
	if root == nil {
		return data, nil
	}
	branding := map[string]interface{}{}
	if value, ok := takeConfigPath(root, "server", "branding"); ok {
		if section, valid := value.(map[string]interface{}); valid {
			branding = mergeConfigMaps(branding, section)
		}
	}
	seo := map[string]interface{}{}
	if value, ok := takeConfigPath(root, "server", "seo"); ok {
		if section, valid := value.(map[string]interface{}); valid {
			seo = mergeConfigMaps(seo, section)
		}
	}
	webUI := map[string]interface{}{}
	for _, key := range []string{"theme", "accent"} {
		if value, ok := takeConfigPath(root, "web", "ui", key); ok {
			webUI[key] = value
		}
	}
	if value, ok := takeConfigPath(branding, "logo_url"); ok {
		setConfigPathIfMissing(branding, value, "logo")
	}
	if value, ok := takeConfigPath(branding, "favicon_url"); ok {
		setConfigPathIfMissing(branding, value, "favicon")
	}
	for oldKey, newKey := range map[string]string{"logo": "logo", "favicon": "favicon"} {
		if value, ok := takeConfigPath(root, "web", "ui", oldKey); ok {
			setConfigPathIfMissing(branding, value, newKey)
		}
	}
	if value, ok := takeConfigPath(root, "web", "favicon_url"); ok {
		setConfigPathIfMissing(branding, value, "favicon")
	}
	if value, ok := takeConfigPath(root, "server", "title"); ok {
		setConfigPathIfMissing(branding, value, "title")
	}
	if value, ok := takeConfigPath(root, "server", "description"); ok {
		setConfigPathIfMissing(branding, value, "description")
	}
	if value, ok := takeConfigPath(seo, "default_title"); ok {
		setConfigPathIfMissing(branding, value, "title")
	}
	if value, ok := takeConfigPath(seo, "default_description"); ok {
		setConfigPathIfMissing(branding, value, "description")
	}
	if value, ok := takeConfigPath(branding, "primary_color"); ok {
		setConfigPathIfMissing(webUI, value, "accent")
	}
	_, _ = takeConfigPath(branding, "theme_color")
	_, _ = takeConfigPath(branding, "source_code_url")
	if value, ok := takeConfigPath(seo, "opengraph", "image"); ok {
		setConfigPathIfMissing(seo, value, "og_image")
	}
	_, _ = takeConfigPath(seo, "opengraph", "description")
	_, _ = takeConfigPath(seo, "enabled")
	_, _ = takeConfigPath(seo, "sitemap", "enabled")
	_, _ = takeConfigPath(seo, "feed", "enabled")
	if value, ok := takeConfigPath(seo, "twitter", "site"); ok {
		setConfigPathIfMissing(seo, value, "twitter_handle")
	}
	if value, ok := takeConfigPath(branding, "footer_text"); ok {
		setConfigPathIfMissing(branding, map[string]interface{}{
			"format": "html",
			"body":   "<p>" + html.EscapeString(fmt.Sprint(value)) + "</p>",
		}, "content", "footer")
	}
	if value, ok := takeConfigPath(root, "web", "footer", "custom_html"); ok {
		setConfigPathIfMissing(branding, map[string]interface{}{"format": "html", "body": value}, "content", "footer")
	}
	for _, page := range []string{"about", "privacy", "terms"} {
		if value, ok := takeConfigPath(root, "server", "pages", page, "content"); ok {
			setConfigPathIfMissing(branding, map[string]interface{}{"format": "md", "body": value}, "content", page)
		}
	}
	if len(branding) == 0 && len(seo) == 0 && len(webUI) == 0 {
		return data, nil
	}
	brandingDocument := map[string]interface{}{}
	if len(branding) > 0 {
		setConfigPath(brandingDocument, branding, "server", "branding")
	}
	if len(seo) > 0 {
		setConfigPath(brandingDocument, seo, "server", "seo")
	}
	if len(webUI) > 0 {
		setConfigPath(brandingDocument, webUI, "web", "ui")
	}
	dropInDir := filepath.Join(filepath.Dir(serverPath), "yaml.d")
	if err := os.MkdirAll(dropInDir, 0700); err != nil {
		return data, err
	}
	if err := os.Chmod(dropInDir, 0700); err != nil {
		return data, err
	}
	dropInPath := filepath.Join(dropInDir, "branding.yml")
	writePath := dropInPath
	if info, err := os.Lstat(dropInPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(dropInPath)
			resolvedDir, dirErr := filepath.EvalSymlinks(dropInDir)
			if resolveErr != nil || dirErr != nil {
				return data, fmt.Errorf("cannot safely resolve branding.yml symlink")
			}
			relative, relErr := filepath.Rel(resolvedDir, resolved)
			if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return data, fmt.Errorf("branding.yml symlink escapes yaml.d")
			}
			writePath = resolved
		} else if !info.Mode().IsRegular() {
			return data, fmt.Errorf("branding.yml is not a regular file")
		}
	} else if !os.IsNotExist(err) {
		return data, err
	}
	if existing, err := os.ReadFile(writePath); err == nil {
		var existingDocument map[string]interface{}
		if err := yaml.Unmarshal(existing, &existingDocument); err != nil {
			return data, fmt.Errorf("existing branding.yml is invalid: %w", err)
		}
		brandingDocument = mergeConfigMaps(brandingDocument, existingDocument)
	} else if !os.IsNotExist(err) {
		return data, err
	}
	brandingBytes, err := yaml.Marshal(brandingDocument)
	if err != nil {
		return data, err
	}
	backupPath := serverPath + ".bak"
	if err := writePrivateConfigFile(backupPath, data); err != nil {
		return data, err
	}
	if err := writePrivateConfigFile(writePath, brandingBytes); err != nil {
		return data, err
	}
	serverBytes, err := yaml.Marshal(root)
	if err != nil {
		return data, err
	}
	header := "# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.\n"
	serverBytes = append([]byte(header), serverBytes...)
	if err := writePrivateConfigFile(serverPath, serverBytes); err != nil {
		return data, err
	}
	log.Printf("Migrated branding settings from %s to %s", serverPath, dropInPath)
	return serverBytes, nil
}

func migrateLegacyAreaSections(serverPath string, data []byte) ([]byte, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return data, nil
	}
	areas := make(map[string]map[string]interface{})
	pull := func(filename string, path ...string) {
		value, ok := takeConfigPath(root, path...)
		if !ok {
			return
		}
		if areas[filename] == nil {
			areas[filename] = make(map[string]interface{})
		}
		setConfigPath(areas[filename], value, path...)
	}
	for _, section := range []struct {
		name string
		file string
	}{
		{name: "cors", file: "web.yml"}, {name: "csrf", file: "web.yml"},
		{name: "url_detection", file: "web.yml"}, {name: "compression", file: "web.yml"},
		{name: "healthz", file: "web.yml"}, {name: "i18n", file: "web.yml"},
		{name: "pages", file: "pages.yml"}, {name: "contact", file: "contact.yml"},
		{name: "privacy", file: "privacy.yml"}, {name: "compliance", file: "privacy.yml"},
		{name: "tracking", file: "privacy.yml"}, {name: "security", file: "security.yml"},
		{name: "rate_limit", file: "security.yml"}, {name: "geoip", file: "security.yml"},
		{name: "limits", file: "security.yml"}, {name: "tokens", file: "security.yml"},
		{name: "logs", file: "logs.yml"}, {name: "metrics", file: "metrics.yml"},
		{name: "notifications", file: "notifications.yml"}, {name: "tor", file: "tor.yml"},
		{name: "i2p", file: "i2p.yml"}, {name: "maintenance", file: "maintenance.yml"},
		{name: "scheduler", file: "maintenance.yml"}, {name: "update", file: "maintenance.yml"},
		{name: "auto_update", file: "maintenance.yml"}, {name: "backup", file: "maintenance.yml"},
		{name: "restore", file: "maintenance.yml"},
	} {
		pull(section.file, "server", section.name)
	}
	pull("security.yml", "data")
	pull("tor.yml", "tor")
	pull("branding.yml", "web", "ui")
	if webValue, ok := takeConfigPath(root, "web"); ok {
		if web, ok := webValue.(map[string]interface{}); ok && len(web) > 0 {
			if areas["web.yml"] == nil {
				areas["web.yml"] = make(map[string]interface{})
			}
			setConfigPath(areas["web.yml"], web, "web")
		}
	}
	if app, ok := root["app"].(map[string]interface{}); ok {
		delete(root, "app")
		if len(app) > 0 {
			if areas["app.yml"] == nil {
				areas["app.yml"] = make(map[string]interface{})
			}
			setConfigPath(areas["app.yml"], app, "app")
		}
	}
	for key, value := range root {
		if key == "server" || key == "web" || key == "tor" {
			continue
		}
		delete(root, key)
		if areas["app.yml"] == nil {
			areas["app.yml"] = make(map[string]interface{})
		}
		areas["app.yml"][key] = value
	}
	if server, ok := root["server"].(map[string]interface{}); ok {
		for key, value := range server {
			switch key {
			case "port", "address", "fqdn", "mode", "api_version", "baseurl", "pidfile", "daemonize", "debug", "database", "cache", "ssl", "trusted_proxies", "cluster":
				continue
			}
			delete(server, key)
			if areas["app.yml"] == nil {
				areas["app.yml"] = make(map[string]interface{})
			}
			setConfigPath(areas["app.yml"], value, "server", key)
		}
	}
	secrets := make(map[string]interface{})
	secretPaths := [][]string{
		{"server", "token"}, {"server", "security", "encryption_key"}, {"server", "security", "encryption_key_version"},
		{"server", "notifications", "email", "smtp", "password"}, {"server", "cache", "password"},
		{"server", "database", "token"}, {"server", "pages", "contact", "captcha_secret_key"},
		{"server", "ssl", "letsencrypt", "dns_credentials"}, {"data", "cve", "api_key"},
		{"server", "backup", "encryption_password"}, {"server", "metrics", "auth", "tokens", "prometheus"},
		{"server", "metrics", "auth", "tokens", "grafana"}, {"server", "metrics", "auth", "tokens", "loki"},
	}
	for _, secretPath := range secretPaths {
		if value, ok := takeConfigPath(root, secretPath...); ok {
			setConfigPath(secrets, value, secretPath...)
		}
		for _, area := range areas {
			if value, ok := takeConfigPath(area, secretPath...); ok {
				setConfigPath(secrets, value, secretPath...)
			}
		}
	}
	for filename, area := range areas {
		pruneEmptyConfigMaps(area)
		if len(area) == 0 {
			delete(areas, filename)
		}
	}
	if len(secrets) > 0 {
		secretsPath := filepath.Join(filepath.Dir(serverPath), "secrets.yml")
		if existing, err := os.ReadFile(secretsPath); err == nil {
			var existingDocument map[string]interface{}
			if err := yaml.Unmarshal(existing, &existingDocument); err != nil {
				return data, fmt.Errorf("existing secrets.yml is invalid: %w", err)
			}
			secrets = mergeConfigMaps(secrets, existingDocument)
		} else if !os.IsNotExist(err) {
			return data, err
		}
		encoded, err := yaml.Marshal(secrets)
		if err != nil {
			return data, err
		}
		areas["secrets.yml"] = map[string]interface{}{"__secrets_content__": string(encoded)}
	}
	for name, value := range root {
		if section, ok := value.(map[string]interface{}); ok && len(section) == 0 {
			delete(root, name)
		}
	}
	if len(areas) == 0 {
		return data, nil
	}
	dropInDir := filepath.Join(filepath.Dir(serverPath), "yaml.d")
	if err := os.MkdirAll(dropInDir, 0700); err != nil {
		return data, err
	}
	if err := os.Chmod(dropInDir, 0700); err != nil {
		return data, err
	}
	filenames := make([]string, 0, len(areas))
	for filename := range areas {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	outputs := make(map[string][]byte, len(areas))
	for _, filename := range filenames {
		if filename == "secrets.yml" {
			var encoded string
			if secretsArea, ok := areas[filename]; ok {
				encoded, _ = secretsArea["__secrets_content__"].(string)
			}
			if encoded != "" {
				outputs[filepath.Join(filepath.Dir(serverPath), filename)] = []byte(encoded)
			}
			continue
		}
		path := filepath.Join(dropInDir, filename)
		writePath, err := safeDropInWritePath(dropInDir, path)
		if err != nil {
			return data, err
		}
		merged := areas[filename]
		if existing, err := os.ReadFile(writePath); err == nil {
			var existingDocument map[string]interface{}
			if err := yaml.Unmarshal(existing, &existingDocument); err != nil {
				return data, fmt.Errorf("existing %s is invalid: %w", filename, err)
			}
			merged = mergeConfigMaps(merged, existingDocument)
		} else if !os.IsNotExist(err) {
			return data, err
		}
		encoded, err := yaml.Marshal(merged)
		if err != nil {
			return data, err
		}
		outputs[writePath] = encoded
	}
	backupPath := serverPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := writePrivateConfigFile(backupPath, data); err != nil {
			return data, err
		}
	}
	for path, contents := range outputs {
		if err := writePrivateConfigFile(path, contents); err != nil {
			return data, err
		}
	}
	serverBytes, err := yaml.Marshal(root)
	if err != nil {
		return data, err
	}
	header := "# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.\n"
	serverBytes = append([]byte(header), serverBytes...)
	if err := writePrivateConfigFile(serverPath, serverBytes); err != nil {
		return data, err
	}
	log.Printf("Migrated legacy config sections from %s to yaml.d", serverPath)
	return serverBytes, nil
}

func safeDropInWritePath(dropInDir, path string) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("config drop-in %s is not a regular file", path)
		}
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("cannot safely resolve config drop-in %s: %w", path, err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dropInDir)
	if err != nil {
		return "", fmt.Errorf("cannot safely resolve yaml.d: %w", err)
	}
	relative, err := filepath.Rel(resolvedDir, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config drop-in %s symlink escapes yaml.d", path)
	}
	info, err = os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("config drop-in %s does not target a regular file", path)
	}
	return resolved, nil
}

func pruneEmptyConfigMaps(document map[string]interface{}) bool {
	for key, value := range document {
		nested, ok := value.(map[string]interface{})
		if ok && pruneEmptyConfigMaps(nested) {
			delete(document, key)
		}
	}
	return len(document) == 0
}

func takeConfigPath(root map[string]interface{}, path ...string) (interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := root
	for _, key := range path[:len(path)-1] {
		value, ok := current[key]
		if !ok {
			return nil, false
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	value, ok := current[path[len(path)-1]]
	if ok {
		delete(current, path[len(path)-1])
	}
	return value, ok
}

func setConfigPath(root map[string]interface{}, value interface{}, path ...string) {
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			current[key] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func setConfigPathIfMissing(root map[string]interface{}, value interface{}, path ...string) {
	if existing, ok := getConfigPath(root, path...); !ok || existing == nil || existing == "" {
		setConfigPath(root, value, path...)
	}
}

func getConfigPath(root map[string]interface{}, path ...string) (interface{}, bool) {
	current := root
	for _, key := range path[:len(path)-1] {
		value, ok := current[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = value
	}
	value, ok := current[path[len(path)-1]]
	return value, ok
}

func mergeConfigMaps(lower, higher map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(lower)+len(higher))
	for key, value := range lower {
		merged[key] = value
	}
	for key, value := range higher {
		lowerMap, lowerOK := merged[key].(map[string]interface{})
		higherMap, higherOK := value.(map[string]interface{})
		if lowerOK && higherOK {
			merged[key] = mergeConfigMaps(lowerMap, higherMap)
		} else {
			merged[key] = value
		}
	}
	return merged
}

func applyI2PEnvOverrides(cfg *ApplicationConfig) {
	if value, ok := os.LookupEnv("I2P_ENABLED"); ok {
		cfg.Server.I2P.Enabled = strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
	}
}

// applyInitOnlyEnvOverrides applies environment values used only when the
// first server.yml is created. Existing config always takes precedence.
func applyInitOnlyEnvOverrides(cfg *ApplicationConfig) {
	if value := os.Getenv("APPLICATION_NAME"); value != "" {
		cfg.Server.Branding.Title = value
	}
	if value := os.Getenv("APPLICATION_TAGLINE"); value != "" {
		cfg.Server.Branding.Tagline = value
	}
}

// applyDatabaseEnvOverrides applies runtime database environment variables
// after config persistence so ephemeral values never rewrite server.yml.
func applyDatabaseEnvOverrides(cfg *ApplicationConfig) {
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
func ensureEncryptionKey(cfg *ApplicationConfig) (bool, error) {
	changed := false
	if cfg.Server.Security.EncryptionKey == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return false, fmt.Errorf("generate server.security.encryption_key: %w", err)
		}
		cfg.Server.Security.EncryptionKey = base64.StdEncoding.EncodeToString(key)
		changed = true
	}
	if cfg.Server.Security.EncryptionKeyVersion == 0 {
		cfg.Server.Security.EncryptionKeyVersion = 1
		changed = true
	}
	return changed, nil
}

// ensureOperatorToken generates and assigns the global operator token
// (server.token) the first time server.yml is created or loaded without one
// already set, per AI.md PART 8 "server.token". The token gates destructive
// operator-only actions (AI.md PART 21 "Restore Authorization"). Returns
// true if a token was generated, so the caller knows to persist it.
func ensureOperatorToken(cfg *ApplicationConfig) (bool, error) {
	if cfg.Server.Token != "" {
		return false, nil
	}
	token, err := randomBase62Token("adm_")
	if err != nil {
		return false, fmt.Errorf("generate server.token: %w", err)
	}
	cfg.Server.Token = token
	return true, nil
}

func ensureMetricsTokens(cfg *ApplicationConfig) (bool, error) {
	changed := false
	if cfg.Server.Metrics.Auth.Tokens.Prometheus == "" {
		token, err := randomBase62Token("mtr_")
		if err != nil {
			return changed, fmt.Errorf("generate prometheus metrics token: %w", err)
		}
		cfg.Server.Metrics.Auth.Tokens.Prometheus = token
		changed = true
	}
	if cfg.Server.Metrics.Auth.Tokens.Grafana == "" {
		token, err := randomBase62Token("mtr_")
		if err != nil {
			return changed, fmt.Errorf("generate grafana metrics token: %w", err)
		}
		cfg.Server.Metrics.Auth.Tokens.Grafana = token
		changed = true
	}
	if cfg.Server.Metrics.Auth.Tokens.Loki == "" {
		token, err := randomBase62Token("mtr_")
		if err != nil {
			return changed, fmt.Errorf("generate loki metrics token: %w", err)
		}
		cfg.Server.Metrics.Auth.Tokens.Loki = token
		changed = true
	}
	return changed, nil
}

func missingMetricsTokenPaths(cfg *ApplicationConfig) []string {
	missing := make([]string, 0, 3)
	if cfg.Server.Metrics.Auth.Tokens.Prometheus == "" {
		missing = append(missing, "server.metrics.auth.tokens.prometheus")
	}
	if cfg.Server.Metrics.Auth.Tokens.Grafana == "" {
		missing = append(missing, "server.metrics.auth.tokens.grafana")
	}
	if cfg.Server.Metrics.Auth.Tokens.Loki == "" {
		missing = append(missing, "server.metrics.auth.tokens.loki")
	}
	return missing
}

func ensureWebhookSigningSecrets(cfg *ApplicationConfig) ([]string, error) {
	generated := make([]string, 0)
	roles := []struct {
		name string
		cfg  *ContactRoleConfig
	}{
		{"admin", &cfg.Server.Contact.Admin},
		{"security", &cfg.Server.Contact.Security},
		{"abuse", &cfg.Server.Contact.Abuse},
		{"general", &cfg.Server.Contact.General},
	}
	for _, role := range roles {
		transports := make([]string, 0, len(role.cfg.Webhooks))
		for transport, endpoint := range role.cfg.Webhooks {
			if strings.HasSuffix(transport, "_secret") || strings.TrimSpace(endpoint) == "" {
				continue
			}
			transports = append(transports, transport)
		}
		sort.Strings(transports)
		for _, transport := range transports {
			secretKey := transport + "_secret"
			if strings.TrimSpace(role.cfg.Webhooks[secretKey]) != "" {
				continue
			}
			secretBytes := make([]byte, 32)
			if _, err := rand.Read(secretBytes); err != nil {
				return nil, fmt.Errorf("generate server.contact.%s.webhooks.%s: %w", role.name, secretKey, err)
			}
			role.cfg.Webhooks[secretKey] = base64.RawURLEncoding.EncodeToString(secretBytes)
			generated = append(generated, "server.contact."+role.name+".webhooks."+secretKey)
		}
	}
	return generated, nil
}

func randomBase62Token(prefix string) (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	out := make([]byte, 0, 32)
	buf := make([]byte, 1)
	for len(out) < 32 {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if buf[0] >= 248 {
			continue
		}
		out = append(out, alphabet[buf[0]%62])
	}
	return prefix + string(out), nil
}

// applySMTPEnvOverrides applies SMTP_* environment variable overrides on
// top of whatever was loaded from server.yml, per AI.md PART 17
// "Environment Variable Priority" - useful for containers. Only variables
// that are actually set override the config value.
func applySMTPEnvOverrides(cfg *ApplicationConfig) {
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
func applyCacheEnvOverrides(cfg *ApplicationConfig) {
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
func GetApplicationConfig() *ApplicationConfig {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return DefaultApplicationConfig()
	}
	return current
}

// Sanitized returns a copy of the config with sensitive values redacted,
// safe to expose via /debug/config. Only debug-gated callers should use this.
func GetSanitizedApplicationConfig() *ApplicationConfig {
	cfg := *GetApplicationConfig()
	if cfg.Server.SSL.LetsEncrypt.DNSCredentials != "" {
		cfg.Server.SSL.LetsEncrypt.DNSCredentials = redactedValue
	}
	if cfg.Server.Security.EncryptionKey != "" {
		cfg.Server.Security.EncryptionKey = redactedValue
	}
	if cfg.Server.Database.URL != "" {
		cfg.Server.Database.URL = redactedValue
	}
	if cfg.Server.Database.Token != "" {
		cfg.Server.Database.Token = redactedValue
	}
	if cfg.Server.Cache.URL != "" {
		cfg.Server.Cache.URL = redactedValue
	}
	if cfg.Server.Cache.Password != "" {
		cfg.Server.Cache.Password = redactedValue
	}
	if cfg.Server.Backup.EncryptionPassword != "" {
		cfg.Server.Backup.EncryptionPassword = redactedValue
	}
	if cfg.Data.CVE.APIKey != "" {
		cfg.Data.CVE.APIKey = redactedValue
	}
	if cfg.Server.CVE.APIKey != "" {
		cfg.Server.CVE.APIKey = redactedValue
	}
	if cfg.Server.Notifications.Email.SMTP.Password != "" {
		cfg.Server.Notifications.Email.SMTP.Password = redactedValue
	}
	if cfg.Server.Token != "" {
		cfg.Server.Token = redactedValue
	}
	if cfg.Server.Pages.Contact.CaptchaSecretKey != "" {
		cfg.Server.Pages.Contact.CaptchaSecretKey = redactedValue
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
func SaveApplicationConfig(path string, cfg *ApplicationConfig) error {
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
func SaveCurrentApplicationConfig() error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil || configPath == "" {
		return fmt.Errorf("no configuration loaded")
	}
	return saveConfig(current, configPath)
}

// Update updates the configuration and saves it
func UpdateApplicationConfig(cfg *ApplicationConfig) error {
	mu.Lock()
	defer mu.Unlock()
	current = cfg
	if configPath != "" {
		return saveConfig(cfg, configPath)
	}
	return nil
}

// saveConfig writes configuration to a YAML file with comments
func saveConfig(cfg *ApplicationConfig, path string) error {
	if cfg.Data.CVE.APIKey != "" {
		cfg.Server.CVE = cfg.Data.CVE
	} else if cfg.Server.CVE.APIKey != "" {
		cfg.Data.CVE = cfg.Server.CVE
	}
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := secureConfigDirectory(dir); err != nil {
		return err
	}
	if err := saveSecrets(filepath.Join(dir, "secrets.yml"), cfg); err != nil {
		return fmt.Errorf("failed to save secrets: %w", err)
	}
	if err := saveAreaConfigs(dir, cfg); err != nil {
		return fmt.Errorf("failed to save area configuration: %w", err)
	}

	// Generate YAML with comments
	content := generateConfigYAML(cfg)

	if err := writePrivateConfigFile(path, []byte(content)); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func saveInitialApplicationConfig(cfg *ApplicationConfig, path string) error {
	dir := filepath.Dir(path)
	if err := secureConfigDirectory(dir); err != nil {
		return err
	}
	if err := saveSecrets(filepath.Join(dir, "secrets.yml"), cfg); err != nil {
		return fmt.Errorf("failed to save secrets: %w", err)
	}
	return writePrivateConfigFile(path, []byte(generateInitialConfigYAML(cfg)))
}

func saveAreaConfigs(configDir string, cfg *ApplicationConfig) error {
	areas, err := collectConfigAreas(cfg)
	if err != nil {
		return err
	}
	defaultsConfig := DefaultApplicationConfig()
	defaultsYAML, err := yaml.Marshal(defaultsConfig)
	if err != nil {
		return err
	}
	var defaultsMap map[string]interface{}
	if err := yaml.Unmarshal(defaultsYAML, &defaultsMap); err != nil {
		return err
	}
	mergeConfigValues(defaultsMap, secretConfigValues(cfg))
	defaultsYAML, err = yaml.Marshal(defaultsMap)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(defaultsYAML, defaultsConfig); err != nil {
		return err
	}
	defaults, err := collectConfigAreas(defaultsConfig)
	if err != nil {
		return err
	}
	areaDir := filepath.Join(configDir, "yaml.d")
	for name, area := range areas {
		pruneEmptyConfigMaps(area)
		if len(area) == 0 {
			continue
		}
		path := filepath.Join(areaDir, name)
		_, statErr := os.Lstat(path)
		if os.IsNotExist(statErr) {
			currentBytes, marshalErr := yaml.Marshal(area)
			if marshalErr != nil {
				return marshalErr
			}
			defaultBytes, marshalErr := yaml.Marshal(defaults[name])
			if marshalErr != nil {
				return marshalErr
			}
			if string(currentBytes) == string(defaultBytes) {
				continue
			}
			if err := secureConfigDirectory(areaDir); err != nil {
				return err
			}
		} else if statErr != nil {
			return statErr
		}
		path, err = safeDropInWritePath(areaDir, path)
		if err != nil {
			return err
		}
		existingBytes, readErr := os.ReadFile(path)
		if readErr == nil {
			var existing map[string]interface{}
			if err := yaml.Unmarshal(existingBytes, &existing); err != nil {
				return fmt.Errorf("existing %s is invalid: %w", name, err)
			}
			area = mergeConfigMaps(existing, area)
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
		content, err := yaml.Marshal(area)
		if err != nil {
			return err
		}
		if err := writePrivateConfigFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

func collectConfigAreas(cfg *ApplicationConfig) (map[string]map[string]interface{}, error) {
	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(encoded, &root); err != nil {
		return nil, err
	}
	removeSecretConfigValues(root, secretConfigValues(cfg))
	areas := make(map[string]map[string]interface{})
	ensureArea := func(filename string) map[string]interface{} {
		if areas[filename] == nil {
			areas[filename] = make(map[string]interface{})
		}
		return areas[filename]
	}
	pull := func(filename string, path ...string) {
		value, ok := takeConfigPath(root, path...)
		if !ok {
			return
		}
		if areas[filename] == nil {
			areas[filename] = make(map[string]interface{})
		}
		setConfigPath(areas[filename], value, path...)
	}
	for _, section := range []struct{ name, file string }{
		{"cors", "web.yml"}, {"csrf", "web.yml"}, {"url_detection", "web.yml"},
		{"compression", "web.yml"}, {"healthz", "web.yml"}, {"i18n", "web.yml"},
		{"pages", "pages.yml"}, {"contact", "contact.yml"}, {"privacy", "privacy.yml"},
		{"compliance", "privacy.yml"}, {"tracking", "privacy.yml"}, {"security", "security.yml"},
		{"rate_limit", "security.yml"}, {"geoip", "security.yml"}, {"limits", "security.yml"},
		{"tokens", "security.yml"}, {"logs", "logs.yml"}, {"metrics", "metrics.yml"},
		{"notifications", "notifications.yml"}, {"tor", "tor.yml"}, {"i2p", "i2p.yml"},
		{"maintenance", "maintenance.yml"}, {"scheduler", "maintenance.yml"}, {"update", "maintenance.yml"},
		{"auto_update", "maintenance.yml"}, {"backup", "maintenance.yml"}, {"restore", "maintenance.yml"},
	} {
		pull(section.file, "server", section.name)
	}
	pull("security.yml", "data")
	pull("tor.yml", "tor")
	pull("branding.yml", "server", "branding")
	pull("branding.yml", "server", "seo")
	pull("branding.yml", "web", "ui")
	if webValue, ok := takeConfigPath(root, "web"); ok {
		if web, ok := webValue.(map[string]interface{}); ok && len(web) != 0 {
			if areas["web.yml"] == nil {
				areas["web.yml"] = make(map[string]interface{})
			}
			setConfigPath(areas["web.yml"], web, "web")
		}
	}
	if cli, ok := takeConfigPath(root, "cli"); ok {
		setConfigPath(ensureArea("app.yml"), cli, "cli")
	}
	if output, ok := takeConfigPath(root, "output"); ok {
		setConfigPath(ensureArea("app.yml"), output, "output")
	}
	for key, value := range root {
		if key == "server" {
			continue
		}
		setConfigPath(ensureArea("app.yml"), value, key)
	}
	if server, ok := root["server"].(map[string]interface{}); ok {
		for key, value := range server {
			if key == "port" || key == "address" || key == "fqdn" || key == "mode" || key == "api_version" || key == "baseurl" || key == "pidfile" || key == "daemonize" || key == "debug" || key == "user" || key == "group" || key == "database" || key == "cache" || key == "ssl" || key == "trusted_proxies" {
				continue
			}
			setConfigPath(ensureArea("app.yml"), value, "server", key)
		}
	}
	for name, area := range areas {
		pruneEmptyConfigMaps(area)
		if len(area) == 0 {
			delete(areas, name)
		}
	}
	return areas, nil
}

func removeSecretConfigValues(root, secrets map[string]interface{}) {
	for key, secret := range secrets {
		value, exists := root[key]
		if !exists {
			continue
		}
		if secretMap, ok := secret.(map[string]interface{}); ok {
			if valueMap, ok := value.(map[string]interface{}); ok {
				removeSecretConfigValues(valueMap, secretMap)
				if len(valueMap) == 0 {
					delete(root, key)
				}
			}
			continue
		}
		delete(root, key)
	}
}

func secureConfigDirectory(dir string) error {
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create private config directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("failed to secure config directory: %w", err)
	}
	return nil
}

// generateConfigYAML generates the node-specific server.yml content.
func generateConfigYAML(cfg *ApplicationConfig) string {
	return generateConfigYAMLWithBranding(cfg, false)
}

func generateInitialConfigYAML(cfg *ApplicationConfig) string {
	return generateConfigYAMLWithBranding(cfg, true)
}

func generateConfigYAMLWithBranding(cfg *ApplicationConfig, includeBranding bool) string {
	configCopy := *cfg
	configCopy.Server.Token = ""
	configCopy.Server.Security.EncryptionKey = ""
	configCopy.Server.Security.EncryptionKeyVersion = 0
	configCopy.Server.Database.Token = ""
	if hasCredentialURL(configCopy.Server.Database.URL) {
		configCopy.Server.Database.URL = ""
	}
	configCopy.Server.Cache.Password = ""
	if hasCredentialURL(configCopy.Server.Cache.URL) {
		configCopy.Server.Cache.URL = ""
	}
	configCopy.Server.Notifications.Email.SMTP.Password = ""
	configCopy.Server.Pages.Contact.CaptchaSecretKey = ""
	configCopy.Server.SSL.LetsEncrypt.DNSCredentials = ""
	configCopy.Server.CVE.APIKey = ""
	configCopy.Data.CVE.APIKey = ""
	configCopy.Server.Backup.EncryptionPassword = ""
	configCopy.Server.Metrics.Token = ""
	configCopy.Server.Metrics.Auth.Tokens = MetricsTokensConfig{}
	configCopy.Server.Contact.Admin.Webhooks = nil
	configCopy.Server.Contact.Security.Webhooks = nil
	configCopy.Server.Contact.Abuse.Webhooks = nil
	configCopy.Server.Contact.General.Webhooks = nil
	cfg = &configCopy
	outputColor := "  # color: false\n"
	if cfg.Output.Color != nil {
		outputColor = fmt.Sprintf("  color: %t\n", *cfg.Output.Color)
	}
	outputEmoji := "  # emoji: true\n"
	if cfg.Output.Emoji != nil {
		outputEmoji = fmt.Sprintf("  emoji: %t\n", *cfg.Output.Emoji)
	}
	content := fmt.Sprintf(`# Airports API Server Configuration
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
    default_language: "%s"
    available_languages: %s
  # Browser-session cookie defaults; max_age 0s means browser-session lifetime
  session:
    cookie_name: "%s"
    max_age: "%s"
    secure: "%s"
    http_only: %t
    same_site: "%s"
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
      disk_check:
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
    read:
      requests: %d
      window: %d
    write:
      requests: %d
      window: %d
    health:
      requests: %d
      window: %d
    strict:
      # token reveal, rotate and restore (PART 8), per IP or per owner; every attempt, valid or not, counts
      requests: %d
      window: %d
    auth_failures:
      # failed authentication (invalid, revoked or expired token), per IP; at the limit the IP is locked out until the window passes
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
    reports:
      # Closed reports are purged after this many days; open reports are retained.
      retention_days: %d
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
    # Any server.compliance standard is enabled.
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
    # Used only when GDPR is enabled; empty means residency is not configured.
    gdpr_allowed_regions: []
    # FedRAMP impact level: Low, Moderate, or High.
    fedramp_impact_level: ""

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
    disk_low:
      enabled: %t
      threshold_percent: %d
      check_interval: "%s"
      repeat: "%s"

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
    bootstrap_timeout: 300

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
    notifications:
      enabled: %t
  # Site-wide announcement banners (see PART 16 -> "Site Banner")
  announcements:
    enabled: %t
    # List of announcement messages
    messages: %s
  llms:
    enabled: %t
    include_endpoints: %t
    include_schemas: %t
    custom_sections: %s
  api_contact: %q
  # robots.txt configuration
  robots:
    # Allowed paths
    allow: %s
    # Denied paths
    deny: %s
  security:
    # Primary project security reporting URL
    report_url: %q
    # Secondary security contact email or mailto: URI
    contact: %q
    # security.txt expiration date, or {1year} for automatic renewal
    expires: %q
    # Coordinated disclosure policy body
    policy: %q
    # OpenPGP keyservers used when publishing a project key
    keyservers: %s
    publish_pgp_key: %t
    admin: "%s"
data:
  cve:
    api_key: "%s"
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
		cfg.Server.I18n.DefaultLanguage,
		formatStringSlice(cfg.Server.I18n.Supported),
		cfg.Server.Session.CookieName,
		cfg.Server.Session.MaxAge,
		cfg.Server.Session.Secure,
		cfg.Server.Session.HTTPOnly,
		cfg.Server.Session.SameSite,
		cfg.Server.Update.Branch,
		cfg.Server.Update.AutoInstall,
		cfg.Server.Update.DeferDays,
		cfg.Server.Update.CLIMinVersion,
		cfg.Server.Update.LastNotifiedVersion,
		cfg.Server.Healthz.Root.Enabled,
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
		cfg.Server.Scheduler.Tasks.DiskCheck.Enabled,
		cfg.Server.Scheduler.Tasks.DiskCheck.Schedule,
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
		cfg.Server.RateLimit.Read.Requests,
		cfg.Server.RateLimit.Read.Window,
		cfg.Server.RateLimit.Write.Requests,
		cfg.Server.RateLimit.Write.Window,
		cfg.Server.RateLimit.Health.Requests,
		cfg.Server.RateLimit.Health.Window,
		cfg.Server.RateLimit.Strict.Requests,
		cfg.Server.RateLimit.Strict.Window,
		cfg.Server.RateLimit.AuthFailures.Requests,
		cfg.Server.RateLimit.AuthFailures.Window,
		cfg.Server.RateLimit.GlobalBurst,
		cfg.Server.Database.Driver,
		cfg.Server.Database.URL,
		formatStringSlice(cfg.Server.CORS.AllowedOrigins),
		formatStringSlice(cfg.Server.TrustedProxies.Additional),
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
		cfg.Server.Security.Reports.RetentionDays,
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
		cfg.Server.Notifications.DiskLow.Enabled,
		cfg.Server.Notifications.DiskLow.ThresholdPercent,
		cfg.Server.Notifications.DiskLow.CheckInterval,
		cfg.Server.Notifications.DiskLow.Repeat,
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
		formatServerPublicSettings(cfg.Server.Contact, cfg.Server.Pages.Contact, cfg.Server.Pages.Help, cfg.Server.Tracking, cfg.Server.Privacy),
		cfg.Web.UI.Notifications.Enabled,
		cfg.Web.Announcements.Enabled,
		formatAnnouncementMessages(cfg.Web.Announcements.Messages),
		cfg.Web.LLMS.Enabled,
		cfg.Web.LLMS.IncludeEndpoints,
		cfg.Web.LLMS.IncludeSchemas,
		formatStringSlice(cfg.Web.LLMS.CustomSections),
		cfg.Web.APIContact,
		formatStringSlice(cfg.Web.Robots.Allow),
		formatStringSlice(cfg.Web.Robots.Deny),
		cfg.Web.Security.ReportURL,
		cfg.Web.Security.Contact,
		cfg.Web.Security.Expires,
		cfg.Web.Security.Policy,
		formatStringSlice(cfg.Web.Security.Keyservers),
		cfg.Web.Security.PublishPGPKey,
		cfg.Web.Security.Admin,
		cfg.Data.CVE.APIKey,
	)
	defaults := DefaultApplicationConfig()
	if includeBranding && (cfg.Server.Branding.Title != defaults.Server.Branding.Title || cfg.Server.Branding.Tagline != defaults.Server.Branding.Tagline) {
		branding := fmt.Sprintf("server:\n  branding:\n    title: %s\n    tagline: %s\n", strconv.Quote(cfg.Server.Branding.Title), strconv.Quote(cfg.Server.Branding.Tagline))
		content = strings.Replace(content, "server:\n", branding, 1)
	}
	return nodeConfigYAML(content, includeBranding)
}

func nodeConfigYAML(content string, includeBranding bool) string {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil || len(document.Content) == 0 {
		return content
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return content
	}
	serverNodeKeys := map[string]bool{
		"port": true, "fqdn": true, "address": true, "mode": true, "api_version": true,
		"baseurl": true, "debug": true, "user": true, "group": true, "pidfile": true,
		"daemonize": true, "database": true, "cache": true, "ssl": true,
		"trusted_proxies": true, "cluster": true,
	}
	if includeBranding {
		serverNodeKeys["branding"] = true
	}
	var serverNode *yaml.Node
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "server" {
			serverNode = root.Content[index+1]
			break
		}
	}
	if serverNode == nil || serverNode.Kind != yaml.MappingNode {
		return content
	}
	filteredServer := make([]*yaml.Node, 0, len(serverNode.Content))
	for index := 0; index+1 < len(serverNode.Content); index += 2 {
		key, value := serverNode.Content[index], serverNode.Content[index+1]
		if serverNodeKeys[key.Value] {
			filteredServer = append(filteredServer, key, value)
		}
	}
	serverNode.Content = filteredServer
	root.Content = []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "server"}, serverNode}
	root.HeadComment, root.FootComment, root.LineComment = "", "", ""
	document.HeadComment, document.FootComment, document.LineComment = "", "", ""
	body, err := yaml.Marshal(&document)
	if err != nil {
		return content
	}
	header := "# This node only: listen, identity, database, cache, ssl. The rest is in yaml.d/ and credentials are in secrets.yml.\n"
	return header + string(body)
}

func loadSecrets(path string, cfg *ApplicationConfig) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read secrets file: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return false, fmt.Errorf("failed to secure secrets file: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("failed to parse secrets file: %w", err)
	}
	filtered := filterSecretValues(raw, "")
	if len(filtered) == 0 {
		return false, nil
	}
	filteredYAML, err := yaml.Marshal(filtered)
	if err != nil {
		return false, fmt.Errorf("failed to filter secrets file: %w", err)
	}
	if err := yaml.Unmarshal(filteredYAML, cfg); err != nil {
		return false, fmt.Errorf("failed to load secrets file: %w", err)
	}
	return true, nil
}

func saveSecrets(path string, cfg *ApplicationConfig) error {
	secrets := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		var existing map[string]interface{}
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse existing secrets file: %w", err)
		}
		secrets = filterSecretValues(existing, "")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read existing secrets file: %w", err)
	}
	mergeConfigValues(secrets, secretConfigValues(cfg))
	if len(secrets) == 0 {
		return nil
	}
	content, err := yaml.Marshal(secrets)
	if err != nil {
		return err
	}
	return writePrivateConfigFile(path, content)
}

func mergeConfigValues(destination, source map[string]interface{}) {
	for key, value := range source {
		if sourceMap, ok := value.(map[string]interface{}); ok {
			destinationMap, ok := destination[key].(map[string]interface{})
			if !ok {
				destinationMap = make(map[string]interface{})
				destination[key] = destinationMap
			}
			mergeConfigValues(destinationMap, sourceMap)
			continue
		}
		destination[key] = value
	}
}

func secretConfigValues(cfg *ApplicationConfig) map[string]interface{} {
	secrets := make(map[string]interface{})
	values := []struct {
		path  string
		value interface{}
	}{
		{"server.token", cfg.Server.Token},
		{"server.security.encryption_key", cfg.Server.Security.EncryptionKey},
		{"server.security.encryption_key_version", cfg.Server.Security.EncryptionKeyVersion},
		{"server.notifications.email.smtp.password", cfg.Server.Notifications.Email.SMTP.Password},
		{"server.cache.password", cfg.Server.Cache.Password},
		{"server.database.token", cfg.Server.Database.Token},
		{"server.pages.contact.captcha_secret_key", cfg.Server.Pages.Contact.CaptchaSecretKey},
		{"server.ssl.letsencrypt.dns_credentials", cfg.Server.SSL.LetsEncrypt.DNSCredentials},
		{"data.cve.api_key", cfg.Data.CVE.APIKey},
		{"server.backup.encryption_password", cfg.Server.Backup.EncryptionPassword},
		{"server.metrics.auth.tokens.prometheus", cfg.Server.Metrics.Auth.Tokens.Prometheus},
		{"server.metrics.auth.tokens.grafana", cfg.Server.Metrics.Auth.Tokens.Grafana},
		{"server.metrics.auth.tokens.loki", cfg.Server.Metrics.Auth.Tokens.Loki},
	}
	for _, entry := range values {
		if valueIsSet(entry.value) {
			setDottedConfigPath(secrets, entry.path, entry.value)
		}
	}
	if hasCredentialURL(cfg.Server.Database.URL) {
		setDottedConfigPath(secrets, "server.database.url", cfg.Server.Database.URL)
	}
	if hasCredentialURL(cfg.Server.Cache.URL) {
		setDottedConfigPath(secrets, "server.cache.url", cfg.Server.Cache.URL)
	}
	for _, role := range []struct {
		name string
		cfg  ContactRoleConfig
	}{
		{"admin", cfg.Server.Contact.Admin},
		{"security", cfg.Server.Contact.Security},
		{"abuse", cfg.Server.Contact.Abuse},
		{"general", cfg.Server.Contact.General},
	} {
		for name, value := range role.cfg.Webhooks {
			if strings.TrimSpace(value) != "" {
				setDottedConfigPath(secrets, "server.contact."+role.name+".webhooks."+name, value)
			}
		}
	}
	return secrets
}

func setDottedConfigPath(root map[string]interface{}, dotted string, value interface{}) {
	parts := strings.Split(dotted, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		child, ok := current[part].(map[string]interface{})
		if !ok {
			child = make(map[string]interface{})
			current[part] = child
		}
		current = child
	}
	current[parts[len(parts)-1]] = value
}

func filterSecretValues(value interface{}, prefix string) map[string]interface{} {
	input, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	filtered := make(map[string]interface{})
	for key, child := range input {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := child.(map[string]interface{}); ok {
			if result := filterSecretValues(nested, path); len(result) > 0 {
				filtered[key] = result
			}
			continue
		}
		if secretPathAllowed(path) {
			filtered[key] = child
		}
	}
	return filtered
}

func secretPathAllowed(path string) bool {
	switch path {
	case "server.token",
		"server.security.encryption_key",
		"server.security.encryption_key_version",
		"server.notifications.email.smtp.password",
		"server.cache.password", "server.cache.url",
		"server.database.token", "server.database.url",
		"server.pages.contact.captcha_secret_key",
		"server.ssl.letsencrypt.dns_credentials",
		"data.cve.api_key",
		"server.backup.encryption_password",
		"server.metrics.auth.tokens.prometheus",
		"server.metrics.auth.tokens.grafana",
		"server.metrics.auth.tokens.loki":
		return true
	}
	return strings.HasPrefix(path, "server.contact.admin.webhooks.") ||
		strings.HasPrefix(path, "server.contact.security.webhooks.") ||
		strings.HasPrefix(path, "server.contact.abuse.webhooks.") ||
		strings.HasPrefix(path, "server.contact.general.webhooks.")
}

func valueIsSet(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case int:
		return typed != 0
	default:
		return value != nil
	}
}

func hasCredentialURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	if parsed.User != nil {
		return true
	}
	for key, values := range parsed.Query() {
		key = strings.ToLower(key)
		switch key {
		case "authtoken", "token", "password", "secret", "api_key", "apikey":
			for _, credential := range values {
				if credential != "" {
					return true
				}
			}
		}
	}
	return false
}

func writePrivateConfigFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Chmod(path, 0600)
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

func formatServerPublicSettings(cfg ContactConfig, page ContactPageConfig, help HelpPagesConfig, tracking TrackingConfig, privacy PrivacyConfig) string {
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
			if strings.HasSuffix(key, "_secret") {
				continue
			}
			if _, ok := seen[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(extra)
		for _, key := range extra {
			fmt.Fprintf(&out, "        %s: %s\n", strconv.Quote(key), strconv.Quote(role.cfg.Webhooks[key]))
		}
	}
	fmt.Fprintf(&out, "    dpo:\n      name: %s\n      email: %s\n", strconv.Quote(cfg.DPO.Name), strconv.Quote(cfg.DPO.Email))
	fmt.Fprintf(&out, "  pages:\n    contact:\n      enabled: %t\n      captcha: %s\n      captcha_site_key: %s\n      captcha_secret_key: %s\n      success_message: %s\n",
		page.Enabled, strconv.Quote(page.Captcha), strconv.Quote(page.CaptchaSiteKey), strconv.Quote(page.CaptchaSecretKey), strconv.Quote(page.SuccessMessage))
	antispam := page.Antispam
	fmt.Fprintf(&out, "      antispam:\n        honeypot: %t\n        min_fill_seconds: %d\n        max_age_seconds: %d\n        per_ip: %d\n        per_email: %d\n        global: %d\n        window: %s\n        max_links: %d\n        reject_markup: %t\n        blocked_words: %s\n        blocked_email_domains: %s\n        strikes: %d\n        ban_hours: %d\n        review_min_seconds: %d\n        review_max_seconds: %d\n        duplicate_window_hours: %d\n",
		antispam.Honeypot, antispam.MinFillSeconds, antispam.MaxAgeSeconds, antispam.PerIP,
		antispam.PerEmail, antispam.Global, strconv.Quote(antispam.Window), antispam.MaxLinks,
		antispam.RejectMarkup, formatStringSlice(antispam.BlockedWords), formatStringSlice(antispam.BlockedEmailDomains),
		antispam.Strikes, antispam.BanHours, antispam.ReviewMinSeconds, antispam.ReviewMaxSeconds, antispam.DuplicateWindowHours)
	fmt.Fprintf(&out, "    help:\n      content: %s\n      mcp: %s\n", strconv.Quote(help.Content), strconv.Quote(help.MCP))
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
	fmt.Fprintf(&out, "    consent:\n      show_until_acknowledged: %t\n      default_enabled: %t\n      message: %s\n      message_if_sold: %s\n      policy:\n        text: %s\n        url: %s\n        version: %s\n      buttons:\n        decline: %s\n        accept: %s\n      position: %s\n      show_preferences: %t\n      preferences_text: %s\n",
		consent.ShowUntilAcknowledged, consent.DefaultEnabled,
		strconv.Quote(consent.Message), strconv.Quote(consent.MessageIfSold),
		strconv.Quote(consent.Policy.Text), strconv.Quote(consent.Policy.URL), strconv.Quote(consent.Policy.Version),
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

// ReloadApplicationConfig reloads configuration from file
func ReloadApplicationConfig() error {
	if configPath == "" {
		return fmt.Errorf("no config path set")
	}
	_, err := LoadApplicationConfig(configPath)
	return err
}

// GetConfiguredServerPort returns the configured port or empty string
func GetConfiguredServerPort() string {
	cfg := GetApplicationConfig()
	return cfg.Server.Port
}

// SetConfiguredServerPort updates the port in configuration
func SetConfiguredServerPort(port string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultApplicationConfig()
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
		current = DefaultApplicationConfig()
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
		current = DefaultApplicationConfig()
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
		current = DefaultApplicationConfig()
	}
	current.Server.SSL.LastExpiryWarningDays = days
	if configPath != "" {
		return saveConfig(current, configPath)
	}
	return nil
}

// GetTheme returns the current theme
func GetConfiguredWebTheme() string {
	cfg := GetApplicationConfig()
	return cfg.Web.UI.Theme
}

// GetCORS returns the configured CORS allowed origins, defaulting to "*"
func GetConfiguredCORSOrigins() []string {
	cfg := GetApplicationConfig()
	if len(cfg.Server.CORS.AllowedOrigins) == 0 {
		return []string{"*"}
	}
	return cfg.Server.CORS.AllowedOrigins
}
