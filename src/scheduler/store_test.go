package scheduler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestEnsureSchemaUsesCanonicalSchedulerColumns(t *testing.T) {
	db := testDB(t)
	for table, want := range map[string][]string{
		"scheduler_tasks":   {"id", "name", "schedule", "last_run", "next_run"},
		"scheduler_history": {"task_id", "started_at", "finished_at", "status", "duration_ms", "error"},
	} {
		rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatalf("read %s schema: %v", table, err)
		}
		columns := map[string]bool{}
		for rows.Next() {
			var cid, notNull, primary int
			var name, kind string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
				_ = rows.Close()
				t.Fatalf("scan %s schema: %v", table, err)
			}
			columns[name] = true
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close %s schema rows: %v", table, err)
		}
		for _, name := range want {
			if !columns[name] {
				t.Errorf("%s is missing canonical column %q", table, name)
			}
		}
	}
	for table, legacy := range map[string][]string{
		"scheduler_tasks":   {"task_id", "task_name"},
		"scheduler_history": {"ran_at"},
	} {
		rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatalf("read %s schema: %v", table, err)
		}
		columns := map[string]bool{}
		for rows.Next() {
			var cid, notNull, primary int
			var name, kind string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primary); err != nil {
				_ = rows.Close()
				t.Fatalf("scan %s schema: %v", table, err)
			}
			columns[name] = true
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close %s schema rows: %v", table, err)
		}
		for _, name := range legacy {
			if columns[name] {
				t.Errorf("%s unexpectedly creates legacy column %q", table, name)
			}
		}
	}
}

func TestEnsureSchemaPreservesExistingSchedulerState(t *testing.T) {
	db, err := sql.Open("sqlite", "file:scheduler_legacy_state?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE scheduler_tasks (task_id TEXT PRIMARY KEY, task_name TEXT NOT NULL, schedule TEXT NOT NULL, last_run TEXT, last_status TEXT, last_error TEXT, next_run TEXT, run_count INTEGER NOT NULL DEFAULT 0, fail_count INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1)`,
		`INSERT INTO scheduler_tasks (task_id, task_name, schedule, last_run, last_status, last_error, next_run, run_count, fail_count, enabled) VALUES ('legacy-task', 'Legacy task', '@daily', '2026-11-20T02:00:00Z', 'success', '', '2026-11-21T02:00:00Z', 4, 1, 0)`,
		`CREATE TABLE scheduler_history (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, ran_at TEXT NOT NULL, status TEXT NOT NULL, duration_ms INTEGER NOT NULL, error TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO scheduler_history (task_id, ran_at, status, duration_ms, error) VALUES ('legacy-task', '2026-11-20T02:00:00Z', 'success', 1200, '')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("prepare legacy scheduler state: %v", err)
		}
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	state, err := loadTaskState(db, "legacy-task")
	if err != nil {
		t.Fatalf("load migrated task state: %v", err)
	}
	if state.Name != "Legacy task" || state.RunCount != 4 || state.FailCount != 1 || state.Enabled {
		t.Errorf("task state was not preserved: %+v", state)
	}
	if state.LastRun.Unix() != 1795140000 || state.NextRun.Unix() != 1795226400 {
		t.Errorf("task timestamps were not converted to Unix seconds: last=%v next=%v", state.LastRun, state.NextRun)
	}
	for table, legacy := range map[string][]string{
		"scheduler_tasks":   {"task_id", "task_name"},
		"scheduler_history": {"ran_at"},
	} {
		columns, err := schedulerTableColumns(context.Background(), db, table)
		if err != nil {
			t.Fatalf("read migrated %s schema: %v", table, err)
		}
		for _, name := range legacy {
			if columns[name] {
				t.Errorf("migrated %s still has legacy column %q", table, name)
			}
		}
	}
	history, err := listHistory(db, "legacy-task", 10)
	if err != nil {
		t.Fatalf("list migrated history: %v", err)
	}
	if len(history) != 1 || history[0].RanAt.Unix() != 1795140000 || history[0].DurationMS != 1200 {
		t.Errorf("history was not preserved: %+v", history)
	}
	var finishedAt int64
	if err := db.QueryRow(`SELECT finished_at FROM scheduler_history WHERE task_id = ?`, "legacy-task").Scan(&finishedAt); err != nil {
		t.Fatalf("read migrated history finish time: %v", err)
	}
	if finishedAt != 1795140001 {
		t.Errorf("finished_at = %d, want %d", finishedAt, 1795140001)
	}
}

// TestQueryTimeoutTiers guards the store.go timeout constants against drift
// from the AI.md PART 10 "Query Timeouts" table: Simple SELECT 5s,
// INSERT/UPDATE/DELETE 10s, Migrations 5m.
func TestQueryTimeoutTiers(t *testing.T) {
	if simpleSelectTimeout != 5*time.Second {
		t.Errorf("simpleSelectTimeout = %v, want 5s per AI.md PART 10", simpleSelectTimeout)
	}
	if writeTimeout != 10*time.Second {
		t.Errorf("writeTimeout = %v, want 10s per AI.md PART 10", writeTimeout)
	}
	if migrationTimeout != 5*time.Minute {
		t.Errorf("migrationTimeout = %v, want 5m per AI.md PART 10", migrationTimeout)
	}
}

// TestStoreFunctions_RespectContextTimeouts exercises every store.go
// function end-to-end to confirm the *Context query variants introduced for
// AI.md PART 10 compliance still behave correctly under normal operation
// (no regression from adding bounded contexts).
func TestStoreFunctions_RespectContextTimeouts(t *testing.T) {
	db := testDB(t)

	if err := upsertTaskDefault(db, "t1", "Task One", "@every 1h", true); err != nil {
		t.Fatalf("upsertTaskDefault: %v", err)
	}

	st, err := loadTaskState(db, "t1")
	if err != nil {
		t.Fatalf("loadTaskState: %v", err)
	}
	if st.ID != "t1" || st.Name != "Task One" {
		t.Fatalf("loadTaskState returned unexpected state: %+v", st)
	}

	if err := updateNextRun(db, "t1", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("updateNextRun: %v", err)
	}

	if err := recordRun(db, "t1", "success", "", time.Now(), time.Now().Add(time.Hour), 42); err != nil {
		t.Fatalf("recordRun: %v", err)
	}

	states, err := listTaskStates(db)
	if err != nil {
		t.Fatalf("listTaskStates: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("listTaskStates returned %d states, want 1", len(states))
	}

	if _, err := setEnabled(db, "t1", false); err != nil {
		t.Fatalf("setEnabled: %v", err)
	}

	history, err := listHistory(db, "t1", 10)
	if err != nil {
		t.Fatalf("listHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("listHistory returned %d entries, want 1", len(history))
	}
}
