package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
)

const (
	writeTimeout       = 10 * time.Second
	readTimeout        = 15 * time.Second
	transactionTimeout = 30 * time.Second
	busyRetryAttempts  = 5
)

// Store owns separate SQLite read and write pools for one database file.
// Callers must use Read and Write instead of receiving either raw handle.
type Store struct {
	w      *sql.DB
	r      *sql.DB
	remote bool
}

// OpenStore opens separate read and write handles for the configured SQLite
// database. The write handle has exactly one connection; read connections are
// query-only and use the configured read-pool size.
func OpenStore(cfg config.DatabaseConfig, projectName string) (*Store, error) {
	if isLibSQLDriver(cfg.Driver) {
		dsn := strings.TrimSpace(cfg.URL)
		if dsn == "" {
			return nil, errors.New("db: libsql driver requires a remote URL")
		}
		var err error
		dsn, err = libSQLURL(dsn, cfg.Token)
		if err != nil {
			return nil, err
		}
		database, err := sql.Open("libsql", dsn)
		if err != nil {
			return nil, fmt.Errorf("db: open libsql database: %w", err)
		}
		maxOpen := cfg.Pool.MaxOpen
		if maxOpen <= 0 {
			maxOpen = 4
		}
		maxIdle := cfg.Pool.MaxIdle
		if maxIdle <= 0 || maxIdle > maxOpen {
			maxIdle = maxOpen
		}
		database.SetMaxOpenConns(maxOpen)
		database.SetMaxIdleConns(maxIdle)
		database.SetConnMaxLifetime(parsePoolDuration(cfg.Pool.MaxLifetime, 5*time.Minute))
		database.SetConnMaxIdleTime(parsePoolDuration(cfg.Pool.MaxIdleTime, time.Minute))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = database.PingContext(ctx)
		cancel()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("db: connect to libsql: %w", err), database.Close())
		}
		store := &Store{w: database, r: database, remote: true}
		if err := initializeAppSecrets(store); err != nil {
			return nil, errors.Join(fmt.Errorf("db: initialize application secrets: %w", err), database.Close())
		}
		return store, nil
	}
	if !isSQLiteDriver(cfg.Driver) {
		return nil, fmt.Errorf("db: unsupported driver %q", cfg.Driver)
	}
	dsn, err := databaseDSN(cfg, projectName)
	if err != nil {
		return nil, err
	}
	readers := cfg.Pool.MaxOpen
	if readers <= 0 {
		readers = 4
	}
	maxIdle := cfg.Pool.MaxIdle
	if maxIdle <= 0 || maxIdle > readers {
		maxIdle = readers
	}
	maxLifetime := parsePoolDuration(cfg.Pool.MaxLifetime, 5*time.Minute)
	maxIdleTime := parsePoolDuration(cfg.Pool.MaxIdleTime, time.Minute)
	return OpenSQLiteWithPool(dsn, readers, maxIdle, maxLifetime, maxIdleTime)
}

// OpenSQLite opens the write and read handles of one SQLite file.
func OpenSQLite(path string, readers int) (*Store, error) {
	if readers <= 0 {
		readers = 4
	}
	return OpenSQLiteWithPool(path, readers, readers, 5*time.Minute, time.Minute)
}

// OpenSQLiteWithPool opens both handles using the configured read-pool limits.
func OpenSQLiteWithPool(path string, readers, maxIdle int, maxLifetime, maxIdleTime time.Duration) (*Store, error) {
	if readers <= 0 {
		readers = 4
	}
	if maxIdle < 0 || maxIdle > readers {
		maxIdle = readers
	}
	writeDB, err := openSQLiteHandle(path, true)
	if err != nil {
		return nil, err
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)
	writeDB.SetConnMaxLifetime(0)
	writeDB.SetConnMaxIdleTime(0)

	readDB, err := openSQLiteHandle(path, false)
	if err != nil {
		return nil, errors.Join(err, writeDB.Close())
	}
	readDB.SetMaxOpenConns(readers)
	readDB.SetMaxIdleConns(maxIdle)
	readDB.SetConnMaxLifetime(maxLifetime)
	readDB.SetConnMaxIdleTime(maxIdleTime)
	if err := initializeAppSecrets(writeDB); err != nil {
		return nil, errors.Join(fmt.Errorf("db: initialize application secrets: %w", err), readDB.Close(), writeDB.Close())
	}
	return &Store{w: writeDB, r: readDB}, nil
}

