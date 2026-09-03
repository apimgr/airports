package logging

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestManagerRotateAt(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Server.Rotate = "never,1B"
	cfg.Server.Keep = "none"

	m := NewManager(dir, cfg)
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

func TestManagerRotateSkipsDisabledLogs(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()

	m := NewManager(dir, cfg)
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
	cfg := DefaultConfig()
	cfg.Server.Rotate = "never,1B"
	cfg.Server.Keep = "none"
	// error.log's rotate policy is deliberately invalid so its RotateIfDue
	// call always fails to parse — this must not stop server.log (or any
	// other log) from rotating cleanly in the same RotateAt pass.
	cfg.Error.Rotate = "not-a-valid-policy"
	cfg.Error.Keep = "none"

	m := NewManager(dir, cfg)
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

func TestDefaultConfigMatchesSpecDefaults(t *testing.T) {
	cfg := DefaultConfig()

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
	cfg := DefaultConfig()
	cfg.Debug.Enabled = true
	m := NewManager(dir, cfg)
	defer m.Close()

	writers := []*LogWriter{m.Access(), m.Server(), m.Error(), m.App(), m.Auth(), m.Audit(), m.Security(), m.Debug()}
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
}

func TestWriteAuditMasksEmailsByDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	m := NewManager(dir, cfg)
	defer m.Close()

	if err := m.WriteAudit(AuditCategorySecurity, `event=security.report_received actor=jane.doe@example.com`); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.Audit().Path())
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
}

func TestWriteAuditMaskEmailsDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Audit.MaskEmails = false
	m := NewManager(dir, cfg)
	defer m.Close()

	if err := m.WriteAudit(AuditCategorySecurity, `actor=jane.doe@example.com`); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.Audit().Path())
	if err != nil {
		t.Fatalf("read audit.log: %v", err)
	}
	if !strings.Contains(string(data), "jane.doe@example.com") {
		t.Errorf("expected unmasked email when mask_emails=false, got %q", string(data))
	}
}

func TestWriteAuditSkipsDisabledCategory(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Audit.Events.Backup = false
	m := NewManager(dir, cfg)
	defer m.Close()

	if err := m.WriteAudit(AuditCategoryBackup, "backup.created"); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}
	if err := m.WriteAudit(AuditCategorySecurity, "security.ip_blocked"); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	data, err := os.ReadFile(m.Audit().Path())
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
