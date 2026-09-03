package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newFakeSourceTree creates a minimal ConfigDir/DataDir pair with a real
// server.yml and a real (valid) SQLite server.db, suitable for Create/Verify
// round-trip tests.
func newFakeSourceTree(t *testing.T) (configDir, dataDir string) {
	t.Helper()
	root := testTempDir(t)
	configDir = filepath.Join(root, "config")
	dataDir = filepath.Join(root, "data", "db", "sqlite")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll config: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll data: %v", err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write server.yml: %v", err)
	}

	dbPath := filepath.Join(dataDir, "server.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT)"); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('foo', 'bar')"); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	// dataDir returned to caller is the DB's grandparent (the actual data
	// root), matching findServerDB's expected {data_dir}/db/sqlite layout.
	dataDir = filepath.Join(root, "data")
	return configDir, dataDir
}

func TestCreateAndVerify_Unencrypted(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	dest := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")

	err := Create(dest, CreateOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		AppVersion: "1.2.3",
		CreatedBy:  "test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Verify(dest, ""); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestCreateAndVerify_Encrypted(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	dest := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz.enc")

	err := Create(dest, CreateOptions{
		ConfigDir:          configDir,
		DataDir:            dataDir,
		AppVersion:         "1.2.3",
		EncryptionPassword: "s3cret-pw",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Verify(dest, "s3cret-pw"); err != nil {
		t.Fatalf("Verify with correct password: %v", err)
	}
	if err := Verify(dest, "wrong-pw"); err == nil {
		t.Fatalf("expected Verify to fail with wrong password")
	}
	if err := Verify(dest, ""); err == nil {
		t.Fatalf("expected Verify to fail with no password on encrypted backup")
	}
}

func TestCreate_ComplianceRequiresPassword(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	dest := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")

	err := Create(dest, CreateOptions{
		ConfigDir:         configDir,
		DataDir:           dataDir,
		ComplianceEnabled: true,
	})
	if err != ErrEncryptionRequired {
		t.Fatalf("expected ErrEncryptionRequired, got %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("expected no file to be written, stat err = %v", statErr)
	}
}

func TestVerify_FileNotExist(t *testing.T) {
	if err := Verify("/tmp/apimgr/does-not-exist-airports-backup.tar.gz", ""); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestVerify_EmptyFile(t *testing.T) {
	dir := testTempDir(t)
	p := filepath.Join(dir, "empty.tar.gz")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := Verify(p, ""); err == nil {
		t.Fatalf("expected error for empty file")
	}
}

func TestVerify_BadMagic(t *testing.T) {
	dir := testTempDir(t)
	p := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(p, []byte("not a real backup container at all"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if err := Verify(p, ""); err == nil {
		t.Fatalf("expected error for bad magic/manifest")
	}
}

func TestVerify_ChecksumMismatch(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	destDir := testTempDir(t)
	dest := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")

	if err := Create(dest, CreateOptions{ConfigDir: configDir, DataDir: dataDir}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	manifest, payload, err := readContainer(dest)
	if err != nil {
		t.Fatalf("readContainer: %v", err)
	}
	// Corrupt the payload so the stored checksum no longer matches.
	corrupted := append([]byte(nil), payload...)
	corrupted[0] ^= 0xFF
	manifestJSON, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := writeContainer(dest, manifestJSON, corrupted); err != nil {
		t.Fatalf("writeContainer: %v", err)
	}

	if err := Verify(dest, ""); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}

func TestVerify_CorruptDatabase(t *testing.T) {
	configDir, dataDir := newFakeSourceTree(t)
	// Overwrite the valid sqlite file with garbage bytes so extraction
	// succeeds but the integrity check fails.
	dbPath := filepath.Join(dataDir, "db", "sqlite", "server.db")
	if err := os.WriteFile(dbPath, []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatalf("corrupt db: %v", err)
	}

	destDir := testTempDir(t)
	dest := filepath.Join(destDir, "airports_backup_2025-01-15.tar.gz")
	if err := Create(dest, CreateOptions{ConfigDir: configDir, DataDir: dataDir}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Verify(dest, ""); err == nil {
		t.Fatalf("expected database integrity failure")
	}
}

func TestApplyRetention_PriorityExample(t *testing.T) {
	dir := testTempDir(t)

	// Mirrors AI.md PART 21's "Keep 1 weekly + 1 monthly + 1 yearly" example:
	// yesterday's daily, last Sunday's weekly, Jan-1's monthly+yearly, plus
	// the always-kept incremental.
	makeFile := func(name string, size int) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	makeFile("airports_backup_2025-01-15.tar.gz", 100) // daily (Wednesday)
	makeFile("airports_backup_2025-01-12.tar.gz", 100) // weekly (Sunday)
	makeFile("airports_backup_2025-01-01.tar.gz", 100) // monthly + yearly (Jan 1)
	makeFile("airports-daily.tar.gz", 50)              // incremental

	cfg := RetentionConfig{
		MaxBackups:  1,
		KeepWeekly:  1,
		KeepMonthly: 1,
		KeepYearly:  1,
	}

	deleted, err := ApplyRetention(dir, "airports", cfg)
	if err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected nothing deleted (all 4 files within retention), got %v", deleted)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 files remaining, got %d", len(entries))
	}
}

func TestApplyRetention_PrunesOldestDaily(t *testing.T) {
	dir := testTempDir(t)

	makeFile := func(name string) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Three daily-only (non Sunday, non 1st-of-month) backups; only the
	// newest max_backups=1 should survive.
	makeFile("airports_backup_2025-01-14.tar.gz")
	makeFile("airports_backup_2025-01-15.tar.gz")
	makeFile("airports_backup_2025-01-16.tar.gz")

	deleted, err := ApplyRetention(dir, "airports", RetentionConfig{MaxBackups: 1})
	if err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted, got %v", deleted)
	}

	remaining, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Name() != "airports_backup_2025-01-16.tar.gz" {
		t.Fatalf("expected only the newest daily to remain, got %v", remaining)
	}
}

func TestApplyRetention_DisabledTierFallsBackToDaily(t *testing.T) {
	// Regression test: a backup landing on a weekly/monthly/yearly boundary
	// date must not be discarded just because that tier's keep-count is 0
	// (disabled) — it should fall back into the always-on daily bucket
	// instead of being treated as belonging to a zero-capacity bucket.
	cases := []struct {
		name     string
		fileName string
		cfg      RetentionConfig
	}{
		{"weekly-disabled", "airports_backup_2025-01-12.tar.gz", RetentionConfig{MaxBackups: 1, KeepWeekly: 0}},   // Sunday
		{"monthly-disabled", "airports_backup_2025-02-01.tar.gz", RetentionConfig{MaxBackups: 1, KeepMonthly: 0}}, // 1st of month, not Jan 1
		{"yearly-disabled", "airports_backup_2025-01-01.tar.gz", RetentionConfig{MaxBackups: 1, KeepYearly: 0}},   // Jan 1
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := testTempDir(t)
			p := filepath.Join(dir, tc.fileName)
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatalf("write %s: %v", tc.fileName, err)
			}

			deleted, err := ApplyRetention(dir, "airports", tc.cfg)
			if err != nil {
				t.Fatalf("ApplyRetention: %v", err)
			}
			if len(deleted) != 0 {
				t.Fatalf("expected the sole backup to survive via daily fallback, got deleted: %v", deleted)
			}

			remaining, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir: %v", err)
			}
			if len(remaining) != 1 || remaining[0].Name() != tc.fileName {
				t.Fatalf("expected %s to remain, got %v", tc.fileName, remaining)
			}
		})
	}
}

func TestApplyRetention_MaxTotalSize(t *testing.T) {
	dir := testTempDir(t)

	makeFile := func(name string, size int, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		mtime := time.Now().Add(-age)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatalf("Chtimes %s: %v", name, err)
		}
	}

	makeFile("airports_backup_2025-01-13.tar.gz", 1000, 3*24*time.Hour)
	makeFile("airports_backup_2025-01-14.tar.gz", 1000, 2*24*time.Hour)
	makeFile("airports_backup_2025-01-15.tar.gz", 1000, 1*24*time.Hour)

	deleted, err := ApplyRetention(dir, "airports", RetentionConfig{
		MaxBackups:   10,
		MaxTotalSize: 1500,
	})
	if err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	if len(deleted) == 0 {
		t.Fatalf("expected size-cap pruning to delete at least one file")
	}

	remaining, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var total int64
	for _, e := range remaining {
		info, _ := e.Info()
		total += info.Size()
	}
	if total > 1500 {
		t.Fatalf("expected remaining total <= 1500 bytes, got %d", total)
	}
}

