// Package scheduler implements the always-running, database-backed task
// scheduler described in AI.md PART 18 "Scheduler" — persistent state in
// server.db, automatic catch-up of missed tasks after downtime, retry with
// exponential backoff, and graceful shutdown. External schedulers (cron,
// systemd timers, Task Scheduler, launchd, Kubernetes CronJob) are never
// used; robfig/cron/v3 only supplies in-process schedule parsing/ticking.
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// TaskState is a scheduled task's persisted status, matching the
// server.db "Scheduler State (Persistent)" column table in AI.md PART 18.
type TaskState struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"`
	Enabled    bool      `json:"enabled"`
	LastRun    time.Time `json:"last_run,omitempty"`
	LastStatus string    `json:"last_status,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	NextRun    time.Time `json:"next_run,omitempty"`
	RunCount   int       `json:"run_count"`
	FailCount  int       `json:"fail_count"`
}

// HistoryEntry is one row of a task's execution history.
type HistoryEntry struct {
	RanAt      time.Time `json:"ran_at"`
	Status     string    `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}

// task is the in-process registration backing a TaskState row.
type task struct {
	id, name, schedule string
	handler            func() error
	entryID            cron.EntryID
	hasEntry           bool

	retryOnFail bool
	retryDelay  time.Duration

	mu            sync.Mutex
	retryAttempts int
	retryTimer    *time.Timer
}

// maxRetries and the exponential backoff multiplier per AI.md PART 18
// "Retry Policy" (max_retries=3, backoff=exponential: 5m, 10m, 20m).
const maxRetries = 3

// Scheduler is the always-running, database-backed task scheduler.
type Scheduler struct {
	c  *cron.Cron
	db *sql.DB

	catchUpWindow time.Duration

	tasksMu sync.RWMutex
	tasks   map[string]*task
	order   []string

	// retryWG tracks retry goroutines scheduled via time.AfterFunc in
	// execute() so Stop() can wait for any already-fired retry to finish
	// instead of returning while one is still in flight.
	retryWG sync.WaitGroup
}

// New creates a Scheduler backed by db (server.db) using the standard
// 5-field cron format (minute hour dom month dow). catchUpWindow controls
// how far in the past a missed next_run may be and still be caught up on
// startup (AI.md PART 18 default: 1h). Schedules evaluate in time.Local;
// use NewWithLocation to honor an explicit server.scheduler.timezone.
func New(db *sql.DB, catchUpWindow time.Duration) *Scheduler {
	return NewWithLocation(db, catchUpWindow, time.Local)
}

// NewWithLocation is New but evaluates cron schedules in loc (AI.md PART 18
// "timezone" config field) instead of time.Local. A nil loc falls back to
// time.Local.
func NewWithLocation(db *sql.DB, catchUpWindow time.Duration, loc *time.Location) *Scheduler {
	if catchUpWindow <= 0 {
		catchUpWindow = time.Hour
	}
	if loc == nil {
		loc = time.Local
	}
	c := cron.New(
		cron.WithLocation(loc),
		cron.WithLogger(cron.DiscardLogger),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger), // panic -> log, continue
		),
	)
	return &Scheduler{
		c:             c,
		db:            db,
		catchUpWindow: catchUpWindow,
		tasks:         make(map[string]*task),
	}
}

// RegisterTask registers a named, persisted cron job. schedule is a 5-field
// cron expression or a robfig/cron descriptor (e.g. "@every 15m"). The
// persisted `enabled` flag (loaded from server.db, defaulting to
// enabledDefault on first sight) governs whether the cron entry is armed —
// operator-driven Enable/Disable calls survive restarts.
func (s *Scheduler) RegisterTask(id, name, schedule string, enabledDefault, retryOnFail bool, retryDelay time.Duration, handler func() error) error {
	if s.db != nil {
		if err := upsertTaskDefault(s.db, id, name, schedule, enabledDefault); err != nil {
			return fmt.Errorf("scheduler: persist task %q: %w", id, err)
		}
	}

	enabled := enabledDefault
	if s.db != nil {
		if st, err := loadTaskState(s.db, id); err == nil {
			enabled = st.Enabled
		}
	}

	t := &task{
		id:          id,
		name:        name,
		schedule:    schedule,
		handler:     handler,
		retryOnFail: retryOnFail,
		retryDelay:  retryDelay,
	}

	s.tasksMu.Lock()
	if _, exists := s.tasks[id]; !exists {
		s.order = append(s.order, id)
	}
	s.tasks[id] = t
	s.tasksMu.Unlock()

	if enabled {
		if err := s.arm(t); err != nil {
			log.Printf("scheduler: invalid schedule for %q (%q): %v — task skipped", id, schedule, err)
			return nil
		}
	}
	log.Printf("scheduler: registered %q (schedule: %s, enabled: %v)", id, schedule, enabled)
	return nil
}

