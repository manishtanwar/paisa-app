package background

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/background/kite"
	"github.com/ananthakumaran/paisa/internal/background/prices"
	"github.com/ananthakumaran/paisa/internal/model/task_execution"
)

type Scheduler struct {
	cron    *cron.Cron
	db      *gorm.DB
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	mu      sync.Mutex
	// Map entry IDs to task names for better reporting
	entryToTask map[cron.EntryID]string
}

type Task interface {
	Name() string
	Schedule() string
	Run(ctx context.Context, db *gorm.DB) error
	// ShouldRunOnStartup returns true if this task should run on server startup
	ShouldRunOnStartup() bool
}

var (
	scheduler *Scheduler
	once      sync.Once
)

// GetScheduler returns a singleton instance of the scheduler
func GetScheduler() *Scheduler {
	once.Do(func() {
		scheduler = &Scheduler{}
	})
	return scheduler
}

// Initialize sets up the background scheduler with the database connection
func (s *Scheduler) Initialize(db *gorm.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}

	s.db = db
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.cron = cron.New(cron.WithLocation(time.Local))
	s.entryToTask = make(map[cron.EntryID]string)

	// Register all background tasks
	s.registerTasks()

	s.started = true
	log.Info("Background scheduler initialized")
}

// Start begins the background scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		log.Error("Scheduler not initialized. Call Initialize() first.")
		return
	}

	s.cron.Start()
	log.Info("Background scheduler started")

	// Run any tasks that should run immediately on startup
	s.runStartupTasks()
}

// Stop gracefully stops the background scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	log.Info("Stopping background scheduler...")

	// Stop the cron scheduler
	ctx := s.cron.Stop()
	<-ctx.Done()

	// Cancel the context to stop any running tasks
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.started = false
	log.Info("Background scheduler stopped")
}

// registerTasks registers all background tasks with the scheduler
func (s *Scheduler) registerTasks() {
	tasks := []Task{
		&kite.DailyTradesTask{},
		&prices.DailyPriceUpdateTask{},
	}

	for _, task := range tasks {
		s.registerTask(task)
	}
}

// registerTask registers a single task with the scheduler
func (s *Scheduler) registerTask(task Task) {
	entryID, err := s.cron.AddFunc(task.Schedule(), func() {
		s.wg.Add(1)
		defer s.wg.Done()

		log.Infof("Starting background task: %s", task.Name())
		start := time.Now()

		// Update last run time before starting
		if err := task_execution.UpdateLastRun(s.db, task.Name()); err != nil {
			log.Errorf("Failed to update last run time for task %s: %v", task.Name(), err)
		}

		err := task.Run(s.ctx, s.db)
		if err != nil {
			log.Errorf("Background task %s failed: %v", task.Name(), err)
		} else {
			log.Infof("Background task %s completed in %v", task.Name(), time.Since(start))
			// Update the last successful run time in database
			if err := task_execution.UpdateLastSuccessfulRun(s.db, task.Name()); err != nil {
				log.Errorf("Failed to update last successful run time for task %s: %v", task.Name(), err)
			}
		}
	})

	if err != nil {
		log.Errorf("Failed to register task %s: %v", task.Name(), err)
		return
	}

	// Store the mapping between entry ID and task name
	s.entryToTask[entryID] = task.Name()

	log.Infof("Registered background task: %s (schedule: %s, entry ID: %d)",
		task.Name(), task.Schedule(), entryID)
}

// runStartupTasks runs tasks that should execute immediately when the server starts
func (s *Scheduler) runStartupTasks() {
	tasks := []Task{
		&kite.DailyTradesTask{},
		&prices.DailyPriceUpdateTask{},
	}

	for _, task := range tasks {
		if !task.ShouldRunOnStartup() {
			continue
		}

		// Check if task should run today
		shouldRun, err := task_execution.ShouldRunToday(s.db, task.Name())
		if err != nil {
			log.Errorf("Failed to check if task %s should run today: %v", task.Name(), err)
			continue
		}

		if shouldRun {
			log.Infof("Running startup task: %s", task.Name())
			s.wg.Add(1)
			go func(t Task) {
				defer s.wg.Done()
				start := time.Now()

				// Update last run time before starting
				if err := task_execution.UpdateLastRun(s.db, t.Name()); err != nil {
					log.Errorf("Failed to update last run time for task %s: %v", t.Name(), err)
				}

				if err := t.Run(s.ctx, s.db); err != nil {
					log.Errorf("Failed to run startup task %s: %v", t.Name(), err)
				} else {
					log.Infof("Startup task %s completed in %v", t.Name(), time.Since(start))
					// Update the last successful run time in database
					if err := task_execution.UpdateLastSuccessfulRun(s.db, t.Name()); err != nil {
						log.Errorf("Failed to update last successful run time for task %s: %v", t.Name(), err)
					}
				}
			}(task)
		} else {
			log.Infof("Skipping startup task %s (already run successfully today)", task.Name())
		}
	}
}

// GetNextRunTimes returns the next run times for all scheduled tasks
func (s *Scheduler) GetNextRunTimes() map[string]time.Time {
	if !s.started {
		return nil
	}

	entries := s.cron.Entries()
	nextRuns := make(map[string]time.Time)

	for _, entry := range entries {
		// Use the actual task name if available, otherwise fall back to generic ID
		taskName, exists := s.entryToTask[entry.ID]
		if !exists {
			taskName = fmt.Sprintf("Task-%d", entry.ID)
		}
		nextRuns[taskName] = entry.Next
	}

	return nextRuns
}
