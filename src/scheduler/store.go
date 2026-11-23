package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/apimgr/airports/src/common/diagnostics"
)

var queryObserver func(string, string, time.Time, error)

func SetQueryObserver(observer func(string, string, time.Time, error)) {
	queryObserver = observer
}

func observeQuery(operation, table string, started time.Time, err error) {
	if queryObserver != nil {
		queryObserver(operation, table, started, err)
	}
}

// Query timeout tiers per AI.md PART 10 "Query Timeouts" — this package has
// no request-scoped context to thread through (the scheduler runs on
// background ticks), so each query derives its own bounded context from
// context.Background() at the point of use, matching the AI.md
// "use parent context or create new" guidance.
const (
	simpleSelectTimeout = 5 * time.Second
	writeTimeout        = 10 * time.Second
	migrationTimeout    = 5 * time.Minute
)

// EnsureSchema creates the scheduler's persistent-state tables per AI.md
// PART 18 "Scheduler State (Persistent)" and applies idempotent schema
// updates without a migrations table.
func EnsureSchema(database Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS scheduler_tasks (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			schedule    TEXT NOT NULL,
			last_run    INTEGER,
			last_status TEXT NOT NULL DEFAULT '',
			last_error  TEXT NOT NULL DEFAULT '',
			next_run    INTEGER,
			run_count   INTEGER NOT NULL DEFAULT 0,
			fail_count  INTEGER NOT NULL DEFAULT 0,
			enabled     INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS scheduler_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id     TEXT NOT NULL,
			started_at  INTEGER NOT NULL,
			finished_at INTEGER,
			status      TEXT NOT NULL,
			error       TEXT,
			duration_ms INTEGER
		)`,
	}
	for _, stmt := range stmts {
		started := time.Now()
		result, err := database.ExecContext(ctx, stmt)
		logExecQuery(stmt, nil, started, result, err)
		if err != nil {
			return fmt.Errorf("scheduler: ensure schema: %w", err)
		}
	}
	for table, renames := range map[string][][2]string{
		"scheduler_tasks":   {{"task_id", "id"}, {"task_name", "name"}},
		"scheduler_history": {{"ran_at", "started_at"}},
	} {
		columns, err := schedulerTableColumns(ctx, database, table)
		if err != nil {
			return err
		}
		for _, rename := range renames {
			if columns[rename[0]] && !columns[rename[1]] {
				stmt := `ALTER TABLE ` + table + ` RENAME COLUMN ` + rename[0] + ` TO ` + rename[1]
				if _, err := database.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("scheduler: rename %s.%s to %s: %w", table, rename[0], rename[1], err)
				}
				delete(columns, rename[0])
				columns[rename[1]] = true
			}
		}
		for _, column := range []struct{ name, definition string }{
			{"id", "id TEXT"},
			{"name", "name TEXT NOT NULL DEFAULT ''"},
			{"started_at", "started_at INTEGER NOT NULL DEFAULT 0"},
			{"finished_at", "finished_at INTEGER"},
		} {
			if columns[column.name] {
				continue
			}
			if table == "scheduler_tasks" && column.name != "id" && column.name != "name" {
				continue
			}
			if table == "scheduler_history" && column.name != "started_at" && column.name != "finished_at" {
				continue
			}
			stmt := `ALTER TABLE ` + table + ` ADD COLUMN ` + column.definition
			if _, err := database.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("scheduler: add %s.%s: %w", table, column.name, err)
			}
		}
	}
	for _, stmt := range []string{
		`UPDATE scheduler_history SET started_at = CAST(strftime('%s', started_at) AS INTEGER) WHERE typeof(started_at) = 'text' AND instr(started_at, '-') > 0`,
		`UPDATE scheduler_history SET finished_at = started_at + COALESCE(duration_ms, 0) / 1000 WHERE finished_at IS NULL AND duration_ms IS NOT NULL`,
		`UPDATE scheduler_tasks SET last_run = CAST(strftime('%s', last_run) AS INTEGER) WHERE last_run IS NOT NULL AND instr(last_run, '-') > 0`,
		`UPDATE scheduler_tasks SET next_run = CAST(strftime('%s', next_run) AS INTEGER) WHERE next_run IS NOT NULL AND instr(next_run, '-') > 0`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_history_task ON scheduler_history(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_history_started ON scheduler_history(started_at)`,
	} {
		if _, err := database.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func schedulerTableColumns(ctx context.Context, database Database, table string) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primary int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, rows.Close()
}

