package notify

import (
	"os"
	"path/filepath"
	"testing"
)

// testTempDir creates an isolated temp dir under /tmp/apimgr/airports-XXXXXX
// per project convention, and returns it plus automatic cleanup.
func testTempDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0755); err != nil {
		t.Fatalf("MkdirAll base: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-notify-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestLoadTemplateEmbeddedDefault(t *testing.T) {
	subject, body, err := LoadTemplate("", "test")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if subject != "Test Email - {app_name}" {
		t.Errorf("subject = %q, want %q", subject, "Test Email - {app_name}")
	}
	if body == "" {
		t.Error("body is empty")
	}
}

func TestLoadTemplateAllDefaultsParse(t *testing.T) {
	names := []string{
		"security_alert", "backup_complete", "backup_failed",
		"ssl_expiring", "ssl_renewed", "ssl_renewal_failed",
		"scheduler_error", "update_available", "update_installed", "test",
	}
	for _, name := range names {
		subject, body, err := LoadTemplate("", name)
		if err != nil {
			t.Errorf("LoadTemplate(%q): %v", name, err)
			continue
		}
		if subject == "" {
			t.Errorf("LoadTemplate(%q): empty subject", name)
		}
		if body == "" {
			t.Errorf("LoadTemplate(%q): empty body", name)
		}
	}
}

func TestLoadTemplateUnknownName(t *testing.T) {
	if _, _, err := LoadTemplate("", "does_not_exist"); err == nil {
		t.Error("expected error for unknown template name")
	}
}

func TestLoadTemplateConfigDirOverride(t *testing.T) {
	dir := testTempDir(t)
	overrideDir := filepath.Join(dir, "template", "email")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	custom := "Subject: Custom Test - {app_name}\n---\nCustom body {app_name}.\n"
	if err := os.WriteFile(filepath.Join(overrideDir, "test.txt"), []byte(custom), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	subject, body, err := LoadTemplate(dir, "test")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if subject != "Custom Test - {app_name}" {
		t.Errorf("subject = %q, want override subject", subject)
	}
	if body != "Custom body {app_name}.\n" {
		t.Errorf("body = %q, want override body", body)
	}
}

func TestLoadTemplateConfigDirFallsBackWhenMissing(t *testing.T) {
	dir := testTempDir(t)

	subject, _, err := LoadTemplate(dir, "test")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if subject != "Test Email - {app_name}" {
		t.Errorf("subject = %q, want embedded default", subject)
	}
}

func TestParseTemplateMissingSeparator(t *testing.T) {
	if _, _, err := parseTemplate("Subject: X\nno separator here\n"); err == nil {
		t.Error("expected error for missing separator")
	}
}

func TestParseTemplateMissingSubjectPrefix(t *testing.T) {
	if _, _, err := parseTemplate("Not a subject line\n---\nbody\n"); err == nil {
		t.Error("expected error for missing Subject: prefix")
	}
}

func TestParseTemplateTooShort(t *testing.T) {
	if _, _, err := parseTemplate("only one line"); err == nil {
		t.Error("expected error for too-short template")
	}
}