// arm adds (or re-adds) the cron entry for a task.
func (s *Scheduler) arm(t *task) error {
	id, err := s.c.AddFunc(t.schedule, func() { s.execute(t) })
	if err != nil {
		return err
	}
	t.entryID = id
	t.hasEntry = true
	return nil
}

// disarm removes a task's cron entry, if any.
func (s *Scheduler) disarm(t *task) {
	if t.hasEntry {
		s.c.Remove(t.entryID)
		t.hasEntry = false
	}
}

// execute runs a task's handler, persists the result, records history, and
// schedules a retry on failure per AI.md PART 18 "Retry Policy".
func (s *Scheduler) execute(t *task) {
	log.Printf("scheduler: running %q", t.id)
	start := time.Now()
	err := t.handler()
	duration := time.Since(start)

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		log.Printf("scheduler: task %q failed: %v", t.id, err)
	} else {
		log.Printf("scheduler: task %q done (%s)", t.id, duration.Round(time.Millisecond))
	}

	next := s.nextRun(t)
	if s.db != nil {
		if dbErr := recordRun(s.db, t.id, status, errMsg, start, next, duration.Milliseconds()); dbErr != nil {
			log.Printf("scheduler: failed to persist run for %q: %v", t.id, dbErr)
		}
	}

	t.mu.Lock()
	if err != nil && t.retryOnFail {
		t.retryAttempts++
		if t.retryAttempts <= maxRetries {
			delay := t.retryDelay
			if delay <= 0 {
				delay = 5 * time.Minute
			}
			// Exponential backoff: delay, 2x, 4x (e.g. 5m, 10m, 20m).
			for i := 1; i < t.retryAttempts; i++ {
				delay *= 2
			}
			log.Printf("scheduler: scheduling retry %d/%d for %q in %s", t.retryAttempts, maxRetries, t.id, delay)
			s.retryWG.Add(1)
			t.retryTimer = time.AfterFunc(delay, func() {
				defer s.retryWG.Done()
				s.execute(t)
			})
		} else {
			log.Printf("scheduler: task %q exhausted %d retries", t.id, maxRetries)
			t.retryAttempts = 0
		}
	} else if err == nil {
		t.retryAttempts = 0
	}
	t.mu.Unlock()
}

// nextRun returns a task's next scheduled invocation, or the zero time if
// it has no armed cron entry.
func (s *Scheduler) nextRun(t *task) time.Time {
	if !t.hasEntry {
		return time.Time{}
	}
	entry := s.c.Entry(t.entryID)
	if entry.ID == 0 {
		return time.Time{}
	}
	return entry.Next
}

// NextRun returns id's next scheduled invocation, or the zero time if the
// task is unknown or has no armed cron entry. Safe to call from within a
// task's own handler (e.g. to report the upcoming retry time in a
// scheduler_error notification, AI.md PART 17).
func (s *Scheduler) NextRun(id string) time.Time {
	s.tasksMu.RLock()
	t, ok := s.tasks[id]
	s.tasksMu.RUnlock()
	if !ok {
		return time.Time{}
	}
	return s.nextRun(t)
}

// Start begins the scheduler: it first runs any tasks whose persisted
// next_run fell within catchUpWindow of now (AI.md PART 18 "Startup
// Behavior"), in ascending order of their original scheduled time, then
// starts the live cron loop.
func (s *Scheduler) Start() {
	s.catchUp()
	s.c.Start()

	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()
	if s.db != nil {
		for _, id := range s.order {
			t := s.tasks[id]
			if next := s.nextRun(t); !next.IsZero() {
				if err := updateNextRun(s.db, id, next); err != nil {
					log.Printf("scheduler: failed to persist next_run for %q: %v", id, err)
				}
			}
		}
	}
}

// catchUp runs any enabled task whose persisted next_run is in the past
// but within catchUpWindow, in ascending order of scheduled time.
func (s *Scheduler) catchUp() {
	if s.db == nil {
		return
	}
	states, err := listTaskStates(s.db)
	if err != nil {
		log.Printf("scheduler: catch-up: failed to load state: %v", err)
		return
	}

	now := time.Now()
	type missed struct {
		id  string
		due time.Time
	}
	var due []missed
	for _, st := range states {
		if !st.Enabled || st.NextRun.IsZero() {
			continue
		}
		if st.NextRun.Before(now) && now.Sub(st.NextRun) <= s.catchUpWindow {
			due = append(due, missed{id: st.ID, due: st.NextRun})
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].due.Before(due[j].due) })

	// Snapshot the tasks to run under the read lock, then release it before
	// executing handlers. A handler may call back into the scheduler (e.g.
	// NextRun for a scheduler_error notification), which takes tasksMu.RLock
	// again — recursive read-locking deadlocks if an operator Enable/Disable
	// is blocked waiting on tasksMu.Lock in between (see sync.RWMutex docs).
	type dueTask struct {
		t   *task
		due time.Time
	}
	s.tasksMu.RLock()
	toRun := make([]dueTask, 0, len(due))
	for _, m := range due {
		if t, ok := s.tasks[m.id]; ok {
			toRun = append(toRun, dueTask{t: t, due: m.due})
		}
	}
	s.tasksMu.RUnlock()

	for _, m := range toRun {
		log.Printf("scheduler: catching up missed task %q (was due %s)", m.t.id, m.due.Format(time.RFC3339))
		s.execute(m.t)
	}
}

