package config

import (
	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/tor"
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    "", // Will be set to random 64xxx port
			FQDN:    "",
			Address: "[::]",
			Mode:    "production",
			Debug: DebugConfig{
				Pprof: true, LogQueries: true, LogCache: true,
				MaxBodyLogSize: "10KB", BlockProfileRate: 1,
				MutexProfileFraction: 1, RuntimeEndpoints: true,
			},
			Lang:         "auto",
			APIVersion:   "v1",
			BaseURL:      "/",
			URLDetection: URLDetectionConfig{Learning: true, MinSamples: 3, SampleWindow: "5m", LogChanges: true, LiveReload: true},
			I18n: I18nConfig{
				Enabled:          true,
				DefaultLanguage:  "en",
				Supported:        []string{"en", "es", "zh", "fr", "ar", "de", "ja"},
				FallbackLanguage: "en",
				CookieName:       "lang",
				CookieMaxAge:     "365d",
			},
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
			Limits: LimitsConfig{
				MaxBodySize:  "10MB",
				ReadTimeout:  "30s",
				WriteTimeout: "30s",
				IdleTimeout:  "120s",
			},
			Compression: CompressionConfig{
				Enabled: true,
				Level:   5,
				Types:   append([]string(nil), defaultCompressionTypes...),
			},
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
			Session: SessionConfig{
				CookieName: "session",
				MaxAge:     "0s",
				Secure:     "auto",
				HTTPOnly:   true,
				SameSite:   "lax",
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
				Enabled:         true,
				IncludeSystem:   true,
				Auth:            MetricsAuthConfig{Tokens: MetricsTokensConfig{}},
				Root:            MetricsRootConfig{Enabled: true},
				IncludeRuntime:  true,
				Loki:            MetricsLokiConfig{MaxEntries: 1000, MaxAge: "1h"},
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
					StoredOnServer: true,
					Sharing: []SharingCondition{
						{Condition: "analytics", When: "Tracking configured (server.tracking.type set) AND user consents", Data: "Anonymized: page views, browser type, country"},
						{Condition: "email", When: "SMTP configured for sending emails", Data: "Email address, message content"},
						{Condition: "user_initiated", When: "User explicitly shares content (social buttons, exports)", Data: "Whatever user chooses to share"},
					},
				},
				Retention: RetentionPolicy{
					Period:            "Airports does not maintain user accounts or user profiles. Rate-limit records are retained for their configured window and expired records are removed. Request logs follow the configured log rotation and retention settings. Analytics retention is controlled by the configured analytics provider.",
					ExportAvailable:   false,
					DeletionAvailable: false,
				},
				Consent: ConsentConfig{
					Message:               "In accordance with the EU GDPR law this message is being displayed. We use cookies for essential site functionality and, with your consent, for preferences and analytics. Your data is stored on our servers and is never sold.",
					MessageIfSold:         "In accordance with the EU GDPR law this message is being displayed. We use cookies for essential site functionality and, with your consent, for preferences and analytics. Your data may be shared with or sold to third parties as described in our Privacy Policy.",
					ShowUntilAcknowledged: true,
					DefaultEnabled:        true,
					Position:              "bottom",
					ShowPreferences:       true,
					PreferencesText:       "Manage Preferences",
					Policy: struct {
						Text string `yaml:"text"`
						URL  string `yaml:"url"`
					}{Text: "Privacy Policy", URL: "/server/privacy"},
					Buttons: struct {
						Decline string `yaml:"decline"`
						Accept  string `yaml:"accept"`
					}{Decline: "Decline", Accept: "I Agree"},
				},
				Cookies: CookieCategories{
					Essential:   CookieCategory{Enabled: true, Description: "Required for the site to function. Includes security tokens (CSRF) and site preferences. These cookies are strictly necessary and cannot be disabled."},
					Preferences: CookieCategory{Enabled: true, Description: "Remember your settings such as theme (dark/light), language, and UI preferences. Disabling will reset to defaults on each visit."},
					Analytics: AnalyticsCookie{
						CookieCategory:           CookieCategory{Enabled: true, Description: "Help us understand how visitors use our site to improve the experience."},
						DescriptionSuffixNotSold: "Analytics data is anonymized and never sold.",
						DescriptionSuffixSold:    "Analytics data may be shared with third parties.",
					},
				},
				ThirdParty: ThirdPartyConfig{Services: []ThirdPartyService{}},
				Content: PrivacyContent{
					DataCollection: `**We process only the information needed to provide and protect the service:**

**Operational information:**
- Client IP addresses are used for rate limiting, abuse prevention, and security. Rate-limit storage uses hashed IP identifiers.
- Request details may be recorded in logs when request logging is configured; log retention follows the configured rotation and retention policy.
- GeoIP lookup may use the client IP to select nearby airports. The lookup result is not logged or stored.

**Optional analytics (only when configured and allowed by consent):**
- Page views and browser information may be sent to the configured analytics provider.

**We do not maintain user accounts, user profiles, API keys, or a user-data store.**`,
					DataUsage: `**Information processed by Airports is used to:**

- **Provide airport data:** Serve search, geographic queries, and airport information.
- **Protect the service:** Apply rate limits and detect abuse using hashed client-IP identifiers.
- **Operate the service:** Use request logs only when logging is configured and retain them according to configured policy.
- **Improve the experience:** Send analytics only when a provider is configured and consent permits it.

Airports has no user accounts or user profiles, so it does not provide account data export or account deletion controls.`,
					DataUsageIfSold: `**Information processed by Airports is used to:**

- **Provide airport data:** Serve search, geographic queries, and airport information.
- **Protect the service:** Apply rate limits and detect abuse using hashed client-IP identifiers.
- **Operate the service:** Use request logs only when logging is configured and retain them according to configured policy.
- **Improve the experience:** Send analytics only when a provider is configured and consent permits it.


When the configured sold-data option is enabled, the operator may share or sell data as described in the configured sharing conditions. You can opt out using the control below.`,
					DataSecurity: `**How we protect your data:**

- Rate-limit storage uses hashed client-IP identifiers.
- Request log retention and rotation are controlled by server configuration.
- HTTPS/TLS is used when enabled by the server operator.`,
				},
				Contact: LegacyContactConfig{
					General: "",
					Abuse:   "",
				},
			},
			Contact: ContactConfig{
				Admin:    ContactRoleConfig{Email: "admin@{fqdn}"},
				Security: ContactRoleConfig{Email: "security@{fqdn}"},
				Abuse:    ContactRoleConfig{},
				General:  ContactRoleConfig{},
			},
			Pages: ServerPagesConfig{
				Contact: ContactPageConfig{
					Enabled:        true,
					Captcha:        "simple",
					SuccessMessage: "Thank you for your message. We'll respond soon.",
				},
			},
			Tracking: TrackingConfig{Type: "", ID: "", URL: ""},
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
			I2P: i2p.DefaultConfig(),
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
