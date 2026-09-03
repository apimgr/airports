package logging

import (
	"regexp"
	"strings"
	"time"
)

// Manager owns every configured log file's LogWriter and drives rotation
// across all of them from the single daily "log_rotation" scheduled task
// described in AI.md PART 18.
type Manager struct {
	dir      string
	cfg      Config
	access   *LogWriter
	server   *LogWriter
	errorLog *LogWriter
	app      *LogWriter
	auth     *LogWriter
	audit    *LogWriter
	security *LogWriter
	debug    *LogWriter
}

// NewManager creates a Manager rooted at dir (the project's logs
// directory) for the given Config. Log files themselves are opened lazily
// on first write, per LogWriter.
func NewManager(dir string, cfg Config) *Manager {
	return &Manager{
		dir:      dir,
		cfg:      cfg,
		access:   NewWriter(dir, "access", cfg.Access.FileConfig),
		server:   NewWriter(dir, "server", cfg.Server),
		errorLog: NewWriter(dir, "error", cfg.Error),
		app:      NewWriter(dir, "app", cfg.App),
		auth:     NewWriter(dir, "auth", cfg.Auth),
		audit:    NewWriter(dir, "audit", cfg.Audit.FileConfig),
		security: NewWriter(dir, "security", cfg.Security),
		debug:    NewWriter(dir, "debug", cfg.Debug),
	}
}

// Access returns the access.log writer.
func (m *Manager) Access() *LogWriter { return m.access }

// Server returns the server.log writer.
func (m *Manager) Server() *LogWriter { return m.server }

// Error returns the error.log writer.
func (m *Manager) Error() *LogWriter { return m.errorLog }

// App returns the app.log writer.
func (m *Manager) App() *LogWriter { return m.app }

// Auth returns the auth.log writer.
func (m *Manager) Auth() *LogWriter { return m.auth }

// Audit returns the audit.log writer.
func (m *Manager) Audit() *LogWriter { return m.audit }

// Security returns the security.log writer.
func (m *Manager) Security() *LogWriter { return m.security }

// Debug returns the debug.log writer.
func (m *Manager) Debug() *LogWriter { return m.debug }

// writers returns every managed LogWriter alongside a short name used in
// warning messages.
func (m *Manager) writers() map[string]*LogWriter {
	return map[string]*LogWriter{
		"access":   m.access,
		"server":   m.server,
		"error":    m.errorLog,
		"app":      m.app,
		"auth":     m.auth,
		"audit":    m.audit,
		"security": m.security,
		"debug":    m.debug,
	}
}

// AuditCategory identifies which of AuditEventsConfig's toggles governs an
// audit event, per AI.md "Audit Log Events" (config.* -> Configuration,
// security.* -> Security, backup.* -> Backup, server.*/scheduler.* ->
// Server).
type AuditCategory string

// Audit event categories, per AI.md "Audit Log Configuration" § events.
const (
	AuditCategoryConfiguration AuditCategory = "configuration"
	AuditCategorySecurity      AuditCategory = "security"
	AuditCategoryBackup        AuditCategory = "backup"
	AuditCategoryServer        AuditCategory = "server"
)

// enabled reports whether category is toggled on in events.
func (events AuditEventsConfig) enabled(category AuditCategory) bool {
	switch category {
	case AuditCategoryConfiguration:
		return events.Configuration
	case AuditCategorySecurity:
		return events.Security
	case AuditCategoryBackup:
		return events.Backup
	case AuditCategoryServer:
		return events.Server
	default:
		return true
	}
}

// emailPattern matches a bare email address for masking in audit log
// output. Intentionally simple — audit lines are structured log text, not
// arbitrary user documents, so a permissive-but-not-exhaustive pattern is
// sufficient.
var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// maskEmail replaces an email address with its first character followed by
// "***@" and the domain, e.g. "j***@example.com".
func maskEmail(addr string) string {
	at := strings.IndexByte(addr, '@')
	if at <= 0 {
		return addr
	}
	return addr[:1] + "***" + addr[at:]
}

// maskEmails masks every email address found in line.
func maskEmails(line string) string {
	return emailPattern.ReplaceAllStringFunc(line, maskEmail)
}

// WriteAudit writes a line to audit.log, honoring the category's
// events.* toggle and the mask_emails setting, per AI.md "Audit Log
// Configuration". A disabled category is silently skipped (not an error) —
// same convention as a disabled log file.
func (m *Manager) WriteAudit(category AuditCategory, line string) error {
	if !m.cfg.Audit.Events.enabled(category) {
		return nil
	}
	if m.cfg.Audit.MaskEmails {
		line = maskEmails(line)
	}
	return m.audit.Write(line)
}

// Rotate applies each configured log file's rotate/keep/compress policy,
// per AI.md's "log_rotation" scheduled task. It is meant to be called once
// daily at 00:00, but is idempotent — calling it more often is a no-op for
// any log file whose policy is not yet due. A failure rotating one log
// file is logged as a warning and never prevents the remaining log files
// from being processed; Rotate itself never returns an error.
func (m *Manager) Rotate() error {
	return m.RotateAt(time.Now())
}

// RotateAt is Rotate with an explicit "now", to make rotation-due checks
// deterministic in tests.
func (m *Manager) RotateAt(now time.Time) error {
	for name, w := range m.writers() {
		if _, err := w.RotateIfDue(now); err != nil {
			warnf("rotate %s log: %v", name, err)
		}
	}
	return nil
}

// Close closes every managed log file's open handle.
func (m *Manager) Close() error {
	var firstErr error
	for _, w := range m.writers() {
		if err := w.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
