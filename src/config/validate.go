package config

import (
	"fmt"
	"log"
	"mime"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/scheduler"
	"github.com/google/uuid"
)

// validateConfig validates load-time numeric/range config values, replacing
// any invalid value with its documented default and logging a warning -
// per AI.md PART 12 "Config Validation Rule": invalid config must never fail
// startup. Fields outside a sane range are the only thing checked here;
// fields already covered by their own env-override/generation logic
// (encryption key, cache type inference) are left to those functions.
func validateConfig(cfg *Config) {
	def := DefaultConfig()
	if strings.TrimSpace(cfg.Server.Session.CookieName) == "" {
		cfg.Server.Session.CookieName = def.Server.Session.CookieName
	}
	if maxAge, err := time.ParseDuration(strings.TrimSpace(cfg.Server.Session.MaxAge)); err != nil || maxAge < 0 {
		log.Printf("warning: invalid server.session.max_age %q, using default %q", cfg.Server.Session.MaxAge, def.Server.Session.MaxAge)
		cfg.Server.Session.MaxAge = def.Server.Session.MaxAge
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Server.Session.Secure)) {
	case "auto", "true", "false":
		cfg.Server.Session.Secure = strings.ToLower(strings.TrimSpace(cfg.Server.Session.Secure))
	default:
		log.Printf("warning: invalid server.session.secure %q, using default %q", cfg.Server.Session.Secure, def.Server.Session.Secure)
		cfg.Server.Session.Secure = def.Server.Session.Secure
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Server.Session.SameSite)) {
	case "lax", "strict", "none":
		cfg.Server.Session.SameSite = strings.ToLower(strings.TrimSpace(cfg.Server.Session.SameSite))
	default:
		log.Printf("warning: invalid server.session.same_site %q, using default %q", cfg.Server.Session.SameSite, def.Server.Session.SameSite)
		cfg.Server.Session.SameSite = def.Server.Session.SameSite
	}
	if normalized, ok := NormalizeUpdateBranch(cfg.Server.Update.Branch); ok {
		cfg.Server.Update.Branch = normalized
	} else {
		log.Printf("warning: invalid server.update.branch %q, using default %q", cfg.Server.Update.Branch, def.Server.Update.Branch)
		cfg.Server.Update.Branch = def.Server.Update.Branch
	}
	if cfg.Server.Contact.Admin.Email == "" {
		log.Printf("warning: server.contact.admin.email is empty; server notifications have no email recipient")
	}
	switch cfg.Server.Pages.Contact.Captcha {
	case "simple", "recaptcha", "hcaptcha":
	default:
		log.Printf("warning: invalid server.pages.contact.captcha %q, using default %q",
			cfg.Server.Pages.Contact.Captcha, def.Server.Pages.Contact.Captcha)
		cfg.Server.Pages.Contact.Captcha = def.Server.Pages.Contact.Captcha
	}
	if cfg.Server.Pages.Contact.SuccessMessage == "" {
		cfg.Server.Pages.Contact.SuccessMessage = def.Server.Pages.Contact.SuccessMessage
	}
	if err := ValidateTracking(&cfg.Server.Tracking); err != nil {
		log.Printf("warning: invalid server.tracking configuration (%v); analytics disabled", err)
		cfg.Server.Tracking = TrackingConfig{}
	}
	if cfg.Server.Privacy.Consent.Position != "top" && cfg.Server.Privacy.Consent.Position != "bottom" {
		log.Printf("warning: invalid server.privacy.consent.position %q, using default %q",
			cfg.Server.Privacy.Consent.Position, def.Server.Privacy.Consent.Position)
		cfg.Server.Privacy.Consent.Position = def.Server.Privacy.Consent.Position
	}
	if cfg.Server.Privacy.Consent.PreferencesText == "" {
		cfg.Server.Privacy.Consent.PreferencesText = def.Server.Privacy.Consent.PreferencesText
	}
	// Essential cookies are required for security and cannot be disabled.
	cfg.Server.Privacy.Cookies.Essential.Enabled = true
	if cfg.Server.Privacy.Cookies.Essential.Description == "" {
		cfg.Server.Privacy.Cookies.Essential.Description = def.Server.Privacy.Cookies.Essential.Description
	}
	if cfg.Server.Privacy.Cookies.Preferences.Description == "" {
		cfg.Server.Privacy.Cookies.Preferences.Description = def.Server.Privacy.Cookies.Preferences.Description
	}
	if cfg.Server.Privacy.Cookies.Analytics.Description == "" {
		cfg.Server.Privacy.Cookies.Analytics.Description = def.Server.Privacy.Cookies.Analytics.Description
	}
	if cfg.Server.Privacy.Cookies.Analytics.DescriptionSuffixNotSold == "" {
		cfg.Server.Privacy.Cookies.Analytics.DescriptionSuffixNotSold = def.Server.Privacy.Cookies.Analytics.DescriptionSuffixNotSold
	}
	if cfg.Server.Privacy.Cookies.Analytics.DescriptionSuffixSold == "" {
		cfg.Server.Privacy.Cookies.Analytics.DescriptionSuffixSold = def.Server.Privacy.Cookies.Analytics.DescriptionSuffixSold
	}
	if cfg.Server.Privacy.Retention.Period == "" {
		cfg.Server.Privacy.Retention.Period = def.Server.Privacy.Retention.Period
	}
	if cfg.Server.Privacy.Content.DataCollection == "" {
		cfg.Server.Privacy.Content.DataCollection = def.Server.Privacy.Content.DataCollection
	}
	if cfg.Server.Privacy.Content.DataUsage == "" {
		cfg.Server.Privacy.Content.DataUsage = def.Server.Privacy.Content.DataUsage
	}
	if cfg.Server.Privacy.Content.DataUsageIfSold == "" {
		cfg.Server.Privacy.Content.DataUsageIfSold = def.Server.Privacy.Content.DataUsageIfSold
	}
	if cfg.Server.Privacy.Content.DataSecurity == "" {
		cfg.Server.Privacy.Content.DataSecurity = def.Server.Privacy.Content.DataSecurity
	}
	if err := cfg.Server.I2P.Validate(); err != nil {
		log.Printf("warning: invalid I2P configuration (%v), using defaults", err)
		cfg.Server.I2P = def.Server.I2P
	}
	validateServerLimits(&cfg.Server.Limits, def.Server.Limits)
	validateCompression(&cfg.Server.Compression, def.Server.Compression)
	if !validServerPort(cfg.Server.Port, cfg.Server.SSL.Enabled) {
		log.Printf("warning: invalid server.port %q, using default %q", cfg.Server.Port, def.Server.Port)
		cfg.Server.Port = def.Server.Port
	}
	if !validConfigMode(cfg.Server.Mode) {
		log.Printf("warning: invalid server.mode %q, using default %q", cfg.Server.Mode, def.Server.Mode)
		cfg.Server.Mode = def.Server.Mode
	}
	if !validListenAddress(cfg.Server.Address) {
		log.Printf("warning: invalid server.address %q, using default %q", cfg.Server.Address, def.Server.Address)
		cfg.Server.Address = def.Server.Address
	}
	if !validAPIVersion(cfg.Server.APIVersion) {
		log.Printf("warning: invalid server.api_version %q, using default %q", cfg.Server.APIVersion, def.Server.APIVersion)
		cfg.Server.APIVersion = def.Server.APIVersion
	}
	if cfg.Server.SSL.MinVersion != "TLS1.2" && cfg.Server.SSL.MinVersion != "TLS1.3" {
		log.Printf("warning: invalid server.ssl.min_version %q, using default %q", cfg.Server.SSL.MinVersion, def.Server.SSL.MinVersion)
		cfg.Server.SSL.MinVersion = def.Server.SSL.MinVersion
	}
	switch cfg.Web.UI.Theme {
	case "dark", "light", "auto":
	default:
		log.Printf("warning: invalid web.ui.theme %q, using default %q", cfg.Web.UI.Theme, def.Web.UI.Theme)
		cfg.Web.UI.Theme = def.Web.UI.Theme
	}
	switch cfg.Server.Cache.Type {
	case "none", "memory", "valkey", "redis":
	default:
		log.Printf("warning: invalid server.cache.type %q, using default %q", cfg.Server.Cache.Type, def.Server.Cache.Type)
		cfg.Server.Cache.Type = def.Server.Cache.Type
	}
	validateSchedulerSchedules(&cfg.Server.Scheduler.Tasks, def.Server.Scheduler.Tasks)

	if normalized, ok := NormalizeBaseURL(cfg.Server.BaseURL); ok {
		cfg.Server.BaseURL = normalized
	} else {
		log.Printf("warning: invalid server.baseurl %q, using default %q", cfg.Server.BaseURL, def.Server.BaseURL)
		cfg.Server.BaseURL = def.Server.BaseURL
	}

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

