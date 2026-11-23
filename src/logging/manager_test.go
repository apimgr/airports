package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagerRotateAt(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Server.Rotate = "never,1B"
	cfg.Server.Keep = "none"

	m := NewLogManager(dir, cfg)
	if err := m.Server().Write("some server event"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := m.RotateAt(time.Now()); err != nil {
		t.Fatalf("RotateAt: %v", err)
	}

	if _, err := os.Stat(m.Server().Path()); !os.IsNotExist(err) {
		t.Errorf("expected server.log to be rotated away, stat err = %v", err)
	}
}

func TestManagerRecoversInterruptedAuditRotation(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	m := NewLogManager(dir, cfg)
	if err := m.WriteAudit(AuditCategorySecurity, "event=security.before_rotation"); err != nil {
		t.Fatalf("WriteAudit before simulated crash: %v", err)
	}
	previous := m.AuditIntegrity().LastHash
	if len(previous) != 64 {
		t.Fatalf("last audit hash = %q, want SHA-256", previous)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close before simulated crash: %v", err)
	}
	if err := writeAuditRotationJournal(dir, cfg.Audit.Filename, previous); err != nil {
		t.Fatalf("write simulated rotation journal: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, cfg.Audit.Filename)); err != nil {
		t.Fatalf("simulate keep:none removing the rotated file: %v", err)
	}

	recovered := NewLogManager(dir, cfg)
	defer func() { _ = recovered.Close() }()
	if !recovered.AuditIntegrity().Intact || recovered.AuditIntegrity().LastHash != previous {
		t.Fatalf("recovered audit integrity = %+v, want intact at prior hash %s", recovered.AuditIntegrity(), previous)
	}
	if _, err := os.Stat(auditRotationJournalPath(dir, cfg.Audit.Filename)); !os.IsNotExist(err) {
		t.Fatalf("rotation journal still exists after recovery: %v", err)
	}
	if err := recovered.WriteAudit(AuditCategorySecurity, "event=security.after_recovery"); err != nil {
		t.Fatalf("WriteAudit after recovery: %v", err)
	}
	data, err := os.ReadFile(recovered.AuditPath())
	if err != nil {
		t.Fatalf("read recovered audit entries: %v", err)
	}
	var entry auditEntry
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("decode post-recovery audit entry: %v", err)
	}
	if entry.Prev != previous {
		t.Fatalf("post-recovery entry prev = %s, want %s", entry.Prev, previous)
	}
}

func TestManagerReopenAfterExternalRotation(t *testing.T) {
	dir := t.TempDir()
	m := NewLogManager(dir, DefaultLogManagerConfig())
	defer func() { _ = m.Close() }()
	if err := m.Server().Write("before external rotation"); err != nil {
		t.Fatalf("Write before rotation: %v", err)
	}

	rotatedPath := m.Server().Path() + ".rotated"
	if err := os.Rename(m.Server().Path(), rotatedPath); err != nil {
		t.Fatalf("rename active log: %v", err)
	}
	if err := m.Reopen(); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if err := m.Server().Write("after external rotation"); err != nil {
		t.Fatalf("Write after rotation: %v", err)
	}

	rotated, err := os.ReadFile(rotatedPath)
	if err != nil {
		t.Fatalf("read rotated log: %v", err)
	}
	active, err := os.ReadFile(m.Server().Path())
	if err != nil {
		t.Fatalf("read reopened active log: %v", err)
	}
	if !strings.Contains(string(rotated), "before external rotation") || strings.Contains(string(rotated), "after external rotation") {
		t.Errorf("rotated log contents = %q, want only pre-rotation entry", rotated)
	}
	if !strings.Contains(string(active), "after external rotation") || strings.Contains(string(active), "before external rotation") {
		t.Errorf("active log contents = %q, want only post-reopen entry", active)
	}
}

func TestManagerRotateSkipsDisabledLogs(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()

	m := NewLogManager(dir, cfg)
	// Debug log is disabled by default and was never written, so rotation
	// must be a clean no-op rather than an error.
	if err := m.RotateAt(time.Now()); err != nil {
		t.Fatalf("RotateAt: %v", err)
	}
	if _, err := os.Stat(m.Debug().Path()); !os.IsNotExist(err) {
		t.Errorf("expected debug.log to not exist, stat err = %v", err)
	}
}

func TestManagerRotateContinuesAfterOneFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Server.Rotate = "never,1B"
	cfg.Server.Keep = "none"
	// error.log's rotate policy is deliberately invalid so its RotateIfDue
	// call always fails to parse — this must not stop server.log (or any
	// other log) from rotating cleanly in the same RotateAt pass.
	cfg.Error.Rotate = "not-a-valid-policy"
	cfg.Error.Keep = "none"

	m := NewLogManager(dir, cfg)
	if err := m.Server().Write("server event"); err != nil {
		t.Fatalf("Write server: %v", err)
	}
	if err := m.Error().Write("error event"); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := m.RotateAt(time.Now()); err != nil {
		t.Fatalf("RotateAt should never return an error itself: %v", err)
	}

	if _, err := os.Stat(m.Server().Path()); !os.IsNotExist(err) {
		t.Errorf("expected server.log to still rotate despite error.log failing, stat err = %v", err)
	}
	if _, err := os.Stat(m.Error().Path()); err != nil {
		t.Errorf("expected error.log to remain untouched after its own rotation failed, stat err = %v", err)
	}
}

