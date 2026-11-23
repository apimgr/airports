package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	if busyTimeout != 10000 {
		t.Errorf("expected busy_timeout 10000, got %d", busyTimeout)
	}
}

func TestOpenInitializesGeneratedSecrets(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "server.db")
	first, err := Open(config.DatabaseConfig{URL: dsn}, "airports-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureAppSecrets(first); err != nil {
		_ = first.Close()
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range []string{"installation_secret", "cookie_signing_key", "csrf_token_secret"} {
		var value, updated string
		if err := first.QueryRow(`SELECT value, updated_at FROM app_secrets WHERE name = ?`, name).Scan(&value, &updated); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("%s value is not a base64-encoded 32-byte secret", name)
		}
		if _, err := time.Parse(time.RFC3339Nano, updated); err != nil {
			t.Fatalf("%s updated_at is not UTC ISO 8601: %q: %v", name, updated, err)
		}
		values[name] = value
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(config.DatabaseConfig{URL: dsn}, "airports-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	for name, want := range values {
		var got string
		if err := second.QueryRow(`SELECT value FROM app_secrets WHERE name = ?`, name).Scan(&got); err != nil {
			t.Fatalf("read persisted %s: %v", name, err)
		}
		if got != want {
			t.Errorf("reopen replaced generated secret %s", name)
		}
	}
}

