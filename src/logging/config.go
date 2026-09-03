package logging

// FileConfig is the shared shape of every entry under "server.logs.*" in
// AI.md PART 11 "Logging" — one log file's filename, output format,
// rotation policy, and retention policy.
type FileConfig struct {
	// Enabled controls whether this log file is written at all. Only
	// "debug" defaults to false; every other log type defaults to true.
	Enabled bool
	// Filename is the log file's name relative to the logs directory
	// (e.g. "access.log").
	Filename string
	// Format is the output format for this log type (varies per type,
	// see AI.md "Log Format Details").
	Format string
	// Custom is the format string used when Format == "custom".
	Custom string
	// Rotate is the raw rotation policy string, e.g. "weekly,50MB".
	Rotate string
	// Keep is the raw retention policy string, e.g. "none", "30d", "4".
	Keep string
	// Compress gzip-compresses a rotated backup before retention is
	// applied. Only meaningful when Keep is not "none".
	Compress bool
}

// AuditFileConfig extends FileConfig with the audit log's event-category
// toggles and PII-handling flags, per AI.md "Audit Log Configuration".
type AuditFileConfig struct {
	FileConfig
	// IncludeUserAgent includes the request User-Agent string in audit
	// entries where applicable.
	IncludeUserAgent bool
	// Events toggles which audit event categories are logged.
	Events AuditEventsConfig
	// MaskEmails masks email addresses in audit log lines when true.
	// Defaults to true per AI.md "Sane Defaults".
	MaskEmails bool
}

// AuditEventsConfig toggles which audit event categories are written,
// per AI.md "Audit Log Configuration" § events. All categories default to
// true.
type AuditEventsConfig struct {
	// Configuration covers config.* events (config changes).
	Configuration bool
	// Security covers security.* events (rate limits, blocks, etc).
	Security bool
	// Backup covers backup.* events (backup/restore lifecycle).
	Backup bool
	// Server covers server.* and scheduler.* events (start, stop,
	// maintenance mode, scheduled task outcomes).
	Server bool
}

// AccessFileConfig extends FileConfig with the access log's health-check
// suppression toggle, per AI.md "Health-Check Log Suppression".
type AccessFileConfig struct {
	FileConfig
	// LogHealthChecks logs successful (2xx) health-check requests when
	// true. Failures are always logged regardless of this setting.
	LogHealthChecks bool
}

// Config mirrors the "server.logs" YAML block in AI.md PART 11.
type Config struct {
	// Level is the global minimum log level: debug, info, warn, error.
	Level string

	Access   AccessFileConfig
	Server   FileConfig
	Error    FileConfig
	// App is app.log (or "{project_name}.log") — general application
	// events (info/warn), logfmt by default. Per AI.md's "Log Files"
	// table; not itself covered by the main "server.logs" config block
	// example (a self-inconsistency in AI.md — the table is the more
	// complete source and is what this config follows).
	App FileConfig
	// Auth is auth.log — authentication events (token issue/revoke,
	// failures), syslog (RFC 3164) by default. Same table-vs-block-example
	// inconsistency in AI.md as App above.
	Auth     FileConfig
	Audit    AuditFileConfig
	Security FileConfig
	Debug    FileConfig
}

// DefaultConfig returns the sane defaults documented in AI.md "Log
// Rotation": access rotates monthly, audit rotates daily, everything else
// rotates weekly or at 50MB, and nothing is retained after rotation by
// default.
func DefaultConfig() Config {
	return Config{
		Level: "warn",
		Access: AccessFileConfig{
			FileConfig: FileConfig{
				Enabled:  true,
				Filename: "access.log",
				Format:   "apache",
				Rotate:   "monthly",
				Keep:     "none",
			},
			LogHealthChecks: false,
		},
		Server: FileConfig{
			Enabled:  true,
			Filename: "server.log",
			Format:   "text",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
		Error: FileConfig{
			Enabled:  true,
			Filename: "error.log",
			Format:   "text",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
		App: FileConfig{
			Enabled:  true,
			Filename: "app.log",
			Format:   "logfmt",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
		Auth: FileConfig{
			Enabled:  true,
			Filename: "auth.log",
			Format:   "syslog",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
		Audit: AuditFileConfig{
			FileConfig: FileConfig{
				Enabled:  true,
				Filename: "audit.log",
				Format:   "json",
				Rotate:   "daily",
				Keep:     "none",
				Compress: false,
			},
			IncludeUserAgent: true,
			Events: AuditEventsConfig{
				Configuration: true,
				Security:      true,
				Backup:        true,
				Server:        true,
			},
			MaskEmails: true,
		},
		Security: FileConfig{
			Enabled:  true,
			Filename: "security.log",
			Format:   "fail2ban",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
		Debug: FileConfig{
			Enabled:  false,
			Filename: "debug.log",
			Format:   "text",
			Rotate:   "weekly,50MB",
			Keep:     "none",
		},
	}
}