// NormalizeUpdateBranch maps accepted rolling-channel aliases to canonical names.
func NormalizeUpdateBranch(branch string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(branch)) {
	case "stable":
		return "stable", true
	case "beta":
		return "beta", true
	case "devel", "daily", "rolling", "dev", "development":
		return "devel", true
	default:
		return "", false
	}
}

// ValidateTracking checks supported analytics configuration without making
// invalid optional tracking prevent the application from starting.
func ValidateTracking(cfg *TrackingConfig) error {
	if cfg == nil {
		return fmt.Errorf("tracking config is nil")
	}
	cfg.Type = strings.ToLower(strings.TrimSpace(cfg.Type))
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.Type == "" || cfg.Type == "none" {
		return nil
	}
	validURL := func(raw string, required bool) error {
		if raw == "" {
			if required {
				return fmt.Errorf("%s requires a URL", cfg.Type)
			}
			return nil
		}
		if !validTrackingURL(raw) {
			return fmt.Errorf("invalid %s URL", cfg.Type)
		}
		return nil
	}
	switch cfg.Type {
	case "google":
		if !regexp.MustCompile(`^(UA-[0-9]+-[0-9]+|G-[A-Z0-9]+)$`).MatchString(cfg.ID) {
			return fmt.Errorf("google analytics ID must match UA-<digits>-<digits> or G-<alphanumeric>")
		}
	case "matomo", "piwik":
		if _, err := strconv.Atoi(cfg.ID); err != nil {
			return fmt.Errorf("%s ID must be an integer", cfg.Type)
		}
		return validURL(cfg.URL, true)
	case "owa":
		if cfg.ID == "" {
			return fmt.Errorf("OWA requires a site ID")
		}
		return validURL(cfg.URL, true)
	case "fathom":
		if cfg.ID == "" {
			return fmt.Errorf("fathom requires a site ID")
		}
		return validURL(cfg.URL, false)
	case "plausible":
		if !validTrackingDomain(cfg.ID) {
			return fmt.Errorf("plausible ID must be a valid domain")
		}
		return validURL(cfg.URL, false)
	case "umami":
		if _, err := uuid.Parse(cfg.ID); err != nil {
			return fmt.Errorf("umami ID must be a valid UUID")
		}
		return validURL(cfg.URL, true)
	case "simple":
		return nil
	case "cloudflare":
		if cfg.ID == "" {
			return fmt.Errorf("cloudflare requires a beacon token")
		}
	default:
		return fmt.Errorf("unknown analytics type %q", cfg.Type)
	}
	return nil
}