func TestOpenMigratesLegacyBinarySecrets(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{0xa5}, 32)
	if _, err := legacy.Exec(`CREATE TABLE app_secrets (name TEXT PRIMARY KEY, value BLOB NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`INSERT INTO app_secrets(name, value) VALUES('installation_secret', ?)`, secret); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := Open(config.DatabaseConfig{URL: dsn}, "airports-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureAppSecrets(database); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	var stored, updated string
	if err := database.QueryRow(`SELECT value, updated_at FROM app_secrets WHERE name = 'installation_secret'`).Scan(&stored, &updated); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(DecodeAppSecret("installation_secret", []byte(stored)), secret) {
		t.Fatal("legacy secret was not preserved during base64 migration")
	}
	if updated == "" {
		t.Fatal("legacy secret has no updated_at timestamp")
	}
}

func TestStoreSeparatesReadAndWriteHandles(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "server.db"), 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	if err := store.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY, value TEXT NOT NULL)`)
		return err
	}); err != nil {
		t.Fatalf("create sample table: %v", err)
	}
	if err := store.Read(context.Background(), func(ctx context.Context, readDB *sql.DB) error {
		var busyTimeout int
		if err := readDB.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			return err
		}
		if busyTimeout != 10000 {
			t.Fatalf("read handle busy_timeout = %d, want 10000", busyTimeout)
		}
		var synchronous int
		if err := readDB.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
			return err
		}
		if synchronous != 1 {
			t.Fatalf("read handle synchronous = %d, want NORMAL (1)", synchronous)
		}
		_, err := readDB.ExecContext(ctx, `INSERT INTO sample(value) VALUES('forbidden')`)
		if err == nil {
			t.Fatal("read-only handle accepted a write")
		}
		return nil
	}); err != nil {
		t.Fatalf("read callback: %v", err)
	}
	if err := store.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO sample(value) VALUES('written')`)
		return err
	}); err != nil {
		t.Fatalf("write callback: %v", err)
	}
}

func TestStoreConcurrentReadersAndWriters(t *testing.T) {
	if testing.Short() {
		t.Skip("10-second SQLite concurrency contract test")
	}
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "concurrency.db"), 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	if err := store.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE writes (id INTEGER PRIMARY KEY, worker INTEGER NOT NULL)`)
		return err
	}); err != nil {
		t.Fatalf("create writes table: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var failures atomic.Int64
	var workers sync.WaitGroup
	for worker := 0; worker < 20; worker++ {
		workers.Add(2)
		go func(worker int) {
			defer workers.Done()
			for ctx.Err() == nil {
				err := store.Write(ctx, func(tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, `INSERT INTO writes(worker) VALUES(?)`, worker)
					return err
				})
				if err != nil && ctx.Err() == nil {
					failures.Add(1)
					return
				}
			}
		}(worker)
		go func() {
			defer workers.Done()
			for ctx.Err() == nil {
				err := store.Read(ctx, func(readCtx context.Context, readDB *sql.DB) error {
					var count int64
					return readDB.QueryRowContext(readCtx, `SELECT COUNT(*) FROM writes`).Scan(&count)
				})
				if err != nil && ctx.Err() == nil {
					failures.Add(1)
					return
				}
			}
		}()
	}
	workers.Wait()
	if got := failures.Load(); got != 0 {
		t.Fatalf("concurrent SQLite workload had %d errors", got)
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

func TestStoreHandleRoutesQueriesThroughSeparatePools(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "handle.db"), 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.Exec(`CREATE TABLE handle_values (value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(context.Background(), `INSERT INTO handle_values(value) VALUES(?)`, "stored"); err != nil {
		t.Fatal(err)
	}
	row := store.QueryRow(`SELECT value FROM handle_values`)
	var value string
	if err := row.Scan(&value); err != nil || value != "stored" {
		t.Fatalf("QueryRow value = %q, err = %v", value, err)
	}
	rows, err := store.QueryContext(context.Background(), `SELECT value FROM handle_values`)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Scan(&value) != nil || value != "stored" {
		t.Fatalf("QueryContext returned value %q", value)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := store.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("combined Stats.MaxOpenConnections = %d, want 3", got)
	}
	// Production callers receive no raw transaction handle; writes go through Store.Write.
	if err := store.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE handle_values SET value = value`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeDatabaseDriver(t *testing.T) {
	for input, want := range map[string]string{"": "sqlite", "SQLite3": "sqlite", "turso": "libsql", "LIBSQL": "libsql", "postgres": ""} {
		if got := normalizeDatabaseDriver(input); got != want {
			t.Errorf("normalizeDatabaseDriver(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLibSQLURLAddsSeparateTokenWithoutOverwritingURLToken(t *testing.T) {
	for _, test := range []struct{ raw, token, want string }{
		{"https://db.example", "secret value", "https://db.example?authToken=secret+value"},
		{"libsql://db.example?authToken=url-token", "separate-token", "libsql://db.example?authToken=url-token"},
	} {
		got, err := libSQLURL(test.raw, test.token)
		if err != nil || got != test.want {
			t.Errorf("libSQLURL(%q) = %q, %v; want %q", test.raw, got, err, test.want)
		}
	}
	if _, err := libSQLURL("", "token"); err == nil {
		t.Fatal("empty URL accepted")
	}
}

func TestRewriteTableNamesOnlyForRemoteDatabase(t *testing.T) {
	query := `CREATE INDEX IF NOT EXISTS idx_security_reports_status ON security_reports(status); SELECT name FROM sqlite_master WHERE name = 'security_reports'`
	if got := rewriteTableNames(query, false); got != query {
		t.Fatalf("SQLite query was rewritten: %s", got)
	}
	want := `CREATE INDEX IF NOT EXISTS srv_idx_security_reports_status ON srv_security_reports(status); SELECT name FROM sqlite_master WHERE name = 'srv_security_reports'`
	if got := rewriteTableNames(query, true); got != want {
		t.Fatalf("remote query = %q, want %q", got, want)
	}
}

func TestDatabaseObjectName(t *testing.T) {
	for _, test := range []struct {
		remote     bool
		name, want string
	}{
		{false, "rate_limits", "rate_limits"},
		{true, "rate_limits", "srv_rate_limits"},
		{true, "srv_rate_limits", "srv_rate_limits"},
	} {
		if got := DatabaseObjectName(test.remote, test.name); got != test.want {
			t.Errorf("DatabaseObjectName(%v, %q) = %q, want %q", test.remote, test.name, got, test.want)
		}
	}
}
