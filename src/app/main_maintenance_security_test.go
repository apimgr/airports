package app

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func closeSecurityTestResource(t *testing.T, closeResource func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := closeResource(); err != nil {
			t.Errorf("close security maintenance test database: %v", err)
		}
	})
}

func TestParseSecurityMaintenanceArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "list all", input: "list", want: []string{"list", ""}},
		{name: "list filtered", input: "list\x00--status\x00triaged", want: []string{"list", "triaged"}},
		{name: "show", input: "show\x00sec_0123456789abcdef", want: []string{"show", "sec_0123456789abcdef"}},
		{name: "update keeps comment spaces", input: "update\x00sec_0123456789abcdef\x00--comment\x00Needs coordinated disclosure", want: []string{"update", "sec_0123456789abcdef"}},
		{name: "reject invalid list status", input: "list\x00--status\x00unknown", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSecurityMaintenanceArgs(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseSecurityMaintenanceArgs() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && (len(got) < len(test.want) || !reflect.DeepEqual(got[:len(test.want)], test.want)) {
				t.Fatalf("parseSecurityMaintenanceArgs() = %#v, want prefix %#v", got, test.want)
			}
			if err == nil && test.name == "update keeps comment spaces" {
				options := decodeSecurityMaintenanceOptions(got[2])
				if options.Comment == nil || *options.Comment != "Needs coordinated disclosure" {
					t.Fatalf("comment = %v", options.Comment)
				}
			}
		})
	}
}

func TestCanAdvanceSecurityReport(t *testing.T) {
	tests := []struct {
		oldStatus string
		newStatus string
		want      bool
	}{
		{oldStatus: "received", newStatus: "triaged", want: true},
		{oldStatus: "confirmed", newStatus: "patching", want: true},
		{oldStatus: "patching", newStatus: "triaged", want: false},
		{oldStatus: "disclosed", newStatus: "wont_fix", want: false},
		{oldStatus: "received", newStatus: "wont_fix", want: true},
	}
	for _, test := range tests {
		if got := canAdvanceSecurityReport(test.oldStatus, test.newStatus); got != test.want {
			t.Errorf("canAdvanceSecurityReport(%q, %q) = %t, want %t", test.oldStatus, test.newStatus, got, test.want)
		}
	}
}

func TestExtractMaintenanceSecurityPreservesOptionArguments(t *testing.T) {
	args := []string{"--maintenance", "security", "update", "sec_0123456789abcdef", "--comment", "Needs coordinated disclosure", "--config", "/tmp/airports-config"}
	found, action, target, remaining := extractMaintenanceFlag(args)
	if !found || action != "security" {
		t.Fatalf("extractMaintenanceFlag() found=%t action=%q", found, action)
	}
	wantTarget := strings.Join([]string{"update", "sec_0123456789abcdef", "--comment", "Needs coordinated disclosure"}, "\x00")
	if target != wantTarget {
		t.Fatalf("target = %q, want %q", target, wantTarget)
	}
	if !reflect.DeepEqual(remaining, []string{"--config", "/tmp/airports-config"}) {
		t.Fatalf("remaining = %#v", remaining)
	}
}

func TestSecurityMaintenanceListAndShow(t *testing.T) {
	root := newMainTestDir(t)
	databasePath := filepath.Join(root, "reports.db")
	database := openRecoveryTestDB(t, databasePath, false)
	closeSecurityTestResource(t, database.Close)
	_, err := database.Exec(`CREATE TABLE security_reports (
		tracking_id TEXT PRIMARY KEY, status TEXT NOT NULL, severity TEXT NOT NULL,
		component TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
		researcher_comment TEXT NOT NULL DEFAULT '', disclosure_date TEXT,
		credit_pref TEXT NOT NULL DEFAULT '')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO security_reports
		(tracking_id, status, severity, component, created_at, updated_at, researcher_comment, disclosure_date, credit_pref)
		VALUES ('sec_test', 'triaged', 'high', 'api', '2026-01-01', '2026-01-02', 'Patch is in progress', '2026-02-01', 'anon')`)
	if err != nil {
		t.Fatal(err)
	}
	list := captureStdout(t, func() {
		if err := listSecurityReports(database, "triaged"); err != nil {
			t.Errorf("listSecurityReports: %v", err)
		}
	})
	if !strings.Contains(list, "sec_test\ttriaged\thigh\tapi") {
		t.Fatalf("filtered report list = %q", list)
	}
	detail := captureStdout(t, func() {
		if err := showSecurityReport(database, "sec_test"); err != nil {
			t.Errorf("showSecurityReport: %v", err)
		}
	})
	for _, expected := range []string{"tracking_id: sec_test", "status: triaged", "researcher_comment: Patch is in progress", "disclosure_date: 2026-02-01"} {
		if !strings.Contains(detail, expected) {
			t.Errorf("report details omitted %q: %s", expected, detail)
		}
	}
	if err := showSecurityReport(database, "missing"); err == nil {
		t.Fatal("showSecurityReport accepted a missing report")
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, "config")
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Database.URL = databasePath
	if err := config.SaveApplicationConfig(filepath.Join(configDir, "server.yml"), cfg); err != nil {
		t.Fatal(err)
	}
	list = captureStdout(t, func() {
		if err := runMaintenanceSecurity(configDir, "list --status triaged"); err != nil {
			t.Errorf("runMaintenanceSecurity(list): %v", err)
		}
	})
	if !strings.Contains(list, "sec_test\ttriaged") {
		t.Fatalf("security maintenance list = %q", list)
	}
	detail = captureStdout(t, func() {
		if err := runMaintenanceSecurity(configDir, "show sec_test"); err != nil {
			t.Errorf("runMaintenanceSecurity(show): %v", err)
		}
	})
	if !strings.Contains(detail, "researcher_comment: Patch is in progress") {
		t.Fatalf("security maintenance details = %q", detail)
	}
}

