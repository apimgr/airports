package logging

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// LogManager owns every configured log file's LogWriter and drives rotation
// across all of them from the single daily "log_rotation" scheduled task
// described in AI.md PART 18.
type LogManager struct {
	dir            string
	cfg            LogManagerConfig
	access         *LogWriter
	server         *LogWriter
	errorLog       *LogWriter
	app            *LogWriter
	auth           *LogWriter
	audit          *LogWriter
	security       *LogWriter
	debug          *LogWriter
	auditMu        sync.Mutex
	auditPrev      string
	auditIntegrity AuditIntegrityResult
	auditInitErr   error
}

// NewManager creates a LogManager rooted at dir (the project's logs
// directory) for the given LogManagerConfig. Log files themselves are opened lazily
// on first write, per LogWriter.
func NewLogManager(dir string, cfg LogManagerConfig) *LogManager {
	m := &LogManager{
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
	m.auditMu.Lock()
	lock, lockErr := m.lockAuditFile()
	if lockErr != nil {
		m.auditInitErr = lockErr
		warnf("lock audit log during startup verification: %v", lockErr)
		m.auditMu.Unlock()
		return m
	}
	if err := recoverAuditRotation(dir, cfg.Audit.Filename); err != nil {
		m.auditInitErr = err
		warnf("recover interrupted audit rotation: %v", err)
	}
	m.auditIntegrity = VerifyAuditLog(dir, cfg.Audit.Filename)
	if m.auditInitErr != nil {
		m.auditIntegrity.Intact = false
		m.auditIntegrity.File = cfg.Audit.Filename
		m.auditIntegrity.Line = 1
	}
	m.auditPrev = m.auditIntegrity.LastHash
	if m.auditPrev == "" {
		m.auditPrev, _, _ = readAuditAnchor(dir, cfg.Audit.Filename)
		if m.auditPrev == "" {
			m.auditPrev = auditChainGenesis
		}
	}
	if m.auditIntegrity.Intact {
		if _, found, anchorErr := readAuditAnchor(dir, cfg.Audit.Filename); anchorErr == nil && !found {
			if err := writeAuditAnchor(dir, cfg.Audit.Filename, m.auditPrev); err != nil {
				warnf("persist initial audit chain anchor: %v", err)
			}
		}
	}
	if !m.auditIntegrity.Intact && cfg.Audit.Enabled && !auditBreakAlreadyReported(dir, cfg.Audit.Filename, m.auditIntegrity.File, m.auditIntegrity.Line) {
		line := fmt.Sprintf("security.audit_chain_broken file=%q line=%d", m.auditIntegrity.File, m.auditIntegrity.Line)
		if err := m.writeAuditLocked(AuditCategorySecurity, line); err != nil {
			warnf("report audit chain failure: %v", err)
		}
	}
	if err := lock.Unlock(); err != nil {
		warnf("unlock audit log after startup verification: %v", err)
	}
	m.auditMu.Unlock()
	return m
}

// Access returns the access.log writer.
func (m *LogManager) Access() *LogWriter { return m.access }

// Server returns the server.log writer.
func (m *LogManager) Server() *LogWriter { return m.server }

// Error returns the error.log writer.
func (m *LogManager) Error() *LogWriter { return m.errorLog }

// App returns the app.log writer.
func (m *LogManager) App() *LogWriter { return m.app }

// Auth returns the auth.log writer.
func (m *LogManager) Auth() *LogWriter { return m.auth }

// AuditPath returns the active audit log path without exposing a raw writer.
func (m *LogManager) AuditPath() string { return m.audit.Path() }

// Security returns the security.log writer.
func (m *LogManager) Security() *LogWriter { return m.security }

// Debug returns the debug.log writer.
func (m *LogManager) Debug() *LogWriter { return m.debug }

// writers returns every managed LogWriter alongside a short name used in
// warning messages.
func (m *LogManager) writers() map[string]*LogWriter {
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
func (m *LogManager) WriteAudit(category AuditCategory, line string) error {
	if m.auditInitErr != nil {
		return fmt.Errorf("logging: audit log initialization failed: %w", m.auditInitErr)
	}
	if !m.cfg.Audit.Events.enabled(category) && !strings.Contains(line, "security.audit_chain_broken") {
		return nil
	}
	m.auditMu.Lock()
	defer m.auditMu.Unlock()
	lock, err := m.lockAuditFile()
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			warnf("unlock audit log after write: %v", err)
		}
	}()
	return m.writeAuditLocked(category, line)
}