func validTrackingDomain(value string) bool {
	if value == "" || len(value) > 253 || strings.ContainsAny(value, ":/@?#") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func validTrackingURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if net.ParseIP(host) == nil && !validTrackingDomain(host) {
		return false
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return false
		}
	}
	return true
}

func validateCompression(compression *CompressionConfig, defaults CompressionConfig) {
	if compression.Level < 1 || compression.Level > 9 {
		log.Printf("warning: invalid server.compression.level %d, using default %d", compression.Level, defaults.Level)
		compression.Level = defaults.Level
	}
	types := make([]string, 0, len(compression.Types))
	for _, raw := range compression.Types {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(raw))
		if err != nil || len(params) != 0 || !strings.Contains(mediaType, "/") {
			log.Printf("warning: invalid server.compression.types entry %q, using defaults", raw)
			compression.Types = append([]string(nil), defaults.Types...)
			return
		}
		types = append(types, strings.ToLower(mediaType))
	}
	compression.Types = types
}

func validateServerLimits(limits *LimitsConfig, defaults LimitsConfig) {
	if _, err := parseConfigByteSize(limits.MaxBodySize); err != nil {
		log.Printf("warning: invalid server.limits.max_body_size %q, using default %q", limits.MaxBodySize, defaults.MaxBodySize)
		limits.MaxBodySize = defaults.MaxBodySize
	}
	for _, setting := range []struct {
		name  string
		value *string
		def   string
	}{
		{name: "read_timeout", value: &limits.ReadTimeout, def: defaults.ReadTimeout},
		{name: "write_timeout", value: &limits.WriteTimeout, def: defaults.WriteTimeout},
		{name: "idle_timeout", value: &limits.IdleTimeout, def: defaults.IdleTimeout},
	} {
		duration, err := time.ParseDuration(strings.TrimSpace(*setting.value))
		if err != nil || duration <= 0 {
			log.Printf("warning: invalid server.limits.%s %q, using default %q", setting.name, *setting.value, setting.def)
			*setting.value = setting.def
		}
	}
}