func TestDefaultLogManagerConfigMatchesSpecDefaults(t *testing.T) {
	cfg := DefaultLogManagerConfig()

	if cfg.Access.Rotate != "monthly" || cfg.Access.Keep != "none" {
		t.Errorf("access defaults = %q/%q, want monthly/none", cfg.Access.Rotate, cfg.Access.Keep)
	}
	if cfg.Audit.Rotate != "daily" || cfg.Audit.Keep != "none" {
		t.Errorf("audit defaults = %q/%q, want daily/none", cfg.Audit.Rotate, cfg.Audit.Keep)
	}
	for name, fc := range map[string]FileConfig{
		"server":   cfg.Server,
		"error":    cfg.Error,
		"security": cfg.Security,
		"debug":    cfg.Debug,
		"app":      cfg.App,
		"auth":     cfg.Auth,
	} {
		if fc.Rotate != "weekly,50MB" || fc.Keep != "none" {
			t.Errorf("%s defaults = %q/%q, want weekly,50MB/none", name, fc.Rotate, fc.Keep)
		}
	}
	if cfg.Debug.Enabled {
		t.Errorf("debug log should be disabled by default")
	}
	if !cfg.Audit.Enabled || cfg.Audit.Format != "json" {
		t.Errorf("audit log must default to enabled and json-only, got enabled=%v format=%q", cfg.Audit.Enabled, cfg.Audit.Format)
	}
	if cfg.App.Format != "logfmt" {
		t.Errorf("app.log default format = %q, want logfmt", cfg.App.Format)
	}
	if cfg.Auth.Format != "syslog" {
		t.Errorf("auth.log default format = %q, want syslog", cfg.Auth.Format)
	}
	if !cfg.Audit.MaskEmails {
		t.Errorf("audit.mask_emails must default to true")
	}
	if !cfg.Audit.Events.Configuration || !cfg.Audit.Events.Security || !cfg.Audit.Events.Backup || !cfg.Audit.Events.Server {
		t.Errorf("all audit event categories must default to true, got %+v", cfg.Audit.Events)
	}
}

func TestManagerWriteAllLogTypes(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Debug.Enabled = true
	m := NewLogManager(dir, cfg)
	defer func() { _ = m.Close() }()

	writers := []*LogWriter{m.Access(), m.Server(), m.Error(), m.App(), m.Auth(), m.Security(), m.Debug()}
	for _, w := range writers {
		if err := w.Write("line"); err != nil {
			t.Fatalf("Write(%s): %v", w.cfg.Filename, err)
		}
	}

	for _, w := range writers {
		data, err := os.ReadFile(w.Path())
		if err != nil {
			t.Fatalf("read %s: %v", w.cfg.Filename, err)
		}
		if !strings.Contains(string(data), "line") {
			t.Errorf("%s missing expected content", w.cfg.Filename)
		}
	}
	if err := m.WriteAudit(AuditCategoryServer, "event=server.started"); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}
	if _, err := os.Stat(m.AuditPath()); err != nil {
		t.Fatalf("audit log was not created: %v", err)
	}
}

func TestWriteAuditMasksEmailsByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	m := NewLogManager(dir, cfg)
	defer func() { _ = m.Close() }()

	if err := m.WriteAudit(AuditCategorySecurity, `event=security.report_received actor=jane.doe@example.com`); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.AuditPath())
	if err != nil {
		t.Fatalf("read audit.log: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "jane.doe@example.com") {
		t.Errorf("audit.log contains unmasked email: %q", got)
	}
	if !strings.Contains(got, "j***@example.com") {
		t.Errorf("audit.log missing masked email, got %q", got)
	}
	var entry auditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("audit entry is not JSON: %v", err)
	}
	if entry.Prev != auditChainGenesis || !strings.HasPrefix(entry.ID, "audit_") || len(strings.TrimPrefix(entry.ID, "audit_")) != 26 || entry.Time == "" {
		t.Errorf("audit entry missing required identifiers or chain hash: %+v", entry)
	}
}

func TestWriteAuditMaskEmailsDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Audit.MaskEmails = false
	m := NewLogManager(dir, cfg)
	defer func() { _ = m.Close() }()

	if err := m.WriteAudit(AuditCategorySecurity, `actor=jane.doe@example.com`); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.AuditPath())
	if err != nil {
		t.Fatalf("read audit.log: %v", err)
	}
	if !strings.Contains(string(data), "jane.doe@example.com") {
		t.Errorf("expected unmasked email when mask_emails=false, got %q", string(data))
	}
}

func TestAuditHashChainDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	m := NewLogManager(dir, cfg)
	if err := m.WriteAudit(AuditCategoryServer, "event=server.started"); err != nil {
		t.Fatalf("write first audit event: %v", err)
	}
	if err := m.WriteAudit(AuditCategorySecurity, "security.ip_blocked ip=203.0.113.1"); err != nil {
		t.Fatalf("write second audit event: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("close logs: %v", err)
	}
	verified := VerifyAuditLog(dir, "audit.log")
	if !verified.Intact || verified.Entries != 2 {
		t.Fatalf("audit chain verification = %+v, want intact with two entries", verified)
	}

	path := filepath.Join(dir, "audit.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	data = bytes.Replace(data, []byte("server.started"), []byte("server.stopped"), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("tamper audit log: %v", err)
	}
	verified = VerifyAuditLog(dir, "audit.log")
	if verified.Intact || verified.Line != 2 {
		t.Fatalf("tampered audit chain verification = %+v, want link failure at line 2", verified)
	}
}

func TestAuditHashChainContinuesAcrossRotation(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Audit.Rotate = "never,1B"
	cfg.Audit.Keep = "forever"
	m := NewLogManager(dir, cfg)
	if err := m.WriteAudit(AuditCategoryServer, "event=server.started"); err != nil {
		t.Fatalf("write first audit event: %v", err)
	}
	if err := m.RotateAt(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("rotate audit log: %v", err)
	}
	if err := m.WriteAudit(AuditCategoryServer, "event=server.stopped"); err != nil {
		t.Fatalf("write post-rotation audit event: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("close logs: %v", err)
	}
	entries, err := verifyRetainedAuditChain(dir)
	if err != nil || entries != 2 {
		t.Fatalf("rotated audit chain verification = (%d, %v), want intact with two entries", entries, err)
	}
}

func TestAuditHashChainOrdersSameSecondRotationSuffixes(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Audit.Rotate = "never,1B"
	cfg.Audit.Keep = "forever"
	m := NewLogManager(dir, cfg)
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		if err := m.WriteAudit(AuditCategoryServer, fmt.Sprintf("event=server.started sequence=%d", i)); err != nil {
			t.Fatalf("write audit entry %d: %v", i, err)
		}
		if err := m.RotateAt(now); err != nil {
			t.Fatalf("rotate after audit entry %d: %v", i, err)
		}
	}
	if err := m.WriteAudit(AuditCategoryServer, "event=server.stopped sequence=12"); err != nil {
		t.Fatalf("write final audit entry: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("close logs: %v", err)
	}
	entries, err := verifyRetainedAuditChain(dir)
	if err != nil || entries != 13 {
		t.Fatalf("same-second rotated audit chain = (%d, %v), want intact with 13 entries", entries, err)
	}
}

func verifyRetainedAuditChain(dir string) (int, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "audit.log.*"))
	if err != nil {
		return 0, err
	}
	segments := make([]string, 0, len(paths)+1)
	for _, path := range paths {
		if strings.HasSuffix(path, ".chain") || strings.HasSuffix(path, ".lock") || strings.HasSuffix(path, ".tmp") {
			continue
		}
		segments = append(segments, path)
	}
	sort.Slice(segments, func(i, j int) bool {
		leftStem, leftNumber := auditSegmentOrder(segments[i])
		rightStem, rightNumber := auditSegmentOrder(segments[j])
		if leftStem != rightStem {
			return leftStem < rightStem
		}
		return leftNumber < rightNumber
	})
	segments = append(segments, filepath.Join(dir, "audit.log"))
	previous := auditChainGenesis
	total := 0
	for _, path := range segments {
		count, last, line, err := verifyAuditFile(path, previous)
		if err != nil {
			return total, fmt.Errorf("verify %s at line %d: %w", filepath.Base(path), line, err)
		}
		total += count
		if last != "" {
			previous = last
		}
	}
	return total, nil
}