// Read runs fn with the query-only read pool and a bounded context.
func (s *Store) Read(ctx context.Context, fn func(context.Context, *sql.DB) error) error {
	if s == nil || s.r == nil {
		return errors.New("db: read from a closed store")
	}
	if fn == nil {
		return errors.New("db: nil read callback")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(ctx, s.r)
}

// Exec runs a single write statement inside the store's retried write path.
func (s *Store) Exec(query string, args ...any) (sql.Result, error) {
	return s.ExecContext(context.Background(), query, args...)
}

// ExecContext runs a single write statement inside the store's retried write path.
func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := s.Write(ctx, func(tx *sql.Tx) error {
		var err error
		result, err = tx.Exec(rewriteTableNames(query, s.remote), args...)
		return err
	})
	return result, err
}

// Query runs a bounded read query through the query-only pool.
func (s *Store) Query(query string, args ...any) (*sql.Rows, error) {
	return s.QueryContext(context.Background(), query, args...)
}

// QueryContext runs a bounded read query through the query-only pool.
func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s == nil || s.r == nil {
		return nil, errors.New("db: query from a closed store")
	}
	queryCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(readTimeout, cancel)
	rows, err := s.r.QueryContext(queryCtx, rewriteTableNames(query, s.remote), args...)
	if err != nil {
		cancel()
		return nil, err
	}
	return rows, nil
}

// QueryRow runs a bounded single-row query through the query-only pool.
func (s *Store) QueryRow(query string, args ...any) *sql.Row {
	return s.QueryRowContext(context.Background(), query, args...)
}

// QueryRowContext runs a bounded single-row query through the query-only pool.
func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if s == nil || s.r == nil {
		return (&sql.DB{}).QueryRowContext(ctx, query, args...)
	}
	queryCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(readTimeout, cancel)
	return s.r.QueryRowContext(queryCtx, rewriteTableNames(query, s.remote), args...)
}

// PingContext checks connectivity through the read-only pool.
func (s *Store) PingContext(ctx context.Context) error {
	if s == nil || s.r == nil {
		return errors.New("db: ping a closed store")
	}
	pingCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return s.r.PingContext(pingCtx)
}

// Stats combines connection statistics from the read and write pools.
func (s *Store) Stats() sql.DBStats {
	if s == nil {
		return sql.DBStats{}
	}
	readStats, writeStats := sql.DBStats{}, sql.DBStats{}
	if s.r != nil {
		readStats = s.r.Stats()
	}
	if s.w != nil {
		writeStats = s.w.Stats()
	}
	readStats.MaxOpenConnections += writeStats.MaxOpenConnections
	readStats.OpenConnections += writeStats.OpenConnections
	readStats.InUse += writeStats.InUse
	readStats.Idle += writeStats.Idle
	readStats.WaitCount += writeStats.WaitCount
	readStats.WaitDuration += writeStats.WaitDuration
	readStats.MaxIdleClosed += writeStats.MaxIdleClosed
	readStats.MaxIdleTimeClosed += writeStats.MaxIdleTimeClosed
	readStats.MaxLifetimeClosed += writeStats.MaxLifetimeClosed
	return readStats
}

// WriteMapped runs a retried write transaction and supplies the database namespace mapper.
func (s *Store) WriteMapped(ctx context.Context, fn func(*sql.Tx, func(string) string) error) error {
	if fn == nil {
		return errors.New("db: nil mapped write callback")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		return fn(tx, func(query string) string { return rewriteTableNames(query, s.remote) })
	})
}