func validServerPort(value string, sslEnabled bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	parts := strings.Split(value, ",")
	if len(parts) > 2 {
		return false
	}
	ports := make([]int, len(parts))
	for i, part := range parts {
		port, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || port < 0 || port > 65535 || (len(parts) == 2 && port == 0) {
			return false
		}
		ports[i] = port
	}
	if len(ports) == 2 {
		return sslEnabled && ports[0] != ports[1]
	}
	return true
}

func validConfigMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "production", "prod", "development", "dev", "devel", "debug":
		return true
	default:
		return false
	}
}

func validListenAddress(value string) bool {
	address := strings.TrimSpace(value)
	if address == "" || strings.ContainsAny(address, " \t\r\n/\\@?#") {
		return false
	}
	host := address
	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		if !strings.HasPrefix(address, "[") || !strings.HasSuffix(address, "]") {
			return false
		}
		host = address[1 : len(address)-1]
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func validAPIVersion(value string) bool {
	if len(value) < 2 || value[0] != 'v' {
		return false
	}
	for _, r := range value[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateSchedulerSchedules(tasks *SchedulerTasks, defaults SchedulerTasks) {
	validate := func(name string, schedule *string, fallback string) {
		if err := scheduler.ValidateTaskSchedule(*schedule); err != nil {
			log.Printf("warning: invalid server.scheduler.tasks.%s.schedule %q, using default %q", name, *schedule, fallback)
			*schedule = fallback
		}
	}
	validate("ssl_renewal", &tasks.SSLRenewal.Schedule, defaults.SSLRenewal.Schedule)
	validate("geoip_update", &tasks.GeoIPUpdate.Schedule, defaults.GeoIPUpdate.Schedule)
	validate("blocklist_update", &tasks.BlocklistUpdate.Schedule, defaults.BlocklistUpdate.Schedule)
	validate("cve_update", &tasks.CVEUpdate.Schedule, defaults.CVEUpdate.Schedule)
	validate("update_check", &tasks.UpdateCheck.Schedule, defaults.UpdateCheck.Schedule)
	validate("token_cleanup", &tasks.TokenCleanup.Schedule, defaults.TokenCleanup.Schedule)
	validate("log_rotation", &tasks.LogRotation.Schedule, defaults.LogRotation.Schedule)
	validate("backup", &tasks.Backup.Schedule, defaults.Backup.Schedule)
	validate("backup_hourly", &tasks.BackupHourly.Schedule, defaults.BackupHourly.Schedule)
	validate("health_check", &tasks.HealthCheck.Schedule, defaults.HealthCheck.Schedule)
	validate("tor_health", &tasks.TorHealth.Schedule, defaults.TorHealth.Schedule)
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