func (m *LogManager) writeAuditLocked(category AuditCategory, line string) error {
	if !m.cfg.Audit.Events.enabled(category) && !strings.Contains(line, "security.audit_chain_broken") {
		return nil
	}
	if err := m.audit.Reopen(); err != nil {
		return err
	}
	integrity := VerifyAuditLog(m.dir, m.cfg.Audit.Filename)
	if integrity.LastHash != "" {
		m.auditPrev = integrity.LastHash
	} else if anchor, found, err := readAuditAnchor(m.dir, m.cfg.Audit.Filename); err == nil && found {
		m.auditPrev = anchor
	} else if m.auditPrev == "" {
		m.auditPrev = auditChainGenesis
	}
	if !integrity.Intact && !strings.Contains(line, "security.audit_chain_broken") && m.cfg.Audit.Enabled && !auditBreakAlreadyReported(m.dir, m.cfg.Audit.Filename, integrity.File, integrity.Line) {
		breakLine := fmt.Sprintf("security.audit_chain_broken file=%q line=%d", integrity.File, integrity.Line)
		if err := m.writeAuditEntryLocked(AuditCategorySecurity, breakLine); err != nil {
			return fmt.Errorf("logging: report audit chain failure: %w", err)
		}
	}
	return m.writeAuditEntryLocked(category, line)
}

func (m *LogManager) writeAuditEntryLocked(category AuditCategory, line string) error {
	if m.cfg.Audit.MaskEmails {
		line = maskEmails(line)
	}
	activeInfo, statErr := os.Stat(m.audit.Path())
	_, anchorExists, anchorErr := readAuditAnchor(m.dir, m.cfg.Audit.Filename)
	if anchorErr != nil {
		return fmt.Errorf("logging: read audit chain anchor: %w", anchorErr)
	}
	if !anchorExists && (errors.Is(statErr, os.ErrNotExist) || (statErr == nil && activeInfo.Size() == 0)) {
		if err := writeAuditAnchor(m.dir, m.cfg.Audit.Filename, m.auditPrev); err != nil {
			return fmt.Errorf("logging: write initial audit chain anchor: %w", err)
		}
	}
	now := time.Now().UTC()
	entry, err := makeAuditEntry(category, line, m.auditPrev, now)
	if err != nil {
		return fmt.Errorf("logging: create audit entry: %w", err)
	}
	encoded, err := auditEntryJSON(entry)
	if err != nil {
		return fmt.Errorf("logging: encode audit entry: %w", err)
	}
	if err := m.audit.Write(string(encoded)); err != nil {
		return err
	}
	hash := sha256.Sum256(encoded)
	m.auditPrev = hex.EncodeToString(hash[:])
	return nil
}

// AuditIntegrity reports the current state of the active and retained audit log chain.
func (m *LogManager) AuditIntegrity() AuditIntegrityResult {
	if m == nil {
		return AuditIntegrityResult{Intact: true}
	}
	m.auditMu.Lock()
	defer m.auditMu.Unlock()
	lock, err := m.lockAuditFile()
	if err != nil {
		return AuditIntegrityResult{File: filepath.Base(m.audit.Path()), Line: 1}
	}
	defer func() { _ = lock.Unlock() }()
	return VerifyAuditLog(m.dir, m.cfg.Audit.Filename)
}

func (m *LogManager) lockAuditFile() (*flock.Flock, error) {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return nil, fmt.Errorf("logging: create audit log directory: %w", err)
	}
	filename := m.cfg.Audit.Filename
	if filename == "" {
		filename = "audit.log"
	}
	lock := flock.New(filepath.Join(m.dir, filename+".lock"))
	if err := lock.Lock(); err != nil {
		return nil, fmt.Errorf("logging: lock audit log: %w", err)
	}
	return lock, nil
}

