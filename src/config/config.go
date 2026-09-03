package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/apimgr/airports/src/tor"
)

// Config represents the complete server configuration
type Config struct {
	Server ServerConfig `yaml:"server"`
	Web    WebConfig    `yaml:"web"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	Port           string                    `yaml:"port"`            // Single port (HTTP) or dual (8090,64453)
	FQDN           string                    `yaml:"fqdn"`            // Fully qualified domain name
	Address        string                    `yaml:"address"`         // Listen address
	Mode           string                    `yaml:"mode"`            // Application mode: production or development
	APIVersion     string                    `yaml:"api_version"`     // API version prefix used in /api/{api_version}/ routes
	Update         UpdateConfig              `yaml:"update"`          // Update channel/auto-install/defer settings (PART 22)
	Healthz        HealthzConfig             `yaml:"healthz"`         // /server/healthz behavior
	Branding       BrandingConfig            `yaml:"branding"`        // Branding text
	SEO            SEOConfig                 `yaml:"seo"`             // SEO metadata
	User           string                    `yaml:"user"`            // System user (auto-detected if empty)
	Group          string                    `yaml:"group"`           // System group (auto-detected if empty)
	PIDFile        bool                      `yaml:"pidfile"`         // Write a PID file
	Daemonize      bool                      `yaml:"daemonize"`       // Detach from terminal on start
	SSL            SSLConfig                 `yaml:"ssl"`             // SSL/TLS configuration
	Scheduler      SchedulerConfig           `yaml:"scheduler"`       // Background task scheduling
	RateLimit      RateLimitConfig           `yaml:"rate_limit"`      // Rate limiting
	Database       DatabaseConfig            `yaml:"database"`        // Database connection
	CORS           CORSConfig                `yaml:"cors"`            // CORS settings
	TrustedProxies TrustedProxiesConfig      `yaml:"trusted_proxies"` // Additional trusted proxy IPs/CIDRs (private ranges always trusted)
	Maintenance    MaintenanceConfig         `yaml:"maintenance"`     // Self-healing maintenance mode
	GeoIP          GeoIPConfig               `yaml:"geoip"`           // GeoIP settings
	Metrics        MetricsConfig             `yaml:"metrics"`         // Metrics/observability
	Logging        LoggingConfig             `yaml:"logs"`            // Logging settings (server.logs.*)
	Privacy        PrivacyConfig             `yaml:"privacy"`         // Cookie consent, CCPA, contact addresses
	Cache          CacheConfig               `yaml:"cache"`           // Cache backend (memory/valkey/redis)
	Security       SecurityConfig            `yaml:"security"`        // IP allowlist/blocklist management
	CVE            CVEConfig                 `yaml:"cve"`             // NVD CVE feed settings
	Backup         BackupConfig              `yaml:"backup"`          // Backup encryption/retention settings
	Compliance     ComplianceConfig          `yaml:"compliance"`      // Compliance-standard enforcement
	Tor            tor.Config                `yaml:"tor"`             // Tor hidden service settings (PART 31)
	Notifications  ServerNotificationsConfig `yaml:"notifications"`   // WebUI toast + operator email settings (PART 17)
	CSRF           CSRFConfig                `yaml:"csrf"`            // Double-submit CSRF protection for browser forms (PART 16)
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
// Enabled, backups are blocked unless server.backup.encryption_password
// is set.
type ComplianceConfig struct {
	Enabled bool `yaml:"enabled"`
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
	Enabled       bool   `yaml:"enabled"`        // Enable metrics
	Endpoint      string `yaml:"endpoint"`       // Metrics endpoint
	IncludeSystem bool   `yaml:"include_system"` // Include system metrics
	IncludeApp    bool   `yaml:"include_app"`    // Include app metrics
	// Token, when non-empty, requires "Authorization: Bearer <token>" on the
	// metrics endpoint(s) per AI.md PART 20 "Authentication options". Empty
	// (default) means no token check — deployments are expected to firewall
	// /metrics instead. Metrics are internal-only either way.
	Token string `yaml:"token"`
	// DurationBuckets overrides the histogram buckets (seconds) used for
	// *_http_request_duration_seconds per AI.md PART 20 "Configuration".
	DurationBuckets []float64 `yaml:"duration_buckets"`
	// SizeBuckets overrides the histogram buckets (bytes) used for
	// *_http_request_size_bytes / *_http_response_size_bytes per AI.md
	// PART 20 "Configuration".
	SizeBuckets []float64 `yaml:"size_buckets"`
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

// PrivacyConfig controls the cookie-consent banner, /server/privacy content,
// and /server/contact addresses per AI.md PART 16 "Cookie Consent Banner"
// and "/server/privacy". Dynamic messaging switches on Data.Sold.
type PrivacyConfig struct {
	Data    DataPolicy    `yaml:"data"`    // CCPA "is data sold" toggle
	Consent ConsentConfig `yaml:"consent"` // Cookie consent banner text/links
	Contact ContactConfig `yaml:"contact"` // /server/contact display addresses
}

// DataPolicy controls CCPA "Do Not Sell" messaging.
type DataPolicy struct {
	Sold           bool `yaml:"sold"`             // Default false — MIT operators may enable
	StoredOnServer bool `yaml:"stored_on_server"` // Whether any personal data is stored server-side
}

// ConsentConfig configures the cookie consent banner per AI.md PART 16.
type ConsentConfig struct {
	Message       string `yaml:"message"`         // Shown when Data.Sold = false
	MessageIfSold string `yaml:"message_if_sold"` // Shown when Data.Sold = true
	Policy        struct {
		Text string `yaml:"text"`
		URL  string `yaml:"url"`
	} `yaml:"policy"`
	Buttons struct {
		Decline string `yaml:"decline"`
		Accept  string `yaml:"accept"`
	} `yaml:"buttons"`
	Position string `yaml:"position"` // "top" or "bottom"
}

// GetConsentMessage returns MessageIfSold when Data.Sold is true, otherwise
// Message, per AI.md PART 16 "Dynamic Message Selection".
func (p *PrivacyConfig) GetConsentMessage() string {
	if p.Data.Sold {
		return p.Consent.MessageIfSold
	}
	return p.Consent.Message
}

// ContactConfig lists the display-only contact addresses shown on
// /server/contact. Airports has no SMTP subsystem (features-rules.md: never
// send/queue email without a tested SMTP connection), so this page renders
// mailto: links rather than a submission form. Empty fields fall back to
// WebSecurityConfig.Admin (security.txt contact) where documented.
type ContactConfig struct {
	General string `yaml:"general"` // General inquiries address
	Abuse   string `yaml:"abuse"`   // Abuse reports address
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
	Branch      string `yaml:"branch"`       // Release channel: stable, beta, or daily
	AutoInstall bool   `yaml:"auto_install"` // Auto-install updates found by update_check (default off)
	DeferDays   int    `yaml:"defer_days"`   // Days a release must age before update_check adopts it

	// LastNotifiedVersion is internal state (not operator-set) recording the
	// newest version update_check has already notified about, so the
	// WARN log + update_available email fire once per version, not on
	// every scheduler run, per PART 22 "Surfacing rules".
	LastNotifiedVersion string `yaml:"last_notified_version,omitempty"`
}

var (
	current    *Config
	mu         sync.RWMutex
	configPath string
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:       "", // Will be set to random 64xxx port
			FQDN:       "",
			Address:    "[::]",
			Mode:       "production",
			APIVersion: "v1",
			Update: UpdateConfig{
				Branch:      "stable",
				AutoInstall: false,
				DeferDays:   0,
			},
			Healthz: HealthzConfig{
				Root: HealthzRootConfig{Enabled: false},
			},
			Branding: BrandingConfig{
				Title:       "airports",
				Tagline:     "",
				Description: "",
			},
			SEO:       SEOConfig{Keywords: []string{}},
			User:      "",
			Group:     "",
			PIDFile:   true,
			Daemonize: false,
			SSL: SSLConfig{
				Enabled:    false,
				Cert:       "",
				Key:        "",
				MinVersion: "TLS1.2",
				LetsEncrypt: LetsEncryptConfig{
					Enabled:   false,
					Challenge: "http-01",
					Staging:   false,
				},
			},
			Scheduler: SchedulerConfig{
				Enabled:       true,
				Timezone:      "America/New_York",
				CatchUpWindow: "1h",
				Tasks: SchedulerTasks{
					SSLRenewal:      SSLRenewalConfig{Enabled: true, Schedule: "0 3 * * *", RenewBefore: "7d"},
					GeoIPUpdate:     TaskConfig{Enabled: true, Schedule: "0 3 * * 0", RetryOnFail: true, RetryDelay: "1h"},
					BlocklistUpdate: TaskConfig{Enabled: true, Schedule: "0 4 * * *", RetryOnFail: true, RetryDelay: "1h"},
					CVEUpdate:       TaskConfig{Enabled: true, Schedule: "0 5 * * *", RetryOnFail: true, RetryDelay: "1h"},
					UpdateCheck:     TaskConfig{Enabled: true, Schedule: "0 6 * * *"},
					TokenCleanup:    TaskConfig{Enabled: true, Schedule: "*/15 * * * *"},
					LogRotation:     TaskConfig{Enabled: true, Schedule: "0 0 * * *"},
					Backup:          BackupTaskConfig{Enabled: true, Schedule: "0 2 * * *", Retention: 5},
					BackupHourly:    TaskConfig{Enabled: false, Schedule: "0 * * * *"},
					HealthCheck:     TaskConfig{Enabled: true, Schedule: "*/5 * * * *"},
					TorHealth:       TaskConfig{Enabled: true, Schedule: "*/10 * * * *", RetryOnFail: true, RetryDelay: "1m"},
				},
			},
			RateLimit: RateLimitConfig{
				Enabled:     true,
				Read:        RateLimitBucket{Requests: 120, Window: 60},
				Write:       RateLimitBucket{Requests: 10, Window: 60},
				Health:      RateLimitBucket{Requests: 120, Window: 60},
				GlobalBurst: 240,
			},
			Database: DatabaseConfig{
				Driver: "sqlite",
				URL:    "",
				Pool: DatabasePoolConfig{
					MaxOpen:     4,
					MaxIdle:     4,
					MaxLifetime: "5m",
					MaxIdleTime: "1m",
				},
			},
			CORS:           CORSConfig{AllowedOrigins: []string{"*"}},
			TrustedProxies: TrustedProxiesConfig{Additional: []string{}},
			CSRF: CSRFConfig{
				Enabled:     true,
				TokenLength: 32,
				CookieName:  "csrf_token",
				HeaderName:  "X-CSRF-Token",
				Secure:      "auto",
				// GraphQL is a JSON API surface (own CORS/GraphiQL-origin model),
				// not a browser form — exempt by default so the playground's
				// fetch() calls keep working without a hidden csrf_token field.
				ExemptPaths: []string{"/api/graphql", "/server/graphql"},
			},
			Maintenance: MaintenanceConfig{
				SelfHealing: SelfHealingConfig{Enabled: true, RetryInterval: "30s", MaxAttempts: 0},
				Cleanup:     CleanupConfig{DiskThreshold: 90, LogRetentionDays: 7, BackupKeepCount: 5},
				Notify:      MaintenanceNotify{OnEnter: true, OnExit: true},
			},
			GeoIP: GeoIPConfig{
				Enabled:        true,
				AllowCountries: []string{},
				DenyCountries:  []string{},
				Presets:        map[string][]string{},
			},
			Metrics: MetricsConfig{
				Enabled:         false,
				Endpoint:        "/metrics",
				IncludeSystem:   true,
				IncludeApp:      true,
				DurationBuckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				SizeBuckets:     []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			Logging: LoggingConfig{
				Level: "warn",
				Access: LogAccessConfig{
					LogFileConfig: LogFileConfig{
						Filename: "access.log",
						Format:   "apache",
						Custom:   "",
						Rotate:   "monthly",
						Keep:     "none",
					},
					LogHealthChecks: false,
				},
				Server: LogFileConfig{
					Filename: "server.log",
					Format:   "text",
					Custom:   "",
					Rotate:   "weekly,50MB",
					Keep:     "none",
				},
				Error: LogFileConfig{
					Filename: "error.log",
					Format:   "text",
					Custom:   "",
					Rotate:   "weekly,50MB",
					Keep:     "none",
				},
				App: LogFileConfig{
					Filename: "app.log",
					Format:   "logfmt",
					Custom:   "",
					Rotate:   "weekly,50MB",
					Keep:     "none",
				},
				Auth: LogFileConfig{
					Filename: "auth.log",
					Format:   "syslog",
					Custom:   "",
					Rotate:   "weekly,50MB",
					Keep:     "none",
				},
				Audit: LogAuditConfig{
					LogFileConfig: LogFileConfig{
						Filename: "audit.log",
						Format:   "json",
						Custom:   "",
						Rotate:   "daily",
						Keep:     "none",
					},
					Enabled:  true,
					Compress: false,
					Events: LogAuditEventsConfig{
						Configuration: true,
						Security:      true,
						Backup:        true,
						Server:        true,
					},
					MaskEmails: true,
				},
				Security: LogFileConfig{
					Filename: "security.log",
					Format:   "fail2ban",
					Custom:   "",
					Rotate:   "weekly,50MB",
					Keep:     "none",
				},
				Debug: LogDebugConfig{
					LogFileConfig: LogFileConfig{
						Filename: "debug.log",
						Format:   "text",
						Custom:   "",
						Rotate:   "weekly,50MB",
						Keep:     "none",
					},
					Enabled: false,
				},
			},
			Privacy: PrivacyConfig{
				Data: DataPolicy{
					Sold:           false,
					StoredOnServer: false,
				},
				Consent: ConsentConfig{
					Message:       "In accordance with the EU GDPR law this message is being displayed. We use cookies for essential site functionality (theme, security) and, with your consent, for preferences.",
					MessageIfSold: "In accordance with the EU GDPR law this message is being displayed. We use cookies for essential site functionality and, with your consent, for preferences and analytics. Your data may be shared with or sold to third parties as described in our Privacy Policy.",
					Position:      "bottom",
					Policy: struct {
						Text string `yaml:"text"`
						URL  string `yaml:"url"`
					}{Text: "Privacy Policy", URL: "/server/privacy"},
					Buttons: struct {
						Decline string `yaml:"decline"`
						Accept  string `yaml:"accept"`
					}{Decline: "Decline", Accept: "I Agree"},
				},
				Contact: ContactConfig{
					General: "",
					Abuse:   "",
				},
			},
			Cache: CacheConfig{
				Type:     "memory",
				Host:     "localhost",
				Port:     6379,
				DB:       0,
				PoolSize: 10,
				MinIdle:  2,
				Timeout:  "5s",
				Prefix:   "airports:",
				TTL:      "1h",
			},
			Security: SecurityConfig{
				Allowlist:  []AllowlistEntry{},
				BlockedIPs: []BlockedIPEntry{},
				AbuseDetection: AbuseDetectionConfig{
					RequestFlood: RequestFloodConfig{
						Multiplier:    10,
						BlockDuration: "1h",
					},
					AutoBlockIP: true,
					AutoAlert:   true,
				},
			},
			CVE: CVEConfig{
				APIKey: "",
			},
			Backup: BackupConfig{
				EncryptionPassword: "",
				Retention: BackupRetention{
					MaxBackups:   1,
					KeepWeekly:   0,
					KeepMonthly:  0,
					KeepYearly:   0,
					MaxTotalSize: "10%",
				},
			},
			Compliance: ComplianceConfig{
				Enabled: false,
			},
			Tor: tor.DefaultConfig(),
			Notifications: ServerNotificationsConfig{
				WebUI: NotifyWebUIConfig{
					Position: "top-right",
					Duration: 5,
				},
				Email: NotifyEmailConfig{
					Enabled: false,
					SMTP: NotifySMTPConfig{
						Host:     "",
						Port:     587,
						Username: "",
						Password: "",
						TLS:      "auto",
					},
					From: NotifyFromConfig{
						Name:  "",
						Email: "",
					},
					ReplyTo: "",
					Events: map[string]bool{
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
					},
				},
			},
		},
		Web: WebConfig{
			UI: WebUIConfig{
				Theme:   "dark",
				Logo:    "",
				Favicon: "",
				Notifications: NotificationsConfig{
					Enabled: true,
				},
			},
			Announcements: AnnouncementsConfig{
				Enabled:  true,
				Messages: []AnnouncementMessage{},
			},
			Robots: WebRobotsConfig{
				Allow: []string{"/", "/api"},
				Deny:  []string{"/admin", "/debug"},
				AIBots: WebRobotsAIBotsConfig{
					Default: "allow",
					Bots:    map[string]string{},
				},
			},
			Security: WebSecurityConfig{
				Admin: "",
			},
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
		applySMTPEnvOverrides(cfg)
		applyCacheEnvOverrides(cfg)
		ensureEncryptionKey(cfg)
		validateConfig(cfg)
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

	applySMTPEnvOverrides(cfg)
	applyCacheEnvOverrides(cfg)
	validateConfig(cfg)

	if ensureEncryptionKey(cfg) {
		if err := saveConfig(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to persist generated encryption key: %w", err)
		}
	}

	current = cfg
	return cfg, nil
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

// validateConfig validates load-time numeric/range config values, replacing
// any invalid value with its documented default and logging a warning -
// per AI.md PART 12 "Config Validation Rule": invalid config must never fail
// startup. Fields outside a sane range are the only thing checked here;
// fields already covered by their own env-override/generation logic
// (encryption key, cache type inference) are left to those functions.
func validateConfig(cfg *Config) {
	def := DefaultConfig()

	validateRateLimitBucket("read", &cfg.Server.RateLimit.Read, def.Server.RateLimit.Read)
	validateRateLimitBucket("write", &cfg.Server.RateLimit.Write, def.Server.RateLimit.Write)
	validateRateLimitBucket("health", &cfg.Server.RateLimit.Health, def.Server.RateLimit.Health)

	if cfg.Server.RateLimit.GlobalBurst <= 0 {
		log.Printf("warning: invalid server.rate_limit.global_burst %d, using default %d",
			cfg.Server.RateLimit.GlobalBurst, def.Server.RateLimit.GlobalBurst)
		cfg.Server.RateLimit.GlobalBurst = def.Server.RateLimit.GlobalBurst
	}

	if cfg.Server.Database.Pool.MaxOpen <= 0 {
		log.Printf("warning: invalid server.database.pool.max_open %d, using default %d",
			cfg.Server.Database.Pool.MaxOpen, def.Server.Database.Pool.MaxOpen)
		cfg.Server.Database.Pool.MaxOpen = def.Server.Database.Pool.MaxOpen
	}
	if cfg.Server.Database.Pool.MaxIdle < 0 || cfg.Server.Database.Pool.MaxIdle > cfg.Server.Database.Pool.MaxOpen {
		log.Printf("warning: invalid server.database.pool.max_idle %d, using default %d",
			cfg.Server.Database.Pool.MaxIdle, def.Server.Database.Pool.MaxIdle)
		cfg.Server.Database.Pool.MaxIdle = def.Server.Database.Pool.MaxIdle
	}

	if cfg.Server.Cache.PoolSize <= 0 {
		log.Printf("warning: invalid server.cache.pool_size %d, using default %d",
			cfg.Server.Cache.PoolSize, def.Server.Cache.PoolSize)
		cfg.Server.Cache.PoolSize = def.Server.Cache.PoolSize
	}
	if cfg.Server.Cache.MinIdle < 0 || cfg.Server.Cache.MinIdle > cfg.Server.Cache.PoolSize {
		log.Printf("warning: invalid server.cache.min_idle %d, using default %d",
			cfg.Server.Cache.MinIdle, def.Server.Cache.MinIdle)
		cfg.Server.Cache.MinIdle = def.Server.Cache.MinIdle
	}

	if cfg.Server.Maintenance.Cleanup.DiskThreshold < 1 || cfg.Server.Maintenance.Cleanup.DiskThreshold > 100 {
		log.Printf("warning: invalid server.maintenance.cleanup.disk_threshold %d, using default %d",
			cfg.Server.Maintenance.Cleanup.DiskThreshold, def.Server.Maintenance.Cleanup.DiskThreshold)
		cfg.Server.Maintenance.Cleanup.DiskThreshold = def.Server.Maintenance.Cleanup.DiskThreshold
	}
	if cfg.Server.Maintenance.Cleanup.LogRetentionDays <= 0 {
		log.Printf("warning: invalid server.maintenance.cleanup.log_retention_days %d, using default %d",
			cfg.Server.Maintenance.Cleanup.LogRetentionDays, def.Server.Maintenance.Cleanup.LogRetentionDays)
		cfg.Server.Maintenance.Cleanup.LogRetentionDays = def.Server.Maintenance.Cleanup.LogRetentionDays
	}
	if cfg.Server.Maintenance.Cleanup.BackupKeepCount < 0 {
		log.Printf("warning: invalid server.maintenance.cleanup.backup_keep_count %d, using default %d",
			cfg.Server.Maintenance.Cleanup.BackupKeepCount, def.Server.Maintenance.Cleanup.BackupKeepCount)
		cfg.Server.Maintenance.Cleanup.BackupKeepCount = def.Server.Maintenance.Cleanup.BackupKeepCount
	}

	for i := range cfg.Web.Announcements.Messages {
		msg := &cfg.Web.Announcements.Messages[i]
		switch msg.Type {
		case "info", "warning", "error", "success":
			// valid
		default:
			log.Printf("warning: invalid web.announcements.messages[%d].type %q, using default %q",
				i, msg.Type, "info")
			msg.Type = "info"
		}
	}

	switch cfg.Web.Robots.AIBots.Default {
	case "allow", "deny":
		// valid
	case "":
		cfg.Web.Robots.AIBots.Default = def.Web.Robots.AIBots.Default
	default:
		log.Printf("warning: invalid web.robots.ai_bots.default %q, using default %q",
			cfg.Web.Robots.AIBots.Default, def.Web.Robots.AIBots.Default)
		cfg.Web.Robots.AIBots.Default = def.Web.Robots.AIBots.Default
	}
	for bot, access := range cfg.Web.Robots.AIBots.Bots {
		switch access {
		case "allow", "deny", "":
			// valid; an empty value inherits ai_bots.default
		default:
			log.Printf("warning: invalid web.robots.ai_bots.bots.%s %q, inheriting ai_bots.default",
				bot, access)
			cfg.Web.Robots.AIBots.Bots[bot] = ""
		}
	}
}

// validateRateLimitBucket replaces an invalid (non-positive) requests/window
// pair with the documented default for that bucket, warning via log.
func validateRateLimitBucket(name string, bucket *RateLimitBucket, def RateLimitBucket) {
	if bucket.Requests <= 0 {
		log.Printf("warning: invalid server.rate_limit.%s.requests %d, using default %d",
			name, bucket.Requests, def.Requests)
		bucket.Requests = def.Requests
	}
	if bucket.Window <= 0 {
		log.Printf("warning: invalid server.rate_limit.%s.window %d, using default %d",
			name, bucket.Window, def.Window)
		bucket.Window = def.Window
	}
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
	return &cfg
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
	return fmt.Sprintf(`# Airports API Server Configuration
# Documentation: https://airports.apimgr.us/docs

server:
  # Single port (HTTP) or dual port (8090,64453 - second is HTTPS)
  port: "%s"
  # Fully qualified domain name
  fqdn: "%s"
  # Listen address ([::] for all interfaces IPv4/IPv6)
  address: "%s"
  # Application mode: production or development
  mode: "%s"
  # API version prefix used in /api/{api_version}/ routes
  api_version: "%s"
  update:
    # Release channel: stable | beta | daily (also settable via --update branch)
    branch: "%s"
    # Auto-install updates found by the update_check task (default off)
    auto_install: %t
    # Defer window in days (0-365): a release is only eligible once this old
    defer_days: %d
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
    # Prometheus-compatible metrics endpoint
    endpoint: "%s"
    # Include CPU, memory, disk metrics
    include_system: %t
    # Include request, error, latency metrics
    include_app: %t
    # Optional bearer token required as "Authorization: Bearer <token>" to
    # read /metrics. Empty = no token check (rely on firewalling instead).
    token: "%s"
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

  # Compliance mode (PART 11/21): when enabled, backups are blocked unless
  # server.backup.encryption_password is set
  compliance:
    enabled: %t

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

  # Tor hidden service (PART 31): auto-enabled if a tor binary is found.
  # Leave "enabled" unset (omitted) for auto-detection; set true/false to
  # force it. Never use default Tor ports - this app always runs its own
  # isolated Tor process with an auto-assigned control port.
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
		cfg.Server.Port,
		cfg.Server.FQDN,
		cfg.Server.Address,
		cfg.Server.Mode,
		cfg.Server.APIVersion,
		cfg.Server.Update.Branch,
		cfg.Server.Update.AutoInstall,
		cfg.Server.Update.DeferDays,
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
		cfg.Server.Metrics.Endpoint,
		cfg.Server.Metrics.IncludeSystem,
		cfg.Server.Metrics.IncludeApp,
		cfg.Server.Metrics.Token,
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
		cfg.Server.Compliance.Enabled,
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
		b.WriteString(fmt.Sprintf("\n      - cidr: %q\n        description: %q", e.CIDR, e.Description))
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
		b.WriteString(fmt.Sprintf("\n      - cidr: %q\n        reason: %q", e.CIDR, e.Reason))
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
		b.WriteString(fmt.Sprintf(
			"\n      - id: %q\n        type: %q\n        title: %q\n        message: %q\n        start: %q\n        end: %q\n        dismissible: %t",
			m.ID, m.Type, m.Title, m.Message, m.Start, m.End, m.Dismissible))
	}
	return b.String()
}

// formatNotificationEvents renders server.notifications.email.events as a
// YAML block mapping in the fixed key order defined by
// notificationEventOrder, since Go map iteration order is non-deterministic.
func formatNotificationEvents(events map[string]bool) string {
	var b strings.Builder
	for _, key := range notificationEventOrder {
		b.WriteString(fmt.Sprintf("\n        %s: %t", key, events[key]))
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
		b.WriteString(fmt.Sprintf("\n      %s: %s", name, formatStringSlice(presets[name])))
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
		b.WriteString(fmt.Sprintf("\n        %s: %q", name, bots[name]))
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

// SetUpdateBranch updates the update channel (stable, beta, daily) in configuration
func SetUpdateBranch(branch string) error {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = DefaultConfig()
	}
	current.Server.Update.Branch = branch
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