// upsertTaskDefault inserts a task's row on first sight, leaving the
// persisted `enabled` flag untouched on subsequent starts so CLI-driven
// enable/disable survives restarts.
func upsertTaskDefault(database Database, id, name, schedule string, enabledDefault bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	started := time.Now()
	query := `INSERT INTO scheduler_tasks (id, name, schedule, enabled) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, schedule = excluded.schedule`
	args := []any{id, name, schedule, boolToInt(enabledDefault)}
	result, err := database.ExecContext(ctx, query, args...)
	logExecQuery(query, args, started, result, err)
	observeQuery("insert", "scheduler_tasks", started, err)
	return err
}

func loadTaskState(database Database, id string) (TaskState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	started := time.Now()
	query := `SELECT id, name, schedule, last_run, last_status, last_error,
		        next_run, run_count, fail_count, enabled
		 FROM scheduler_tasks WHERE id = ?`
	row := database.QueryRowContext(ctx, query, id)
	state, err := scanTaskState(row)
	rows := int64(0)
	if err == nil {
		rows = 1
	}
	diagnostics.LogQuery(query, 1, started, rows, err)
	observeQuery("select", "scheduler_tasks", started, err)
	return state, err
}

func listTaskStates(database Database) ([]TaskState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	started := time.Now()
	query := `SELECT id, name, schedule, last_run, last_status, last_error,
		        next_run, run_count, fail_count, enabled
		 FROM scheduler_tasks ORDER BY id`
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		diagnostics.LogQuery(query, 0, started, 0, err)
		observeQuery("select", "scheduler_tasks", started, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []TaskState
	for rows.Next() {
		st, err := scanTaskState(rows)
		if err != nil {
			diagnostics.LogQuery(query, 0, started, int64(len(out)), err)
			observeQuery("select", "scheduler_tasks", started, err)
			return nil, err
		}
		out = append(out, st)
	}
	err = rows.Err()
	diagnostics.LogQuery(query, 0, started, int64(len(out)), err)
	observeQuery("select", "scheduler_tasks", started, err)
	return out, err
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanTaskState works for both.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskState(row rowScanner) (TaskState, error) {
	var (
		st         TaskState
		lastRun    sql.NullInt64
		nextRun    sql.NullInt64
		enabledInt int
		lastStatus string
		lastError  string
	)
	if err := row.Scan(&st.ID, &st.Name, &st.Schedule, &lastRun, &lastStatus, &lastError,
		&nextRun, &st.RunCount, &st.FailCount, &enabledInt); err != nil {
		return TaskState{}, err
	}
	st.LastStatus = lastStatus
	st.LastError = lastError
	st.Enabled = enabledInt != 0
	if lastRun.Valid {
		st.LastRun = time.Unix(lastRun.Int64, 0).UTC()
	}
	if nextRun.Valid {
		st.NextRun = time.Unix(nextRun.Int64, 0).UTC()
	}
	return st, nil
}

func updateNextRun(database Database, id string, next time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	started := time.Now()
	query := `UPDATE scheduler_tasks SET next_run = ? WHERE id = ?`
	args := []any{next.Unix(), id}
	result, err := database.ExecContext(ctx, query, args...)
	logExecQuery(query, args, started, result, err)
	observeQuery("update", "scheduler_tasks", started, err)
	return err
}

// recordRun performs three related writes (task-row update, history insert,
// history trim) — bounded by a single write-tier context shared across all
// three, per AI.md PART 10's INSERT/UPDATE/DELETE timeout tier.
func recordRun(database Database, id, status, errMsg string, ranAt time.Time, next time.Time, durationMS int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	run := func(tx *sql.Tx, mapQuery func(string) string) error {
		started := time.Now()
		query := `UPDATE scheduler_tasks
			 SET last_run = ?, last_status = ?, last_error = ?, next_run = ?,
			     run_count = run_count + CASE WHEN ? = 'success' THEN 1 ELSE 0 END,
			     fail_count = fail_count + CASE WHEN ? = 'failed' THEN 1 ELSE 0 END
			 WHERE id = ?`
		args := []any{ranAt.Unix(), status, errMsg, next.Unix(), status, status, id}
		result, err := tx.ExecContext(ctx, mapQuery(query), args...)
		logExecQuery(query, args, started, result, err)
		observeQuery("update", "scheduler_tasks", started, err)
		if err != nil {
			return err
		}
		started = time.Now()
		query = `INSERT INTO scheduler_history (task_id, started_at, finished_at, status, duration_ms, error) VALUES (?, ?, ?, ?, ?, ?)`
		args = []any{id, ranAt.Unix(), ranAt.Add(time.Duration(durationMS) * time.Millisecond).Unix(), status, durationMS, errMsg}
		result, err = tx.ExecContext(ctx, mapQuery(query), args...)
		logExecQuery(query, args, started, result, err)
		observeQuery("insert", "scheduler_history", started, err)
		if err != nil {
			return err
		}
		started = time.Now()
		query = `DELETE FROM scheduler_history WHERE task_id = ? AND id NOT IN (
			SELECT id FROM scheduler_history WHERE task_id = ? ORDER BY started_at DESC LIMIT 100
		)`
		args = []any{id, id}
		result, err = tx.ExecContext(ctx, mapQuery(query), args...)
		logExecQuery(query, args, started, result, err)
		observeQuery("delete", "scheduler_history", started, err)
		return err
	}
	if writer, ok := database.(interface {
		WriteMapped(context.Context, func(*sql.Tx, func(string) string) error) error
	}); ok {
		return writer.WriteMapped(ctx, run)
	}
	transactional, ok := database.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return fmt.Errorf("scheduler: database does not support transactions")
	}
	tx, err := transactional.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := run(tx, func(query string) string { return query }); err != nil {
		return err
	}
	return tx.Commit()
}

func setEnabled(database Database, id string, enabled bool) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	started := time.Now()
	query := `UPDATE scheduler_tasks SET enabled = ? WHERE id = ?`
	args := []any{boolToInt(enabled), id}
	result, err := database.ExecContext(ctx, query, args...)
	logExecQuery(query, args, started, result, err)
	observeQuery("update", "scheduler_tasks", started, err)
	return result, err
}