// List returns every registered task's current persisted state, in
// registration order. Falls back to in-memory-only state when no database
// is configured.
func (s *Scheduler) List() []TaskState {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	if s.db != nil {
		if states, err := listTaskStates(s.db); err == nil {
			byID := make(map[string]TaskState, len(states))
			for _, st := range states {
				byID[st.ID] = st
			}
			out := make([]TaskState, 0, len(s.order))
			for _, id := range s.order {
				if st, ok := byID[id]; ok {
					out = append(out, st)
				}
			}
			return out
		}
	}

	out := make([]TaskState, 0, len(s.order))
	for _, id := range s.order {
		t := s.tasks[id]
		out = append(out, TaskState{ID: t.id, Name: t.name, Schedule: t.schedule, Enabled: t.hasEntry})
	}
	return out
}

// Get returns a single task's current persisted state.
func (s *Scheduler) Get(id string) (TaskState, bool) {
	s.tasksMu.RLock()
	_, ok := s.tasks[id]
	s.tasksMu.RUnlock()
	if !ok {
		return TaskState{}, false
	}
	if s.db == nil {
		return TaskState{}, false
	}
	st, err := loadTaskState(s.db, id)
	if err != nil {
		return TaskState{}, false
	}
	return st, true
}

// RunNow executes a task immediately, outside its normal schedule.
func (s *Scheduler) RunNow(id string) error {
	s.tasksMu.RLock()
	t, ok := s.tasks[id]
	s.tasksMu.RUnlock()
	if !ok {
		return fmt.Errorf("scheduler: unknown task %q", id)
	}
	s.execute(t)
	return nil
}

// Enable arms a task's cron entry and persists enabled=true.
func (s *Scheduler) Enable(id string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("scheduler: unknown task %q", id)
	}
	if s.db != nil {
		if _, err := setEnabled(s.db, id, true); err != nil {
			return fmt.Errorf("scheduler: persist enable %q: %w", id, err)
		}
	}
	if !t.hasEntry {
		if err := s.arm(t); err != nil {
			return fmt.Errorf("scheduler: re-arm %q: %w", id, err)
		}
	}
	return nil
}

// Disable removes a task's cron entry and persists enabled=false.
func (s *Scheduler) Disable(id string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("scheduler: unknown task %q", id)
	}
	if s.db != nil {
		if _, err := setEnabled(s.db, id, false); err != nil {
			return fmt.Errorf("scheduler: persist disable %q: %w", id, err)
		}
	}
	s.disarm(t)
	return nil
}

// History returns the most recent execution history for a task, newest
// first, capped at limit rows (0 defaults to 20).
func (s *Scheduler) History(id string, limit int) ([]HistoryEntry, error) {
	if s.db == nil {
		return nil, nil
	}
	return listHistory(s.db, id, limit)
}

// Stop gracefully halts the scheduler, waiting up to 30 seconds for any
// in-progress job to complete before returning, per AI.md PART 18
// "Shutdown Behavior". This also covers retries scheduled via time.AfterFunc
// in execute(): a retry timer that has not yet fired is cancelled outright,
// but one that already fired (its retry goroutine is running or about to
// start) is waited on via retryWG so Stop() cannot return while a retry is
// still in flight.
func (s *Scheduler) Stop() {
	stopCtx := s.c.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
		log.Printf("scheduler: shutdown timed out waiting for running tasks")
	}

	s.tasksMu.RLock()
	for _, t := range s.tasks {
		t.mu.Lock()
		if t.retryTimer != nil {
			// Stop returns true only if it prevented the timer from firing —
			// in that case the AfterFunc goroutine (and its retryWG.Done())
			// will never run, so compensate for the earlier retryWG.Add(1)
			// here. If it returns false the timer already fired and the
			// goroutine itself owns the Done() call.
			if t.retryTimer.Stop() {
				s.retryWG.Done()
			}
		}
		t.mu.Unlock()
	}
	s.tasksMu.RUnlock()

	waitDone := make(chan struct{})
	go func() {
		s.retryWG.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(30 * time.Second):
		log.Printf("scheduler: shutdown timed out waiting for in-flight retries")
	}
}