func auditSegmentOrder(path string) (string, int) {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".gz")
	lastDot := strings.LastIndexByte(base, '.')
	if lastDot < 0 {
		return base, 0
	}
	number, err := strconv.Atoi(base[lastDot+1:])
	if err != nil {
		return base, 0
	}
	return base[:lastDot], number
}

func TestAuditChainBreakIsReportedOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audit.log"), []byte("not-json\n"), 0600); err != nil {
		t.Fatalf("write corrupt audit file: %v", err)
	}
	cfg := DefaultLogManagerConfig()
	cfg.Audit.Events.Security = false
	first := NewLogManager(dir, cfg)
	if err := first.Close(); err != nil {
		t.Fatalf("close first manager: %v", err)
	}
	second := NewLogManager(dir, cfg)
	if err := second.Close(); err != nil {
		t.Fatalf("close second manager: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if got := strings.Count(string(data), `"event":"security.audit_chain_broken"`); got != 1 {
		t.Fatalf("audit chain failure was reported %d times, want once: %s", got, data)
	}
}

func TestAuditEntryRedactsCredentialsAndMasksTokens(t *testing.T) {
	entry, err := makeAuditEntry(AuditCategorySecurity, "event=security.test password=mailPassword token=token_1234567890 api_token=supersecretvalue", auditChainGenesis, time.Now())
	if err != nil {
		t.Fatalf("make audit entry: %v", err)
	}
	encoded, err := auditEntryJSON(entry)
	if err != nil {
		t.Fatalf("encode audit entry: %v", err)
	}
	got := string(encoded)
	for _, raw := range []string{"mailPassword", "token_1234567890", "supersecretvalue"} {
		if strings.Contains(got, raw) {
			t.Errorf("audit entry leaked sensitive value %q: %s", raw, got)
		}
	}
	for _, masked := range []string{"********", "token_12...", "supersec..."} {
		if !strings.Contains(got, masked) {
			t.Errorf("audit entry missing masked value %q: %s", masked, got)
		}
	}
}

func TestTokenLifecycleAuditUsesStructuredActorAndSource(t *testing.T) {
	entry, err := makeAuditEntry(AuditCategorySecurity, `event=token.created owner_id="owner-123" token_prefix="own_12345678" actor="owner:owner-123" source=api ip=203.0.113.5 details_source=issue`, auditChainGenesis, time.Now())
	if err != nil {
		t.Fatalf("make token audit entry: %v", err)
	}
	if entry.Event != "token.created" || entry.Source != "api" || entry.Actor["id"] != "owner:owner-123" || entry.Actor["ip"] != "203.0.113.5" || entry.Details["source"] != "issue" {
		t.Fatalf("token lifecycle audit fields = %+v, want structured event, actor, source, and issue detail", entry)
	}
}

func TestComplianceAuditPreservesActorType(t *testing.T) {
	entry, err := makeAuditEntry(AuditCategoryServer, `event=compliance.consent_updated actor_type=visitor source=web consent_type=cookie_consent`, auditChainGenesis, time.Now())
	if err != nil {
		t.Fatalf("make consent audit entry: %v", err)
	}
	if entry.Category != "compliance" || entry.Actor["type"] != "visitor" || entry.Actor["id"] != nil {
		t.Fatalf("consent audit fields = %+v, want compliance category and visitor actor type without an anonymous user ID", entry)
	}
	if _, exists := entry.Details["actor_type"]; exists {
		t.Fatalf("actor_type leaked into event details: %+v", entry.Details)
	}
}

func TestWriteAuditSkipsDisabledCategory(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultLogManagerConfig()
	cfg.Audit.Events.Backup = false
	m := NewLogManager(dir, cfg)
	defer func() { _ = m.Close() }()

	if err := m.WriteAudit(AuditCategoryBackup, "backup.created"); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}
	if err := m.WriteAudit(AuditCategorySecurity, "security.ip_blocked"); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.AuditPath())
	if err != nil {
		t.Fatalf("read audit.log: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "backup.created") {
		t.Errorf("disabled backup category was logged: %q", got)
	}
	if !strings.Contains(got, "security.ip_blocked") {
		t.Errorf("enabled security category missing: %q", got)
	}
}