func TestParseMaxTotalSize_Falsey(t *testing.T) {
	dir := testTempDir(t)
	for _, v := range []string{"0", "false", "no", "none", "disable", "disabled", "off", ""} {
		got, err := parseMaxTotalSize(v, dir)
		if err != nil {
			t.Fatalf("parseMaxTotalSize(%q): unexpected error: %v", v, err)
		}
		if got != 0 {
			t.Fatalf("parseMaxTotalSize(%q) = %d, want 0", v, got)
		}
	}
}

func TestParseMaxTotalSize_AbsoluteSizes(t *testing.T) {
	dir := testTempDir(t)
	cases := map[string]int64{
		"1024": 1024,
		"1K":   1024,
		"1KB":  1024,
		"50M":  50 * 1 << 20,
		"2G":   2 * 1 << 30,
	}
	for input, want := range cases {
		got, err := parseMaxTotalSize(input, dir)
		if err != nil {
			t.Fatalf("parseMaxTotalSize(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseMaxTotalSize(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseMaxTotalSize_Percent(t *testing.T) {
	dir := testTempDir(t)
	got, err := parseMaxTotalSize("10%", dir)
	if err != nil {
		t.Fatalf("parseMaxTotalSize percent: %v", err)
	}
	if got <= 0 {
		t.Fatalf("expected positive byte count for 10%%, got %d", got)
	}
}
