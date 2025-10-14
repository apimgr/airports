package scheduler

import (
	"log"
	"time"
)

// Task represents a scheduled task
type Task struct {
	Name     string
	Schedule string // Cron-like: "0 3 * * 0" = Sunday at 3 AM
	Handler  func() error
	enabled  bool
	nextRun  time.Time
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	tasks  []*Task
	stopCh chan struct{}
}

// New creates a new scheduler
func New() *Scheduler {
	return &Scheduler{
		tasks:  make([]*Task, 0),
		stopCh: make(chan struct{}),
	}
}

// AddTask adds a task to the scheduler
func (s *Scheduler) AddTask(name, schedule string, handler func() error) {
	task := &Task{
		Name:     name,
		Schedule: schedule,
		Handler:  handler,
		enabled:  true,
		nextRun:  calculateNextRun(schedule),
	}
	s.tasks = append(s.tasks, task)
	log.Printf("Scheduler: Added task '%s' (schedule: %s, next run: %s)", name, schedule, task.nextRun.Format(time.RFC3339))
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			for _, task := range s.tasks {
				if task.enabled && now.After(task.nextRun) {
					log.Printf("Scheduler: Running task '%s'", task.Name)
					go func(t *Task) {
						if err := t.Handler(); err != nil {
							log.Printf("Scheduler: Task '%s' failed: %v", t.Name, err)
						} else {
							log.Printf("Scheduler: Task '%s' completed successfully", t.Name)
						}
						t.nextRun = calculateNextRun(t.Schedule)
						log.Printf("Scheduler: Task '%s' next run: %s", t.Name, t.nextRun.Format(time.RFC3339))
					}(task)
				}
			}
		}
	}
}

// calculateNextRun calculates the next run time based on cron schedule
// Simple implementation for weekly schedule: "0 3 * * 0" = Sunday at 3 AM
func calculateNextRun(schedule string) time.Time {
	now := time.Now()
	
	// For weekly GeoIP update: Sunday at 3:00 AM
	if schedule == "0 3 * * 0" {
		// Find next Sunday
		daysUntilSunday := (7 - int(now.Weekday())) % 7
		if daysUntilSunday == 0 {
			// Today is Sunday, check if 3 AM has passed
			target := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
			if now.After(target) {
				daysUntilSunday = 7 // Next week
			}
		}
		
		nextSunday := now.AddDate(0, 0, daysUntilSunday)
		return time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 3, 0, 0, 0, now.Location())
	}
	
	// Default: run in 7 days
	return now.AddDate(0, 0, 7)
}
