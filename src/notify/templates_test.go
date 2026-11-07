package notify

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
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

func TestLoadTemplateWarnsAboutUnknownVariable(t *testing.T) {
	dir := testTempDir(t)
	overrideDir := filepath.Join(dir, "template", "email")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	custom := "Subject: Custom - {foo}\n---\nBody for {fqdn}.\n"
	if err := os.WriteFile(filepath.Join(overrideDir, "test.txt"), []byte(custom), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var logOutput bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logOutput)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	if _, _, err := LoadTemplate(dir, "test"); err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if got := logOutput.String(); !strings.Contains(got, `unknown variable {foo}`) || !strings.Contains(got, `did you mean {fqdn}`) {
		t.Errorf("warning = %q, want unknown-variable message with fqdn suggestion", got)
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

func TestParseTemplateRejectsEmptySubject(t *testing.T) {
	if _, _, err := parseTemplate("Subject:   \n---\nbody\n"); err == nil || err.Error() != "subject cannot be empty" {
		t.Errorf("parseTemplate error = %v, want empty subject error", err)
	}
}

func TestParseTemplateRejectsEmptyBody(t *testing.T) {
	if _, _, err := parseTemplate("Subject: valid\n---\n \n"); err == nil || err.Error() != "body cannot be empty" {
		t.Errorf("parseTemplate error = %v, want empty body error", err)
	}
}

func TestParseTemplateRejectsMalformedVariable(t *testing.T) {
	for _, raw := range []string{
		"Subject: {app_name\n---\nbody\n",
		"Subject: app_name}\n---\nbody\n",
		"Subject: {}\n---\nbody\n",
		"Subject: {bad-name}\n---\nbody\n",
	} {
		if _, _, err := parseTemplate(raw); err == nil {
			t.Errorf("parseTemplate(%q) succeeded, want malformed variable error", raw)
		}
	}
}
