package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/scheduler"
)

func TestMaintenanceRecoveryIntervalsAndWait(t *testing.T) {
	for _, raw := range []string{"", "invalid", "0s", "-1s"} {
		if got := maintenanceRetryInterval(raw); got != 30*time.Second {
			t.Errorf("maintenanceRetryInterval(%q) = %s, want 30s", raw, got)
		}
	}
	if got := maintenanceRetryInterval("250ms"); got != 250*time.Millisecond {
		t.Fatalf("maintenanceRetryInterval(valid) = %s, want 250ms", got)
	}
	if !waitForRecoveryTick(context.Background(), time.Millisecond) {
		t.Fatal("waitForRecoveryTick returned false after timer fired")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForRecoveryTick(ctx, time.Hour) {
		t.Fatal("waitForRecoveryTick returned true after cancellation")
	}
}

func TestMaintenanceRecoveryConfigAndLifecycle(t *testing.T) {
	if (*maintenanceRecovery)(nil).currentConfig() != nil {
		t.Fatal("nil receiver currentConfig should return nil")
	}
	(*maintenanceRecovery)(nil).stop()

	first := config.DefaultConfig()
	recovery := &maintenanceRecovery{cfg: first}
	if recovery.currentConfig() != first {
		t.Fatal("currentConfig did not return the active config")
	}
	recovery.updateConfig(nil)
	if recovery.currentConfig() != first {
		t.Fatal("updateConfig(nil) replaced the active config")
	}
	second := config.DefaultConfig()
	recovery.updateConfig(second)
	if recovery.currentConfig() != second {
		t.Fatal("updateConfig did not replace the active config")
	}

	dir := newMainTestDir(t)
	dbPath := filepath.Join(dir, "server.db")
	dbHandle := openRecoveryTestDB(t, dbPath, false)
	defer func() { _ = dbHandle.Close() }()
	started := startMaintenanceRecovery(second, dbHandle, dir, dir, dir, dir, nil, true)
	started.stop()
	if started.currentConfig() != second {
		t.Fatal("stopped recovery lost its config")
	}
}

func TestMaintenanceRecoveryProbes(t *testing.T) {
	if err := probeDatabaseConnection(nil); err == nil {
		t.Fatal("probeDatabaseConnection(nil) succeeded")
	}
	if err := probeDatabaseReadWrite(nil); err == nil {
		t.Fatal("probeDatabaseReadWrite(nil) succeeded")
	}
	if err := probeWritableDirectory(""); err == nil {
		t.Fatal("probeWritableDirectory(empty) succeeded")
	}
	if err := probeWritableDirectory(filepath.Join(newMainTestDir(t), "missing")); err == nil {
		t.Fatal("probeWritableDirectory(nonexistent) succeeded")
	}
	if err := probeWritableDirectory(newMainTestDir(t)); err != nil {
		t.Fatalf("probeWritableDirectory(tempdir): %v", err)
	}

	dir := newMainTestDir(t)
	dbPath := filepath.Join(dir, "server.db")
	withoutSchema := openRecoveryTestDB(t, dbPath, false)
	if err := probeDatabaseConnection(withoutSchema); err != nil {
		t.Fatalf("probeDatabaseConnection(open db): %v", err)
	}
	if err := probeDatabaseReadWrite(withoutSchema); err == nil {
		t.Fatal("probeDatabaseReadWrite succeeded without scheduler schema")
	}
	_ = withoutSchema.Close()

	withSchema := openRecoveryTestDB(t, filepath.Join(dir, "schema.db"), true)
	defer func() { _ = withSchema.Close() }()
	if err := probeDatabaseReadWrite(withSchema); err != nil {
		t.Fatalf("probeDatabaseReadWrite(with schema): %v", err)
	}
}

func TestMaintenanceRecoveryDetectionAndRecovery(t *testing.T) {
	dir := newMainTestDir(t)
	cfg := config.DefaultConfig()
	cfg.Server.Maintenance.Cleanup.DiskThreshold = 0
	recovery := &maintenanceRecovery{
		dataDir:   dir,
		configDir: dir,
		logsDir:   dir,
		backupDir: dir,
	}
	if reason, err := recovery.detectCriticalFailure(cfg); reason != "database_connection" || err == nil {
		t.Fatalf("detectCriticalFailure(nil db) = %q, %v", reason, err)
	}

	dbPath := filepath.Join(dir, "server.db")
	recovery.db = openRecoveryTestDB(t, dbPath, false)
	if reason, err := recovery.detectCriticalFailure(cfg); reason != "database_write" || err == nil {
		t.Fatalf("detectCriticalFailure(no schema) = %q, %v", reason, err)
	}
	_ = recovery.db.Close()

	recovery.db = openRecoveryTestDB(t, filepath.Join(dir, "ready.db"), true)
	defer func() { _ = recovery.db.Close() }()
	if reason, err := recovery.detectCriticalFailure(cfg); reason != "" || err != nil {
		t.Fatalf("detectCriticalFailure(healthy) = %q, %v", reason, err)
	}

	recovery.configDir = filepath.Join(dir, "missing")
	if reason, err := recovery.detectCriticalFailure(cfg); reason != "file_permission" || err == nil {
		t.Fatalf("detectCriticalFailure(unwritable path) = %q, %v", reason, err)
	}
	recovery.configDir = dir

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := recovery.recoverOnce(ctx, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("recoverOnce(canceled) = %v, want context.Canceled", err)
	}
	if err := recovery.recoverOnce(context.Background(), cfg); err != nil {
		t.Fatalf("recoverOnce(healthy) = %v", err)
	}
	if err := recovery.recoverDatabase(context.Background()); err != nil {
		t.Fatalf("recoverDatabase(healthy) = %v", err)
	}
}

func TestMaintenanceRecoveryNotificationNoop(t *testing.T) {
	recovery := &maintenanceRecovery{}
	cfg := config.DefaultConfig()
	recovery.sendNotification(cfg, "maintenance_entered", false, "database_connection", 0)
	recovery.sendNotification(cfg, "maintenance_entered", true, "database_connection", time.Second)
}

func TestServerDatabaseIsEmptyReadsSQLiteRows(t *testing.T) {
	dir := newMainTestDir(t)
	missing := filepath.Join(dir, "missing.db")
	if empty, err := serverDatabaseIsEmpty(missing); err != nil || !empty {
		t.Fatalf("missing database = %v, %v; want empty", empty, err)
	}

	dbPath := filepath.Join(dir, "server.db")
	database := openRecoveryTestDB(t, dbPath, false)
	if _, err := database.Exec(`CREATE TABLE records (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close empty test database: %v", err)
	}
	if empty, err := serverDatabaseIsEmpty(dbPath); err != nil || !empty {
		t.Fatalf("schema-only database = %v, %v; want empty", empty, err)
	}

	database = openRecoveryTestDB(t, dbPath, false)
	if _, err := database.Exec(`INSERT INTO records (id) VALUES (1)`); err != nil {
		t.Fatalf("insert test row: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close populated test database: %v", err)
	}
	if empty, err := serverDatabaseIsEmpty(dbPath); err != nil || empty {
		t.Fatalf("populated database = %v, %v; want non-empty", empty, err)
	}
}

func openRecoveryTestDB(t *testing.T, path string, withSchema bool) *sql.DB {
	t.Helper()
	database, err := db.Open(config.DatabaseConfig{URL: path}, ProjectName)
	if err != nil {
		t.Fatalf("open recovery test database: %v", err)
	}
	if withSchema {
		if err := scheduler.EnsureSchema(database); err != nil {
			_ = database.Close()
			t.Fatalf("initialize recovery test schema: %v", err)
		}
	}
	return database
}