// Write runs fn in one transaction and retries busy failures up to five
// attempts within a ten-second context deadline.
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	if s == nil || s.w == nil {
		return errors.New("db: write to a closed store")
	}
	if fn == nil {
		return errors.New("db: nil write callback")
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	caller := callingFunction(2)
	var totalWait time.Duration
	var busyRetries int
	defer func() { ObserveWriteWait(totalWait, busyRetries) }()
	var err error
	for attempt := 0; attempt < busyRetryAttempts; attempt++ {
		var retry bool
		var wait time.Duration
		retry, wait, err = s.writeOnce(ctx, fn)
		totalWait += wait
		if retry && isBusy(err) {
			busyRetries++
		}
		if !retry || !isBusy(err) {
			if totalWait > time.Second {
				log.Printf("WARN db.busy wait_ms=%d caller=%q", totalWait.Milliseconds(), caller)
			}
			return err
		}
		if attempt+1 == busyRetryAttempts {
			break
		}
		backoff := time.Duration(25*(1<<attempt))*time.Millisecond + time.Duration(rand.IntN(25))*time.Millisecond
		totalWait += backoff
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if totalWait > time.Second {
		log.Printf("WARN db.busy wait_ms=%d caller=%q", totalWait.Milliseconds(), caller)
	}
	return err
}

func (s *Store) writeOnce(ctx context.Context, fn func(*sql.Tx) error) (bool, time.Duration, error) {
	started := time.Now()
	tx, err := s.w.BeginTx(ctx, nil)
	lockWait := time.Since(started)
	if err != nil {
		return true, lockWait, err
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
		return true, lockWait, err
	}
	// A commit error is never replayed: the caller cannot safely know whether
	// the server committed before the connection reported an error.
	if err := tx.Commit(); err != nil {
		return false, lockWait, err
	}
	return false, lockWait, nil
}

// Close checkpoints the WAL, closes read connections, then closes the writer.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var checkpointErr error
	if s.w != nil && !s.remote {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, checkpointErr = s.w.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
		cancel()
	}
	var readErr, writeErr error
	if s.r != nil {
		readErr = s.r.Close()
	}
	if s.w != nil && s.w != s.r {
		writeErr = s.w.Close()
	}
	return errors.Join(checkpointErr, readErr, writeErr)
}

// Snapshot copies a consistent SQLite snapshot to a new database file using
// VACUUM INTO on the serialized write handle.
func (s *Store) Snapshot(ctx context.Context, destination string) error {
	if s == nil || s.w == nil {
		return errors.New("db: snapshot from a closed store")
	}
	if s.remote {
		return errors.New("db: SQLite snapshot is unavailable for remote libsql databases")
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("db: snapshot destination is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if _, err := s.w.ExecContext(ctx, `VACUUM INTO ?`, destination); err != nil {
		return fmt.Errorf("db: create SQLite snapshot: %w", err)
	}
	return nil
}

var serverTableAndIndexNames = []string{
	"api_tokens", "app_secrets", "captcha_challenges", "compliance_events", "owners",
	"pgp_metadata", "pgp_rotation_journal", "rate_limits", "scheduler_history", "scheduler_tasks",
	"security_reports", "security_thanks", "idx_api_tokens_active", "idx_api_tokens_hash",
	"idx_api_tokens_owner", "idx_api_tokens_prefix", "idx_rate_limits_window", "idx_scheduler_history_task", "idx_scheduler_history_started", "idx_scheduler_tasks_id",
	"idx_security_reports_status",
}

// DatabaseObjectName applies the single database namespace rule to table and index names.
func DatabaseObjectName(remote bool, name string) string {
	if remote && !strings.HasPrefix(name, "srv_") {
		return "srv_" + name
	}
	return name
}

func rewriteTableNames(query string, remote bool) string {
	if !remote {
		return query
	}
	for _, name := range serverTableAndIndexNames {
		query = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(name)+`\b`).ReplaceAllString(query, DatabaseObjectName(remote, name))
	}
	return query
}

func sqliteDSN(raw string, write bool) (string, error) {
	raw = strings.TrimSpace(raw)
	var parsed *url.URL
	var err error
	if raw == ":memory:" {
		parsed = &url.URL{Scheme: "file", Opaque: ":memory:"}
	} else if strings.HasPrefix(strings.ToLower(raw), "file:") {
		parsed, err = url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("db: invalid SQLite URL: %w", err)
		}
	} else {
		parsed = &url.URL{Scheme: "file", Path: raw}
	}
	query := parsed.Query()
	query.Add("_pragma", "busy_timeout(10000)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")
	query.Add("_pragma", "foreign_keys(1)")
	if !write {
		query.Add("_pragma", "query_only(1)")
	} else {
		query.Set("_txlock", "immediate")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "SQLITE_BUSY") || strings.Contains(message, "SQLITE_LOCKED") || strings.Contains(message, "DATABASE IS LOCKED")
}

func callingFunction(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	function := runtime.FuncForPC(pc)
	if function == nil {
		return "unknown"
	}
	return function.Name()
}