func auditBreakAlreadyReported(dir, filename, brokenFile string, brokenLine int) bool {
	if filename == "" {
		filename = "audit.log"
	}
	active := filepath.Join(dir, filename)
	paths, _ := filepath.Glob(active + ".*")
	paths = append(paths, active)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		var reader io.Reader = file
		var gz *gzip.Reader
		if strings.HasSuffix(path, ".gz") {
			gz, err = gzip.NewReader(file)
			if err != nil {
				_ = file.Close()
				continue
			}
			reader = gz
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			var entry struct {
				Event   string         `json:"event"`
				Details map[string]any `json:"details"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				continue
			}
			if entry.Event == "security.audit_chain_broken" && entry.Details["file"] == brokenFile {
				if fmt.Sprint(entry.Details["line"]) == fmt.Sprint(brokenLine) {
					if gz != nil {
						_ = gz.Close()
					}
					_ = file.Close()
					return true
				}
			}
		}
		if gz != nil {
			_ = gz.Close()
		}
		_ = file.Close()
	}
	return false
}

// Rotate applies each configured log file's rotate/keep/compress policy,
// per AI.md's "log_rotation" scheduled task. It is meant to be called once
// daily at 00:00, but is idempotent — calling it more often is a no-op for
// any log file whose policy is not yet due. A failure rotating one log
// file is logged as a warning and never prevents the remaining log files
// from being processed; Rotate itself never returns an error.
func (m *LogManager) Rotate() error {
	return m.RotateAt(time.Now())
}

// RotateAt is Rotate with an explicit "now", to make rotation-due checks
// deterministic in tests.
func (m *LogManager) RotateAt(now time.Time) error {
	for name, w := range m.writers() {
		if name == "audit" {
			continue
		}
		if _, err := w.RotateIfDue(now); err != nil {
			warnf("rotate %s log: %v", name, err)
		}
	}
	m.auditMu.Lock()
	lock, lockErr := m.lockAuditFile()
	if lockErr != nil {
		warnf("lock audit log before rotation: %v", lockErr)
		m.auditMu.Unlock()
		return nil
	}
	integrity := VerifyAuditLog(m.dir, m.cfg.Audit.Filename)
	if integrity.LastHash != "" {
		m.auditPrev = integrity.LastHash
	} else if anchor, found, err := readAuditAnchor(m.dir, m.cfg.Audit.Filename); err == nil && found {
		m.auditPrev = anchor
	}
	if !integrity.Intact && !auditBreakAlreadyReported(m.dir, m.cfg.Audit.Filename, integrity.File, integrity.Line) && m.cfg.Audit.Enabled {
		line := fmt.Sprintf("security.audit_chain_broken file=%q line=%d", integrity.File, integrity.Line)
		if err := m.writeAuditLocked(AuditCategorySecurity, line); err != nil {
			warnf("report audit chain failure before rotation: %v", err)
		}
	}
	if err := m.audit.Reopen(); err != nil {
		warnf("reopen audit log before rotation: %v", err)
	}
	activeInfo, activeErr := os.Stat(m.audit.Path())
	journalWritten := false
	if activeErr == nil && activeInfo.Size() > 0 {
		if err := writeAuditRotationJournal(m.dir, m.cfg.Audit.Filename, m.auditPrev); err != nil {
			warnf("record audit rotation state: %v", err)
		} else {
			journalWritten = true
		}
	}
	_, rotateErr := m.audit.RotateIfDue(now)
	if rotateErr != nil {
		warnf("rotate audit log: %v", rotateErr)
	}
	if journalWritten {
		if err := recoverAuditRotation(m.dir, m.cfg.Audit.Filename); err != nil {
			warnf("complete audit rotation state: %v", err)
		} else if integrity := VerifyAuditLog(m.dir, m.cfg.Audit.Filename); integrity.LastHash != "" {
			m.auditPrev = integrity.LastHash
		}
	}
	if err := lock.Unlock(); err != nil {
		warnf("unlock audit log after rotation: %v", err)
	}
	m.auditMu.Unlock()
	return nil
}

// PruneRotatedBefore removes rotated log files older than cutoff while
// preserving every active log. It is used only by maintenance disk-pressure
// cleanup, independently of each writer's normal rotation retention policy.
func (m *LogManager) PruneRotatedBefore(cutoff time.Time) (int, error) {
	if m == nil {
		return 0, nil
	}
	deleted := 0
	var firstErr error
	for name, writer := range m.writers() {
		count, err := writer.pruneRotatedBefore(cutoff)
		deleted += count
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("prune %s logs: %w", name, err)
		}
	}
	return deleted, firstErr
}

// Reopen refreshes the descriptors for currently open logs after an external
// rotation. It attempts every writer and returns the first error encountered.
func (m *LogManager) Reopen() error {
	var firstErr error
	for name, writer := range m.writers() {
		if err := writer.Reopen(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("reopen %s log: %w", name, err)
		}
	}
	return firstErr
}

// Close closes every managed log file's open handle.
func (m *LogManager) Close() error {
	var firstErr error
	for _, w := range m.writers() {
		if err := w.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
