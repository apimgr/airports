package notify

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
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
	subject, body, err := LoadEmailTemplate("", "test")
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
		"scheduler_error", "startup", "shutdown", "disk_low", "database_error",
		"update_available", "update_failed", "update_staged", "update_installed",
		"maintenance_entered", "maintenance_recovered", "maintenance_exhausted", "contact_submission",
		"security_report_received", "security_report_acknowledgment", "security_report_status", "test",
	}
	for _, name := range names {
		subject, body, err := LoadEmailTemplate("", name)
		if err != nil {
			t.Errorf("LoadEmailTemplate(%q): %v", name, err)
			continue
		}
		if subject == "" {
			t.Errorf("LoadEmailTemplate(%q): empty subject", name)
		}
		if body == "" {
			t.Errorf("LoadEmailTemplate(%q): empty body", name)
		}
	}
}

func TestPreviewEmailTemplatesIncludesBuiltInsAndRendersSampleValues(t *testing.T) {
	previews, err := PreviewEmailTemplates("", &config.ApplicationConfig{})
	if err != nil {
		t.Fatalf("PreviewEmailTemplates: %v", err)
	}
	byName := make(map[string]EmailTemplatePreview, len(previews))
	for _, preview := range previews {
		byName[preview.Name] = preview
	}
	for _, name := range []string{"startup", "shutdown", "disk_low", "database_error", "update_failed", "update_staged"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("preview omitted built-in %q", name)
		}
	}
	if got := byName["startup"].Subject; strings.Contains(got, "{app_name}") {
		t.Errorf("startup subject was not rendered: %q", got)
	}
}

func TestTestEmailSubjectPrefix(t *testing.T) {
	if got := TestEmailSubject("Test Email"); got != "[TEST] Test Email" {
		t.Errorf("TestEmailSubject() = %q", got)
	}
}

func TestMaintenanceExhaustedEmailTranslations(t *testing.T) {
	variables := map[string]string{"app_name": "Airports", "attempts": "5", "reason": "database unavailable"}
	for _, language := range i18n.RegisteredLanguageCodes() {
		subject, body := TranslateEmailTemplate("", "maintenance_exhausted", "fallback subject", "fallback body", language, variables)
		if strings.Contains(subject, "email.templates.maintenance_exhausted") || strings.Contains(body, "email.templates.maintenance_exhausted") {
			t.Errorf("%s maintenance exhaustion email fell back to a translation key", language)
		}
		for _, value := range []string{"Airports", "5", "database unavailable"} {
			if !strings.Contains(subject+body, value) {
				t.Errorf("%s maintenance exhaustion email omitted %q", language, value)
			}
		}
	}
}

func TestLoadTemplateUnknownName(t *testing.T) {
	if _, _, err := LoadEmailTemplate("", "does_not_exist"); err == nil {
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

	subject, body, err := LoadEmailTemplate(dir, "test")
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
	if _, _, err := LoadEmailTemplate(dir, "test"); err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if got := logOutput.String(); !strings.Contains(got, `unknown variable {foo}`) || !strings.Contains(got, `did you mean {fqdn}`) {
		t.Errorf("warning = %q, want unknown-variable message with fqdn suggestion", got)
	}
}

func TestLoadTemplateConfigDirFallsBackWhenMissing(t *testing.T) {
	dir := testTempDir(t)

	subject, _, err := LoadEmailTemplate(dir, "test")
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
