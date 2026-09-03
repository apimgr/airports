package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestWriter(t *testing.T, cfg FileConfig) (*LogWriter, string) {
	t.Helper()
	dir := t.TempDir()
	return NewWriter(dir, "test", cfg), dir
}

func TestWriteCreatesFileAndAppends(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "never", Keep: "none"}
	w, _ := newTestWriter(t, cfg)

	if err := w.Write("first line"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Write("second line\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	want := "first line\nsecond line\n"
	if string(data) != want {
		t.Errorf("log file content = %q, want %q", string(data), want)
	}
}

func TestWriteDisabledIsNoop(t *testing.T) {
	cfg := FileConfig{Enabled: false, Filename: "debug.log", Rotate: "never", Keep: "none"}
	w, _ := newTestWriter(t, cfg)

	if err := w.Write("should not be written"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(w.Path()); !os.IsNotExist(err) {
		t.Errorf("expected no log file to be created when disabled, stat err = %v", err)
	}
}

func TestRotateIfDueSizeThreshold(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "access.log", Rotate: "never,1KB", Keep: "none"}
	w, dir := newTestWriter(t, cfg)

	if err := w.Write(strings.Repeat("x", 2048)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rotated, err := w.RotateIfDue(time.Now())
	if err != nil {
		t.Fatalf("RotateIfDue: %v", err)
	}
	if !rotated {
		t.Fatalf("expected rotation to trigger on size threshold")
	}

	if _, err := os.Stat(w.Path()); !os.IsNotExist(err) {
		t.Errorf("expected active log file to be gone after keep:none rotation")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "access.log" {
			t.Errorf("unexpected leftover file %q after keep:none rotation", e.Name())
		}
	}
}

func TestRotateIfDueNotYetDue(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "weekly", Keep: "none"}
	w, _ := newTestWriter(t, cfg)

	if err := w.Write("hello"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w.lastRotation = time.Now()

	rotated, err := w.RotateIfDue(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateIfDue: %v", err)
	}
	if rotated {
		t.Errorf("expected no rotation before the weekly interval elapses")
	}
}

func TestRotateIfDueAgeThreshold(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "daily", Keep: "none"}
	w, _ := newTestWriter(t, cfg)

	if err := w.Write("hello"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w.lastRotation = time.Now().Add(-48 * time.Hour)

	rotated, err := w.RotateIfDue(time.Now())
	if err != nil {
		t.Fatalf("RotateIfDue: %v", err)
	}
	if !rotated {
		t.Errorf("expected rotation to trigger once daily interval has elapsed")
	}
}

func TestRotateMissingLogFileIsNoop(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "never-written.log", Rotate: "daily", Keep: "none"}
	w, _ := newTestWriter(t, cfg)

	rotated, err := w.RotateIfDue(time.Now())
	if err != nil {
		t.Fatalf("RotateIfDue: %v", err)
	}
	if rotated {
		t.Errorf("expected no rotation for a log file that was never written")
	}
}

func TestRotateKeepCountRetention(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "never,1B", Keep: "2"}
	w, dir := newTestWriter(t, cfg)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		if err := w.Write("line"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if _, err := w.RotateIfDue(base.Add(time.Duration(i) * time.Hour)); err != nil {
			t.Fatalf("RotateIfDue #%d: %v", i, err)
		}
	}

	backups, err := w.listRotatedBackups()
	if err != nil {
		t.Fatalf("listRotatedBackups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 retained backups, got %d: %v", len(backups), backups)
	}
	_ = dir
}

func TestRotateKeepDaysRetention(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "never", Keep: "1d"}
	w, dir := newTestWriter(t, cfg)

	oldBackup := filepath.Join(dir, "server.log.old")
	if err := os.WriteFile(oldBackup, []byte("old"), 0640); err != nil {
		t.Fatalf("write old backup: %v", err)
	}
	oldTime := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldBackup, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := w.applyRetention(RetentionPolicy{Mode: RetentionDays, N: 1}); err != nil {
		t.Fatalf("applyRetention: %v", err)
	}

	if _, err := os.Stat(oldBackup); !os.IsNotExist(err) {
		t.Errorf("expected old backup to be pruned, stat err = %v", err)
	}
}

func TestRotateForeverRetentionKeepsEverything(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "never", Keep: "forever"}
	w, dir := newTestWriter(t, cfg)

	backup := filepath.Join(dir, "server.log.old")
	if err := os.WriteFile(backup, []byte("old"), 0640); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := w.applyRetention(RetentionPolicy{Mode: RetentionForever}); err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup to survive forever retention, stat err = %v", err)
	}
}

func TestRotateCompressesWhenConfigured(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "audit.log", Rotate: "never,1B", Keep: "forever", Compress: true}
	w, dir := newTestWriter(t, cfg)

	content := strings.Repeat("audit-event\n", 100)
	if err := w.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rotated, err := w.RotateIfDue(time.Now())
	if err != nil {
		t.Fatalf("RotateIfDue: %v", err)
	}
	if !rotated {
		t.Fatalf("expected rotation to trigger")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var gzPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			gzPath = filepath.Join(dir, e.Name())
		}
	}
	if gzPath == "" {
		t.Fatalf("expected a .gz rotated backup, found none in %v", entries)
	}

	f, err := os.Open(gzPath)
	if err != nil {
		t.Fatalf("open gz file: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gz contents: %v", err)
	}
	if string(got) != content {
		t.Errorf("decompressed content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}

func TestConcurrentWrites(t *testing.T) {
	cfg := FileConfig{Enabled: true, Filename: "server.log", Rotate: "never", Keep: "none"}
	w, _ := newTestWriter(t, cfg)
	defer w.Close()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 20; j++ {
				_ = w.Write("concurrent line")
			}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Count(string(data), "\n")
	if lines != 200 {
		t.Errorf("expected 200 lines from concurrent writers, got %d", lines)
	}
}
