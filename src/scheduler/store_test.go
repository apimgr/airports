package scheduler

import (
	"testing"
	"time"
)

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
