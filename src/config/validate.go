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

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/google/uuid"
)

// validateConfig validates configuration and returns errors for invalid
// combinations that cannot be safely repaired with defaults.
func validateConfig(cfg *ApplicationConfig) {
	def := DefaultApplicationConfig()
	validateCompliance(&cfg.Server.Compliance)
	cfg.Server.I18n.Supported = i18n.NormalizeAvailableLanguages(cfg.Server.I18n.Supported)
	defaultLanguage := strings.ToLower(strings.TrimSpace(cfg.Server.I18n.DefaultLanguage))
	defaultEnabled := false
	for _, code := range cfg.Server.I18n.Supported {
		if code == defaultLanguage {
			defaultEnabled = true
			break
		}
	}
	if !defaultEnabled {
		cfg.Server.I18n.DefaultLanguage = def.Server.I18n.DefaultLanguage
	} else {
		cfg.Server.I18n.DefaultLanguage = defaultLanguage
	}
	validateSEO(&cfg.Server.SEO, def.Server.SEO)
	captcha := strings.ToLower(strings.TrimSpace(cfg.Server.Pages.Contact.Captcha))
	cfg.Server.Pages.Contact.Captcha = captcha
	if captcha == "recaptcha" || captcha == "hcaptcha" {
		missingKeys := make([]string, 0, 2)
		if strings.TrimSpace(cfg.Server.Pages.Contact.CaptchaSiteKey) == "" {
			missingKeys = append(missingKeys, "server.pages.contact.captcha_site_key")
		}
		if strings.TrimSpace(cfg.Server.Pages.Contact.CaptchaSecretKey) == "" {
			missingKeys = append(missingKeys, "server.pages.contact.captcha_secret_key")
		}
		if len(missingKeys) > 0 {
			log.Printf("warning: server.pages.contact.captcha=%q requires %s; using default %q", captcha, strings.Join(missingKeys, " and "), def.Server.Pages.Contact.Captcha)
			cfg.Server.Pages.Contact.Captcha = def.Server.Pages.Contact.Captcha
		}
	}
	validateContactAntispam(&cfg.Server.Pages.Contact.Antispam, def.Server.Pages.Contact.Antispam)
	if strings.TrimSpace(cfg.Server.Session.CookieName) == "" {
		cfg.Server.Session.CookieName = def.Server.Session.CookieName
	}
	if maxAge, err := time.ParseDuration(strings.TrimSpace(cfg.Server.Session.MaxAge)); err != nil || maxAge < 0 {
		log.Printf("warning: invalid server.session.max_age %q, using default %q", cfg.Server.Session.MaxAge, def.Server.Session.MaxAge)
		cfg.Server.Session.MaxAge = def.Server.Session.MaxAge
	}
	if cfg.Server.Tokens.MaxLinked < 0 {
		log.Printf("warning: invalid server.tokens.max_linked %d, using default %d", cfg.Server.Tokens.MaxLinked, def.Server.Tokens.MaxLinked)
		cfg.Server.Tokens.MaxLinked = def.Server.Tokens.MaxLinked
	}
	if cfg.Server.Tokens.IdlePurgeDays < 0 {
		log.Printf("warning: invalid server.tokens.idle_purge_days %d, using default %d", cfg.Server.Tokens.IdlePurgeDays, def.Server.Tokens.IdlePurgeDays)
		cfg.Server.Tokens.IdlePurgeDays = def.Server.Tokens.IdlePurgeDays
	}
	if _, err := parseTokenCookieAge(cfg.Server.Tokens.CookieMaxAge); err != nil {
		log.Printf("warning: invalid server.tokens.cookie_max_age %q, using default %q", cfg.Server.Tokens.CookieMaxAge, def.Server.Tokens.CookieMaxAge)
		cfg.Server.Tokens.CookieMaxAge = def.Server.Tokens.CookieMaxAge
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
	if err := cfg.Server.I2P.ValidateI2PConfig(); err != nil {
		log.Printf("warning: invalid I2P configuration (%v), using defaults", err)
		cfg.Server.I2P = def.Server.I2P
	}
	validateServerLimits(&cfg.Server.Limits, def.Server.Limits)
	validateRestoreConfig(&cfg.Server.Restore, def.Server.Restore)
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
	if accent := strings.TrimSpace(cfg.Web.UI.Accent); accent != "" && !accentColorValidationPattern.MatchString(accent) {
		log.Printf("warning: invalid web.ui.accent %q, using the built-in accent", accent)
		cfg.Web.UI.Accent = ""
	} else {
		cfg.Web.UI.Accent = accent
	}
	switch cfg.Server.Cache.Type {
	case "none", "memory", "valkey", "redis":
	default:
		log.Printf("warning: invalid server.cache.type %q, using default %q", cfg.Server.Cache.Type, def.Server.Cache.Type)
		cfg.Server.Cache.Type = def.Server.Cache.Type
	}
	validateSchedulerSchedules(&cfg.Server.Scheduler.Tasks, def.Server.Scheduler.Tasks)
	validateDiskLowConfig(&cfg.Server.Notifications.DiskLow, def.Server.Notifications.DiskLow)
	validateDatabaseErrorConfig(&cfg.Server.Notifications.DatabaseError, def.Server.Notifications.DatabaseError)

	if normalized, ok := NormalizeBaseURL(cfg.Server.BaseURL); ok {
		cfg.Server.BaseURL = normalized
	} else {
		log.Printf("warning: invalid server.baseurl %q, using default %q", cfg.Server.BaseURL, def.Server.BaseURL)
		cfg.Server.BaseURL = def.Server.BaseURL
	}

	validateRateLimitBucket("read", &cfg.Server.RateLimit.Read, def.Server.RateLimit.Read)
	validateRateLimitBucket("write", &cfg.Server.RateLimit.Write, def.Server.RateLimit.Write)
	validateRateLimitBucket("health", &cfg.Server.RateLimit.Health, def.Server.RateLimit.Health)
	validateRateLimitBucket("strict", &cfg.Server.RateLimit.Strict, def.Server.RateLimit.Strict)
	validateRateLimitBucket("auth_failures", &cfg.Server.RateLimit.AuthFailures, def.Server.RateLimit.AuthFailures)

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
	if cfg.Server.Security.Reports.RetentionDays <= 0 {
		log.Printf("warning: invalid server.security.reports.retention_days %d, using default %d",
			cfg.Server.Security.Reports.RetentionDays, def.Server.Security.Reports.RetentionDays)
		cfg.Server.Security.Reports.RetentionDays = def.Server.Security.Reports.RetentionDays
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
}

func validateContactAntispam(value *ContactAntispamConfig, defaults ContactAntispamConfig) {
	checks := []struct {
		name  string
		value *int
		def   int
	}{
		{"min_fill_seconds", &value.MinFillSeconds, defaults.MinFillSeconds},
		{"max_age_seconds", &value.MaxAgeSeconds, defaults.MaxAgeSeconds},
		{"per_ip", &value.PerIP, defaults.PerIP},
		{"per_email", &value.PerEmail, defaults.PerEmail},
		{"global", &value.Global, defaults.Global},
		{"max_links", &value.MaxLinks, defaults.MaxLinks},
		{"strikes", &value.Strikes, defaults.Strikes},
		{"ban_hours", &value.BanHours, defaults.BanHours},
		{"review_min_seconds", &value.ReviewMinSeconds, defaults.ReviewMinSeconds},
		{"review_max_seconds", &value.ReviewMaxSeconds, defaults.ReviewMaxSeconds},
		{"duplicate_window_hours", &value.DuplicateWindowHours, defaults.DuplicateWindowHours},
	}
	for _, check := range checks {
		if *check.value < 0 {
			log.Printf("warning: invalid server.pages.contact.antispam.%s %d, using default %d", check.name, *check.value, check.def)
			*check.value = check.def
		}
	}
	if strings.TrimSpace(value.Window) == "" {
		value.Window = defaults.Window
	} else if parsed, err := time.ParseDuration(strings.TrimSpace(value.Window)); err != nil || parsed <= 0 {
		log.Printf("warning: invalid server.pages.contact.antispam.window %q, using default %q", value.Window, defaults.Window)
		value.Window = defaults.Window
	} else {
		value.Window = parsed.String()
	}
}

func validateCompliance(compliance *ComplianceConfig) {
	regions := make([]string, 0, len(compliance.GDPRAllowedRegions))
	seenRegions := make(map[string]struct{}, len(compliance.GDPRAllowedRegions))
	for _, raw := range compliance.GDPRAllowedRegions {
		region := strings.TrimSpace(raw)
		if region == "" {
			continue
		}
		if _, exists := seenRegions[region]; exists {
			continue
		}
		seenRegions[region] = struct{}{}
		regions = append(regions, region)
	}
	compliance.GDPRAllowedRegions = regions
	level := strings.ToLower(strings.TrimSpace(compliance.FedRAMPImpactLevel))
	switch level {
	case "low", "moderate", "high":
		compliance.FedRAMPImpactLevel = strings.ToUpper(level[:1]) + level[1:]
	case "":
		compliance.FedRAMPImpactLevel = ""
	default:
		log.Printf("warning: invalid server.compliance.fedramp_impact_level %q; unset it (valid values: Low, Moderate, High)", compliance.FedRAMPImpactLevel)
		compliance.FedRAMPImpactLevel = ""
	}
	if compliance.GDPR && len(compliance.GDPRAllowedRegions) == 0 {
		log.Printf("warning: server.compliance.gdpr is enabled but server.compliance.gdpr_allowed_regions is empty")
	}
	if compliance.FedRAMP && compliance.FedRAMPImpactLevel == "" {
		log.Printf("warning: server.compliance.fedramp is enabled but server.compliance.fedramp_impact_level is unset")
	}
}

func parseTokenCookieAge(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "y") {
		years, err := strconv.Atoi(strings.TrimSuffix(value, "y"))
		if err != nil || years < 1 || years > 100 {
			return 0, fmt.Errorf("invalid year duration")
		}
		return time.Duration(years) * 365 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days < 1 || days > 36500 {
			return 0, fmt.Errorf("invalid day duration")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	return duration, nil
}

var (
	seoMetaNameValidationPattern           = regexp.MustCompile(`^[a-z0-9:_.-]+$`)
	accentColorValidationPattern           = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	seoVerificationNamePattern             = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	seoReservedMetaNames                   = map[string]bool{"description": true, "keywords": true, "author": true, "robots": true, "viewport": true}
	seoVerificationValueValidationPatterns = map[string]*regexp.Regexp{
		"google":    regexp.MustCompile(`^[A-Za-z0-9_-]{1,43}$`),
		"bing":      regexp.MustCompile(`^[A-F0-9]{1,32}$`),
		"yandex":    regexp.MustCompile(`^[a-f0-9]{1,32}$`),
		"baidu":     regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`),
		"pinterest": regexp.MustCompile(`^[a-f0-9]{1,32}$`),
		"facebook":  regexp.MustCompile(`^[a-z0-9]{1,64}$`),
	}
)

func validateSEO(seo *SEOConfig, defaults SEOConfig) {
	separator := seo.TitleSeparator
	if len(separator) > 8 {
		log.Printf("warning: server.seo.title_separator exceeds 8 characters; using default")
		seo.TitleSeparator = defaults.TitleSeparator
	} else {
		seo.TitleSeparator = separator
	}
	if strings.TrimSpace(seo.TitleSeparator) == "" {
		seo.TitleSeparator = defaults.TitleSeparator
	}
	if seo.Canonical != "" {
		parsed, err := url.Parse(seo.Canonical)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			log.Printf("warning: invalid server.seo.canonical; using request origin")
			seo.Canonical = ""
		}
	}
	if len(seo.MetaTags) > 20 {
		log.Printf("warning: server.seo.meta_tags has more than 20 entries; extra entries will be ignored")
	}
	for name, value := range seo.MetaTags {
		if !seoMetaNameValidationPattern.MatchString(name) || seoReservedMetaNames[name] || value == "" {
			log.Printf("warning: ignoring invalid server.seo.meta_tags entry %q", name)
			delete(seo.MetaTags, name)
		}
	}
	for key, value := range map[string]string{
		"google": seo.Verification.Google, "bing": seo.Verification.Bing, "yandex": seo.Verification.Yandex,
		"baidu": seo.Verification.Baidu, "pinterest": seo.Verification.Pinterest, "facebook": seo.Verification.Facebook,
	} {
		if value == "" {
			continue
		}
		pattern := seoVerificationValueValidationPatterns[key]
		if pattern == nil || !pattern.MatchString(value) {
			log.Printf("warning: invalid server.seo.verification.%s value; ignoring it", key)
			switch key {
			case "google":
				seo.Verification.Google = ""
			case "bing":
				seo.Verification.Bing = ""
			case "yandex":
				seo.Verification.Yandex = ""
			case "baidu":
				seo.Verification.Baidu = ""
			case "pinterest":
				seo.Verification.Pinterest = ""
			case "facebook":
				seo.Verification.Facebook = ""
			}
		}
	}
	validCustom := seo.Verification.Custom[:0]
	for _, tag := range seo.Verification.Custom {
		nameValid := tag.Name != "" && seoVerificationNamePattern.MatchString(tag.Name)
		propertyValid := tag.Property != "" && seoVerificationNamePattern.MatchString(tag.Property)
		if (nameValid == propertyValid) || tag.Content == "" || len(tag.Content) > 256 {
			log.Printf("warning: ignoring invalid server.seo.verification.custom entry")
			continue
		}
		validCustom = append(validCustom, tag)
	}
	seo.Verification.Custom = validCustom
	if seo.OpenGraph.Type == "" {
		seo.OpenGraph.Type = defaults.OpenGraph.Type
	}
	if seo.Twitter.Card != "summary" && seo.Twitter.Card != "summary_large_image" {
		log.Printf("warning: invalid server.seo.twitter.card; using default %q", defaults.Twitter.Card)
		seo.Twitter.Card = defaults.Twitter.Card
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

func validateRestoreConfig(value *RestoreConfig, defaults RestoreConfig) {
	for _, setting := range []struct {
		name  string
		value *string
		def   string
	}{
		{name: "max_archive_size", value: &value.MaxArchiveSize, def: defaults.MaxArchiveSize},
		{name: "max_expanded_size", value: &value.MaxExpandedSize, def: defaults.MaxExpandedSize},
		{name: "max_file_size", value: &value.MaxFileSize, def: defaults.MaxFileSize},
	} {
		if _, err := parseConfigByteSize(*setting.value); err != nil {
			log.Printf("warning: invalid server.restore.%s %q, using default %q", setting.name, *setting.value, setting.def)
			*setting.value = setting.def
		}
	}
	if value.MaxFiles < 1 {
		log.Printf("warning: invalid server.restore.max_files %d, using default %d", value.MaxFiles, defaults.MaxFiles)
		value.MaxFiles = defaults.MaxFiles
	}
	if value.MaxRatio < 1 {
		log.Printf("warning: invalid server.restore.max_ratio %d, using default %d", value.MaxRatio, defaults.MaxRatio)
		value.MaxRatio = defaults.MaxRatio
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
	validate("disk_check", &tasks.DiskCheck.Schedule, defaults.DiskCheck.Schedule)
	validate("database_check", &tasks.DatabaseCheck.Schedule, defaults.DatabaseCheck.Schedule)
	validate("token_cleanup", &tasks.TokenCleanup.Schedule, defaults.TokenCleanup.Schedule)
	validate("log_rotation", &tasks.LogRotation.Schedule, defaults.LogRotation.Schedule)
	validate("backup", &tasks.Backup.Schedule, defaults.Backup.Schedule)
	validate("backup_hourly", &tasks.BackupHourly.Schedule, defaults.BackupHourly.Schedule)
	validate("health_check", &tasks.HealthCheck.Schedule, defaults.HealthCheck.Schedule)
	validate("tor_health", &tasks.TorHealth.Schedule, defaults.TorHealth.Schedule)
}

func validateDiskLowConfig(cfg *DiskLowConfig, defaults DiskLowConfig) {
	if cfg.ThresholdPercent < 1 || cfg.ThresholdPercent > 100 {
		log.Printf("warning: invalid server.notifications.disk_low.threshold_percent %d, using default %d", cfg.ThresholdPercent, defaults.ThresholdPercent)
		cfg.ThresholdPercent = defaults.ThresholdPercent
	}
	if repeat, err := time.ParseDuration(cfg.Repeat); err != nil || repeat <= 0 {
		log.Printf("warning: invalid server.notifications.disk_low.repeat %q, using default %q", cfg.Repeat, defaults.Repeat)
		cfg.Repeat = defaults.Repeat
	}
}

func validateDatabaseErrorConfig(cfg *DatabaseErrorConfig, defaults DatabaseErrorConfig) {
	if cfg.Failures < 1 {
		log.Printf("warning: invalid server.notifications.database_error.failures %d, using default %d", cfg.Failures, defaults.Failures)
		cfg.Failures = defaults.Failures
	}
	if repeat, err := time.ParseDuration(cfg.Repeat); err != nil || repeat <= 0 {
		log.Printf("warning: invalid server.notifications.database_error.repeat %q, using default %q", cfg.Repeat, defaults.Repeat)
		cfg.Repeat = defaults.Repeat
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
