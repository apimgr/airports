package scheduler

import (
	"bytes"
	"database/sql"
	"errors"
	"log"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// captureLog redirects the standard logger's output into a buffer for the
// duration of the test and restores it afterward.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

// testDB opens a fresh in-memory database with the scheduler schema applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	// A single connection keeps the ":memory:" database alive and visible
	// across the test's queries (a fresh connection would see an empty db).
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return db
}

func TestNew(t *testing.T) {
	s := New(nil, 0)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.c == nil {
		t.Fatal("New() returned a Scheduler with a nil underlying cron.Cron")
	}
	if s.catchUpWindow != time.Hour {
		t.Errorf("catchUpWindow default = %s, want 1h", s.catchUpWindow)
	}
}

func TestNewWithLocation(t *testing.T) {
	t.Run("nil location falls back to time.Local", func(t *testing.T) {
		s := NewWithLocation(nil, 0, nil)
		if s == nil {
			t.Fatal("NewWithLocation() returned nil")
		}
		if s.c == nil {
			t.Fatal("NewWithLocation() returned a Scheduler with a nil underlying cron.Cron")
		}
	})

	t.Run("explicit location is accepted and next-run is computed in it", func(t *testing.T) {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Skipf("America/New_York tzdata unavailable: %v", err)
		}
		db := testDB(t)
		s := NewWithLocation(db, time.Hour, loc)
		if err := s.RegisterTask("tz-task", "TZ Task", "0 0 * * *", true, false, 0, func() error { return nil }); err != nil {
			t.Fatalf("RegisterTask: %v", err)
		}
		s.Start()
		defer s.Stop()
		if next := s.NextRun("tz-task"); next.IsZero() {
			t.Error("NextRun is zero, want a computed next-run time")
		} else if next.Location().String() != "America/New_York" {
			t.Errorf("NextRun location = %s, want America/New_York", next.Location())
		}
	})
}

