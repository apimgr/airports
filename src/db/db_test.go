package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestOpen_UnsupportedDriver(t *testing.T) {
	_, err := Open(config.DatabaseConfig{Driver: "postgres"}, "airports-test")
	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}
}

func TestOpen_SQLiteWithURL(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "server.db")

	sqlDB, err := Open(config.DatabaseConfig{Driver: "sqlite", URL: dsn}, "airports-test")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	if _, err := os.Stat(dsn); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}

	var mode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode failed: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected WAL journal mode, got %q", mode)
	}

	var busyTimeout int
	if err := sqlDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout failed: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy_timeout 5000, got %d", busyTimeout)
	}
}

func TestOpen_DefaultDriverIsSQLite(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "server2.db")

	sqlDB, err := Open(config.DatabaseConfig{URL: dsn}, "airports-test")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
}

func TestOpen_PoolDefaultsWhenUnset(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "server3.db")

	sqlDB, err := Open(config.DatabaseConfig{Driver: "sqlite", URL: dsn}, "airports-test")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 4 {
		t.Errorf("expected default MaxOpenConnections 4, got %d", stats.MaxOpenConnections)
	}
}

func TestOpen_PoolConfigHonored(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "server4.db")

	sqlDB, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		URL:    dsn,
		Pool: config.DatabasePoolConfig{
			MaxOpen:     10,
			MaxIdle:     3,
			MaxLifetime: "1h",
			MaxIdleTime: "30s",
		},
	}, "airports-test")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("expected MaxOpenConnections 10, got %d", stats.MaxOpenConnections)
	}
}

func TestOpen_PoolConfigInvalidDurationFallsBack(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "server5.db")

	sqlDB, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		URL:    dsn,
		Pool: config.DatabasePoolConfig{
			MaxLifetime: "not-a-duration",
		},
	}, "airports-test")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
}
