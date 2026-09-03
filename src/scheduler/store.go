package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

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

// timeLayout is the persisted timestamp format — RFC3339 in UTC, matching
// the rest of the codebase's audit-log timestamp convention.
const timeLayout = time.RFC3339

// EnsureSchema creates the scheduler's persistent-state tables per AI.md
// PART 18 "Scheduler State (Persistent)". Safe to call on every startup —
// CREATE TABLE IF NOT EXISTS only, no migrations table.
func EnsureSchema(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS scheduler_tasks (
			task_id     TEXT PRIMARY KEY,
			task_name   TEXT NOT NULL,
			schedule    TEXT NOT NULL,
			last_run    TEXT,
			last_status TEXT NOT NULL DEFAULT '',
			last_error  TEXT NOT NULL DEFAULT '',
			next_run    TEXT,
			run_count   INTEGER NOT NULL DEFAULT 0,
			fail_count  INTEGER NOT NULL DEFAULT 0,
			enabled     INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS scheduler_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id     TEXT NOT NULL,
			ran_at      TEXT NOT NULL,
			status      TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			error       TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_history_task ON scheduler_history(task_id, ran_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("scheduler: ensure schema: %w", err)
		}
	}
	return nil
}

// upsertTaskDefault inserts a task's row on first sight, leaving the
// persisted `enabled` flag untouched on subsequent starts so CLI-driven
// enable/disable survives restarts.
func upsertTaskDefault(db *sql.DB, id, name, schedule string, enabledDefault bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`INSERT INTO scheduler_tasks (task_id, task_name, schedule, enabled) VALUES (?, ?, ?, ?)
		 ON CONFLICT(task_id) DO UPDATE SET task_name = excluded.task_name, schedule = excluded.schedule`,
		id, name, schedule, boolToInt(enabledDefault),
	)
	return err
}

func loadTaskState(db *sql.DB, id string) (TaskState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	row := db.QueryRowContext(ctx,
		`SELECT task_id, task_name, schedule, last_run, last_status, last_error,
		        next_run, run_count, fail_count, enabled
		 FROM scheduler_tasks WHERE task_id = ?`, id,
	)
	return scanTaskState(row)
}

func listTaskStates(db *sql.DB) ([]TaskState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT task_id, task_name, schedule, last_run, last_status, last_error,
		        next_run, run_count, fail_count, enabled
		 FROM scheduler_tasks ORDER BY task_id`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []TaskState
	for rows.Next() {
		st, err := scanTaskState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanTaskState works for both.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskState(row rowScanner) (TaskState, error) {
	var (
		st              TaskState
		lastRun         sql.NullString
		nextRun         sql.NullString
		enabledInt      int
		lastStatus      string
		lastError       string
	)
	if err := row.Scan(&st.ID, &st.Name, &st.Schedule, &lastRun, &lastStatus, &lastError,
		&nextRun, &st.RunCount, &st.FailCount, &enabledInt); err != nil {
		return TaskState{}, err
	}
	st.LastStatus = lastStatus
	st.LastError = lastError
	st.Enabled = enabledInt != 0
	if lastRun.Valid && lastRun.String != "" {
		if t, err := time.Parse(timeLayout, lastRun.String); err == nil {
			st.LastRun = t
		}
	}
	if nextRun.Valid && nextRun.String != "" {
		if t, err := time.Parse(timeLayout, nextRun.String); err == nil {
			st.NextRun = t
		}
	}
	return st, nil
}

func updateNextRun(db *sql.DB, id string, next time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	_, err := db.ExecContext(ctx, `UPDATE scheduler_tasks SET next_run = ? WHERE task_id = ?`, formatTime(next), id)
	return err
}

// recordRun performs three related writes (task-row update, history insert,
// history trim) — bounded by a single write-tier context shared across all
// three, per AI.md PART 10's INSERT/UPDATE/DELETE timeout tier.
func recordRun(db *sql.DB, id, status, errMsg string, ranAt time.Time, next time.Time, durationMS int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`UPDATE scheduler_tasks
		 SET last_run = ?, last_status = ?, last_error = ?, next_run = ?,
		     run_count = run_count + CASE WHEN ? = 'success' THEN 1 ELSE 0 END,
		     fail_count = fail_count + CASE WHEN ? = 'failed' THEN 1 ELSE 0 END
		 WHERE task_id = ?`,
		formatTime(ranAt), status, errMsg, formatTime(next), status, status, id,
	)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO scheduler_history (task_id, ran_at, status, duration_ms, error) VALUES (?, ?, ?, ?, ?)`,
		id, formatTime(ranAt), status, durationMS, errMsg,
	)
	if err != nil {
		return err
	}
	// Keep only the most recent 100 history rows per task.
	_, err = db.ExecContext(ctx,
		`DELETE FROM scheduler_history WHERE task_id = ? AND id NOT IN (
			SELECT id FROM scheduler_history WHERE task_id = ? ORDER BY ran_at DESC LIMIT 100
		)`, id, id,
	)
	return err
}

func setEnabled(db *sql.DB, id string, enabled bool) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	return db.ExecContext(ctx, `UPDATE scheduler_tasks SET enabled = ? WHERE task_id = ?`, boolToInt(enabled), id)
}

func listHistory(db *sql.DB, id string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	ctx, cancel := context.WithTimeout(context.Background(), simpleSelectTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT ran_at, status, duration_ms, error FROM scheduler_history
		 WHERE task_id = ? ORDER BY ran_at DESC LIMIT ?`, id, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []HistoryEntry
	for rows.Next() {
		var (
			h      HistoryEntry
			ranAt  string
			errMsg string
		)
		if err := rows.Scan(&ranAt, &h.Status, &h.DurationMS, &errMsg); err != nil {
			return nil, err
		}
		h.Error = errMsg
		if t, err := time.Parse(timeLayout, ranAt); err == nil {
			h.RanAt = t
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