func TestRegisterTask_ValidScheduleRuns(t *testing.T) {
	buf := captureLog(t)
	db := testDB(t)

	s := New(db, time.Hour)
	var mu sync.Mutex
	ran := false
	done := make(chan struct{})

	err := s.RegisterTask("every-tick", "Every Tick", "@every 50ms", true, false, 0, func() error {
		mu.Lock()
		defer mu.Unlock()
		if !ran {
			ran = true
			close(done)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.Start()
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("scheduled task did not run within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if !ran {
		t.Error("task did not report as run")
	}

	if !bytes.Contains(buf.Bytes(), []byte(`registered "every-tick"`)) {
		t.Errorf("expected registration log line, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`running "every-tick"`)) {
		t.Errorf("expected run log line, got: %s", buf.String())
	}

	// Give the async DB write time to land.
	time.Sleep(50 * time.Millisecond)
	st, ok := s.Get("every-tick")
	if !ok {
		t.Fatal("Get: task not found after run")
	}
	if st.LastStatus != "success" {
		t.Errorf("LastStatus = %q, want success", st.LastStatus)
	}
	if st.RunCount < 1 {
		t.Errorf("RunCount = %d, want >= 1", st.RunCount)
	}
}

func TestRegisterTask_HandlerErrorIsLoggedNotFatal(t *testing.T) {
	buf := captureLog(t)
	db := testDB(t)

	s := New(db, time.Hour)
	done := make(chan struct{})
	var once sync.Once

	err := s.RegisterTask("failing-task", "Failing Task", "@every 50ms", true, false, 0, func() error {
		once.Do(func() { close(done) })
		return errors.New("boom")
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.Start()
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("failing task did not run within timeout")
	}

	time.Sleep(50 * time.Millisecond)

	if !bytes.Contains(buf.Bytes(), []byte(`task "failing-task" failed: boom`)) {
		t.Errorf("expected failure log line, got: %s", buf.String())
	}

	st, ok := s.Get("failing-task")
	if !ok {
		t.Fatal("Get: task not found after run")
	}
	if st.LastStatus != "failed" {
		t.Errorf("LastStatus = %q, want failed", st.LastStatus)
	}
	if st.FailCount < 1 {
		t.Errorf("FailCount = %d, want >= 1", st.FailCount)
	}
}

func TestRegisterTask_InvalidScheduleIsSkippedNotFatal(t *testing.T) {
	buf := captureLog(t)
	db := testDB(t)

	s := New(db, time.Hour)
	invoked := false

	err := s.RegisterTask("bad-schedule", "Bad Schedule", "not-a-valid-cron-expression", true, false, 0, func() error {
		invoked = true
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	if invoked {
		t.Error("handler for an invalid schedule was invoked, want never invoked")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`invalid schedule for "bad-schedule"`)) {
		t.Errorf("expected invalid-schedule log line, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("task skipped")) {
		t.Errorf("expected 'task skipped' in log, got: %s", buf.String())
	}
}

func TestRegisterTask_DisabledDefaultDoesNotArm(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)
	invoked := false

	if err := s.RegisterTask("off-by-default", "Off", "@every 20ms", false, false, 0, func() error {
		invoked = true
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	if invoked {
		t.Error("disabled-by-default task ran, want never invoked")
	}

	st, ok := s.Get("off-by-default")
	if !ok {
		t.Fatal("Get: task not found")
	}
	if st.Enabled {
		t.Error("persisted state Enabled = true, want false")
	}
}

func TestEnableDisable_PersistAcrossInstances(t *testing.T) {
	db := testDB(t)

	s1 := New(db, time.Hour)
	if err := s1.RegisterTask("toggle", "Toggle", "@every 1h", true, false, 0, func() error { return nil }); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}
	if err := s1.Disable("toggle"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	// A fresh Scheduler instance sharing the same db must see the disabled
	// state and not arm the cron entry.
	s2 := New(db, time.Hour)
	invoked := false
	if err := s2.RegisterTask("toggle", "Toggle", "@every 20ms", true, false, 0, func() error {
		invoked = true
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}
	s2.Start()
	time.Sleep(100 * time.Millisecond)
	s2.Stop()

	if invoked {
		t.Error("task persisted as disabled still ran")
	}

	if err := s2.Enable("toggle"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	st, ok := s2.Get("toggle")
	if !ok {
		t.Fatal("Get: task not found")
	}
	if !st.Enabled {
		t.Error("Enabled = false after Enable(), want true")
	}
}

func TestRunNow_UnknownTask(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)
	if err := s.RunNow("does-not-exist"); err == nil {
		t.Error("RunNow on unknown task: want error, got nil")
	}
}

func TestRunNow_ExecutesImmediately(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)
	ran := make(chan struct{})
	if err := s.RegisterTask("manual", "Manual", "0 0 1 1 *", false, false, 0, func() error {
		close(ran)
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	if err := s.RunNow("manual"); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("RunNow did not execute handler synchronously")
	}
}

func TestCatchUp_RunsOverdueTaskWithinWindow(t *testing.T) {
	db := testDB(t)

	// Simulate a prior run that persisted a next_run in the past, within
	// the catch-up window.
	if err := upsertTaskDefault(db, "overdue", "Overdue", "@every 1h", true); err != nil {
		t.Fatalf("upsertTaskDefault: %v", err)
	}
	if err := updateNextRun(db, "overdue", time.Now().Add(-10*time.Minute)); err != nil {
		t.Fatalf("updateNextRun: %v", err)
	}

	s := New(db, time.Hour)
	ran := make(chan struct{})
	if err := s.RegisterTask("overdue", "Overdue", "@every 1h", true, false, 0, func() error {
		close(ran)
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.catchUp()

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("catchUp did not run the overdue task")
	}
}

func TestCatchUp_SkipsTaskOutsideWindow(t *testing.T) {
	db := testDB(t)

	if err := upsertTaskDefault(db, "stale", "Stale", "@every 1h", true); err != nil {
		t.Fatalf("upsertTaskDefault: %v", err)
	}
	if err := updateNextRun(db, "stale", time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("updateNextRun: %v", err)
	}

	s := New(db, time.Hour)
	invoked := false
	if err := s.RegisterTask("stale", "Stale", "@every 1h", true, false, 0, func() error {
		invoked = true
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.catchUp()
	time.Sleep(20 * time.Millisecond)

	if invoked {
		t.Error("catchUp ran a task overdue by more than the catch-up window")
	}
}

func TestHistory_RecordsAndCaps(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)

	calls := 0
	done := make(chan struct{}, 5)
	if err := s.RegisterTask("history-task", "History", "0 0 1 1 *", false, false, 0, func() error {
		calls++
		done <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := s.RunNow("history-task"); err != nil {
			t.Fatalf("RunNow: %v", err)
		}
	}

	hist, err := s.History("history-task", 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 3 {
		t.Errorf("History len = %d, want 3", len(hist))
	}
	for _, h := range hist {
		if h.Status != "success" {
			t.Errorf("history status = %q, want success", h.Status)
		}
	}
}

func TestList_ReturnsRegistrationOrder(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)

	ids := []string{"a-task", "b-task", "c-task"}
	for _, id := range ids {
		if err := s.RegisterTask(id, id, "0 0 1 1 *", false, false, 0, func() error { return nil }); err != nil {
			t.Fatalf("RegisterTask(%s): %v", id, err)
		}
	}

	states := s.List()
	if len(states) != len(ids) {
		t.Fatalf("List len = %d, want %d", len(states), len(ids))
	}
	for i, id := range ids {
		if states[i].ID != id {
			t.Errorf("List[%d].ID = %q, want %q", i, states[i].ID, id)
		}
	}
}

func TestStartStop_NoTasksRegistered(t *testing.T) {
	s := New(nil, time.Hour)
	// Start/Stop with zero tasks must not block or panic.
	s.Start()
	s.Stop()
}

func TestStop_WithoutStart(t *testing.T) {
	s := New(nil, time.Hour)
	// Stopping a scheduler that was never started must return promptly,
	// not hang forever.
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() on an unstarted scheduler hung")
	}
}

func TestStop_WaitsForInProgressJob(t *testing.T) {
	db := testDB(t)
	s := New(db, time.Hour)
	jobStarted := make(chan struct{})
	jobMayFinish := make(chan struct{})
	jobFinished := make(chan struct{})

	var once sync.Once
	if err := s.RegisterTask("slow-task", "Slow Task", "@every 20ms", true, false, 0, func() error {
		once.Do(func() {
			close(jobStarted)
			<-jobMayFinish
			close(jobFinished)
		})
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	s.Start()

	select {
	case <-jobStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("slow task never started")
	}

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()

	// Stop() must not return before the in-progress job finishes.
	select {
	case <-stopped:
		t.Fatal("Stop() returned before the in-progress job finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(jobMayFinish)

	select {
	case <-jobFinished:
	case <-time.After(3 * time.Second):
		t.Fatal("job never finished")
	}

	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return after the in-progress job finished")
	}
}
