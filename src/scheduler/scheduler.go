// Package scheduler wraps robfig/cron/v3 to provide a simple named-task
// scheduler for the airports server. Each task has a standard 5-field cron
// expression, a panic-recovery wrapper, and a context timeout enforced by
// the caller's handler.
//
// Per AI.md PART 18: use robfig/cron or go-co-op/gocron — never host cron
// or systemd timers.
package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps a robfig/cron.Cron with named tasks and recover middleware.
type Scheduler struct {
	c *cron.Cron
}

// New creates a Scheduler that uses the standard 5-field cron format
// (minute hour dom month dow) and recovers panics inside every job.
func New() *Scheduler {
	c := cron.New(
		cron.WithLogger(cron.DiscardLogger),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger), // panic → log, continue
		),
	)
	return &Scheduler{c: c}
}

// AddTask registers a named cron job. schedule is a 5-field cron expression
// (e.g. "0 3 * * 0" for Sunday at 03:00). If the expression is invalid, the
// error is logged and the task is silently skipped rather than crashing.
func (s *Scheduler) AddTask(name, schedule string, handler func() error) {
	_, err := s.c.AddFunc(schedule, func() {
		log.Printf("scheduler: running %q", name)
		if err := handler(); err != nil {
			log.Printf("scheduler: task %q failed: %v", name, err)
		} else {
			log.Printf("scheduler: task %q done", name)
		}
	})
	if err != nil {
		log.Printf("scheduler: invalid schedule for %q (%q): %v — task skipped", name, schedule, err)
		return
	}
	log.Printf("scheduler: registered %q (schedule: %s)", name, schedule)
}

// Start begins the scheduler in the background.
func (s *Scheduler) Start() {
	s.c.Start()
}

// Stop gracefully halts the scheduler, waiting for any in-progress job to
// complete before returning.
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}
