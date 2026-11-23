// Package db opens the shared SQLite or remote libSQL database described in AI.md
// PART 10 "Database" — schema is created via idempotent CREATE TABLE IF NOT
// EXISTS statements owned by each consuming package, never a migrations
// table. Driver is modernc.org/sqlite (pure Go, CGO_ENABLED=0-safe) per
// project-rules.md.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/path"
)

// Open opens a database/sql handle for compatibility callers. New production
// code should use OpenStore so queries receive driver-specific handling.
func Open(cfg config.DatabaseConfig, projectName string) (*sql.DB, error) {
	driver := normalizeDatabaseDriver(cfg.Driver)
	dsn, err := databaseDSN(cfg, projectName)
	if err != nil {
		return nil, err
	}
	var sqlDB *sql.DB
	switch driver {
	case "libsql":
		dsn, err = libSQLURL(dsn, cfg.Token)
		if err == nil {
			sqlDB, err = sql.Open("libsql", dsn)
		}
	case "sqlite":
		sqlDB, err = openSQLiteHandle(dsn, true)
	default:
		return nil, fmt.Errorf("db: unsupported driver %q", cfg.Driver)
	}
	if err != nil {
		return nil, err
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
	return sqlDB, nil
}

func databaseDSN(cfg config.DatabaseConfig, projectName string) (string, error) {
	dsn := strings.TrimSpace(cfg.URL)
	if dsn != "" {
		return dsn, nil
	}
	// DATABASE_DIR (AI.md PART 8 "Init-Only Variables") overrides the
	// OS/privilege-appropriate default SQLite directory.
	dir := os.Getenv("DATABASE_DIR")
	if dir == "" {
		dir = paths.GetSQLiteDBPath(projectName)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("db: create db dir %s: %w", dir, err)
	}
	return filepath.Join(dir, "server.db"), nil
}

func normalizeDatabaseDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite2", "sqlite3":
		return "sqlite"
	case "libsql", "turso":
		return "libsql"
	default:
		return ""
	}
}

func isSQLiteDriver(driver string) bool {
	return normalizeDatabaseDriver(driver) == "sqlite"
}

func isLibSQLDriver(driver string) bool {
	return normalizeDatabaseDriver(driver) == "libsql"
}

func libSQLURL(raw, token string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("db: libsql driver requires a remote URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("db: invalid libsql URL: %w", err)
	}
	if parsed.Scheme != "libsql" && parsed.Scheme != "https" {
		return "", fmt.Errorf("db: libsql driver requires libsql:// or https:// URL")
	}
	if token != "" {
		query := parsed.Query()
		if query.Get("authToken") == "" {
			query.Set("authToken", token)
			parsed.RawQuery = query.Encode()
		}
	}
	return parsed.String(), nil
}

func openSQLiteHandle(dsn string, write bool) (*sql.DB, error) {
	resolved, err := sqliteDSN(dsn, write)
	if err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, fmt.Errorf("db: open SQLite database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, errorsJoinClose(sqlDB, fmt.Errorf("db: ping SQLite database: %w", err))
	}
	if write && !isSQLiteMemoryDSN(dsn) {
		var mode string
		if err := sqlDB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
			return nil, errorsJoinClose(sqlDB, fmt.Errorf("db: read journal mode: %w", err))
		}
		if !strings.EqualFold(mode, "wal") {
			log.Printf("ERROR db.open journal_mode=%q; refusing SQLite path %s because WAL is required", mode, dsn)
			return nil, errorsJoinClose(sqlDB, fmt.Errorf("db: WAL mode unavailable for %s (got %q)", dsn, mode))
		}
	}
	return sqlDB, nil
}

// isSQLiteMemoryDSN identifies SQLite databases that do not persist a WAL file.
func isSQLiteMemoryDSN(dsn string) bool {
	value := strings.ToLower(strings.TrimSpace(dsn))
	return value == ":memory:" || strings.Contains(value, ":memory:") || strings.Contains(value, "mode=memory")
}

func errorsJoinClose(db *sql.DB, err error) error {
	if closeErr := db.Close(); closeErr != nil {
		return fmt.Errorf("%w (close database: %v)", err, closeErr)
	}
	return err
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