func TestSecurityMaintenanceUpdateAppliesForwardChanges(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "security-update.db"), false)
	closeSecurityTestResource(t, database.Close)
	_, err := database.Exec(`CREATE TABLE security_reports (
		tracking_id TEXT PRIMARY KEY, status TEXT NOT NULL, credit_pref TEXT NOT NULL DEFAULT 'no',
		credit_name TEXT, severity TEXT NOT NULL, updated_at TEXT NOT NULL,
		researcher_comment TEXT NOT NULL DEFAULT '', disclosure_date TEXT, closed_at TEXT, contact_enc BLOB
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO security_reports(tracking_id, status, severity, updated_at) VALUES('sec_update', 'received', 'medium', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOG_DIR", filepath.Join(root, "logs"))
	comment := "Coordinated disclosure is in progress"
	date := "2026-12-01"
	severity := "high"
	status := "triaged"
	options := encodeSecurityMaintenanceOptions(securityMaintenanceOptions{Status: &status, Comment: &comment, DisclosureDate: &date, Severity: &severity})
	if err := updateSecurityReport(database, "sec_update", options, config.DefaultApplicationConfig(), root); err != nil {
		t.Fatalf("update security report: %v", err)
	}
	var gotStatus, gotComment, gotDate, gotSeverity string
	if err := database.QueryRow(`SELECT status, researcher_comment, disclosure_date, severity FROM security_reports WHERE tracking_id = 'sec_update'`).Scan(&gotStatus, &gotComment, &gotDate, &gotSeverity); err != nil {
		t.Fatal(err)
	}
	if gotStatus != status || gotComment != comment || gotDate != date || gotSeverity != severity {
		t.Fatalf("updated report = %q, %q, %q, %q", gotStatus, gotComment, gotDate, gotSeverity)
	}
	backward := "received"
	if err := updateSecurityReport(database, "sec_update", encodeSecurityMaintenanceOptions(securityMaintenanceOptions{Status: &backward}), config.DefaultApplicationConfig(), root); err == nil || !strings.Contains(err.Error(), "cannot move backward") {
		t.Fatalf("backward status update error = %v", err)
	}
}

func TestSecurityMaintenanceUpdateRejectsMissingAndInvalidReports(t *testing.T) {
	root := newMainTestDir(t)
	database := openRecoveryTestDB(t, filepath.Join(root, "security-update-errors.db"), false)
	closeSecurityTestResource(t, database.Close)
	if _, err := database.Exec(`CREATE TABLE security_reports (
		tracking_id TEXT PRIMARY KEY, status TEXT NOT NULL, credit_pref TEXT NOT NULL DEFAULT 'no',
		credit_name TEXT, severity TEXT NOT NULL, updated_at TEXT NOT NULL,
		researcher_comment TEXT NOT NULL DEFAULT '', disclosure_date TEXT, closed_at TEXT, contact_enc BLOB
	)`); err != nil {
		t.Fatal(err)
	}
	if err := updateSecurityReport(database, "sec_missing", `{}`, config.DefaultApplicationConfig(), root); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing report update error = %v", err)
	}
	if _, err := database.Exec(`INSERT INTO security_reports(tracking_id, status, severity, updated_at) VALUES('sec_invalid', 'received', 'low', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	invalid := "received"
	if err := updateSecurityReport(database, "sec_invalid", encodeSecurityMaintenanceOptions(securityMaintenanceOptions{Status: &invalid}), config.DefaultApplicationConfig(), root); err == nil || !strings.Contains(err.Error(), "cannot move") {
		t.Fatalf("invalid report status update error = %v", err)
	}
}

func TestLoadInstallationSecretIsStableAndValidated(t *testing.T) {
	database := openRecoveryTestDB(t, filepath.Join(newMainTestDir(t), "installation.db"), false)
	closeSecurityTestResource(t, database.Close)
	first, err := loadInstallationSecret(database)
	if err != nil || len(first) != 32 {
		t.Fatalf("first loadInstallationSecret() = %d bytes, %v", len(first), err)
	}
	second, err := loadInstallationSecret(database)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("second installation secret changed: equal=%t err=%v", reflect.DeepEqual(first, second), err)
	}
}
