// Package db opens the shared SQLite "server.db" database described in AI.md
// PART 10 "Database" — schema is created via idempotent CREATE TABLE IF NOT
// EXISTS statements owned by each consuming package, never a migrations
// table. Driver is modernc.org/sqlite (pure Go, CGO_ENABLED=0-safe) per
// project-rules.md.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/path"
)

// Open connects to the configured database and enables WAL mode for
// single-writer/multi-reader concurrency (SQLite is the default driver; a
// non-sqlite Driver value is rejected since airports only ships a SQLite
// implementation today).
func Open(cfg config.DatabaseConfig, projectName string) (*sql.DB, error) {
	if cfg.Driver != "" && cfg.Driver != "sqlite" {
		return nil, fmt.Errorf("db: unsupported driver %q (only sqlite is implemented)", cfg.Driver)
	}

	dsn := cfg.URL
	if dsn == "" {
		// DATABASE_DIR (AI.md PART 8 "Init-Only Variables") overrides the
		// OS/privilege-appropriate default SQLite directory.
		dir := os.Getenv("DATABASE_DIR")
		if dir == "" {
			dir = paths.GetSQLiteDBPath(projectName)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("db: create db dir %s: %w", dir, err)
		}
		dsn = filepath.Join(dir, "server.db")
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", dsn, err)
	}

	// SQLite is single-writer — keep the pool small to avoid SQLITE_BUSY
	// contention (retried at the call site rather than the pool level).
	// Values come from config.DatabaseConfig.Pool (AI.md PART 10 "Connection
	// Pooling"), falling back to the documented 4/4/5m/1m defaults when unset.
	maxOpen := cfg.Pool.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 4
	}
	maxIdle := cfg.Pool.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 4
	}
	maxLifetime := parsePoolDuration(cfg.Pool.MaxLifetime, 5*time.Minute)
	maxIdleTime := parsePoolDuration(cfg.Pool.MaxIdleTime, time.Minute)

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(maxLifetime)
	sqlDB.SetConnMaxIdleTime(maxIdleTime)

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: enable WAL mode: %w", err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: set busy_timeout: %w", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: enable foreign_keys: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: ping %s: %w", dsn, err)
	}

	return sqlDB, nil
}

// parsePoolDuration parses a pool config duration string, falling back to
// def when raw is empty or invalid.
func parsePoolDuration(raw string, def time.Duration) time.Duration {
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}