func listHistory(database Database, id string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	started := time.Now()
	query := `SELECT started_at, status, duration_ms, error FROM scheduler_history
		 WHERE task_id = ? ORDER BY started_at DESC LIMIT ?`
	args := []any{id, limit}
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		diagnostics.LogQuery(query, len(args), started, 0, err)
		observeQuery("select", "scheduler_history", started, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []HistoryEntry
	for rows.Next() {
		var (
			h      HistoryEntry
			ranAt  int64
			errMsg string
		)
		if err := rows.Scan(&ranAt, &h.Status, &h.DurationMS, &errMsg); err != nil {
			diagnostics.LogQuery(query, len(args), started, int64(len(out)), err)
			observeQuery("select", "scheduler_history", started, err)
			return nil, err
		}
		h.Error = errMsg
		h.RanAt = time.Unix(ranAt, 0).UTC()
		out = append(out, h)
	}
	err = rows.Err()
	diagnostics.LogQuery(query, len(args), started, int64(len(out)), err)
	observeQuery("select", "scheduler_history", started, err)
	return out, err
}

func logExecQuery(query string, args []any, started time.Time, result sql.Result, err error) {
	rows := int64(0)
	if result != nil {
		if affected, resultErr := result.RowsAffected(); resultErr == nil {
			rows = affected
		}
	}
	diagnostics.LogQuery(query, len(args), started, rows, err)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
