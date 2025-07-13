package server

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/background"
	"github.com/ananthakumaran/paisa/internal/background/kite"
	"github.com/ananthakumaran/paisa/internal/background/prices"
	"github.com/ananthakumaran/paisa/internal/model/task_execution"
)

// GetBackgroundTasks returns all background task information
func GetBackgroundTasks(db *gorm.DB) gin.H {
	scheduler := background.GetScheduler()
	nextRuns := scheduler.GetNextRunTimes()

	// Get all task executions from database with retry logic
	var executions []task_execution.TaskExecution
	var err error

	// Retry the database query if it's locked
	for i := 0; i < 3; i++ {
		err = db.Order("last_run DESC").Find(&executions).Error
		if err == nil {
			break
		}

		// Check if it's a database locked error
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "database table is locked") ||
			strings.Contains(err.Error(), "busy") {
			if i < 2 {
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}
		break
	}

	if err != nil {
		return gin.H{"error": "Failed to fetch task executions"}
	}

	// Create a map of task executions by task name
	executionMap := make(map[string]task_execution.TaskExecution)
	for _, exec := range executions {
		executionMap[exec.TaskName] = exec
	}

	// Build the response with all task information
	var tasks []gin.H
	for taskName, nextRun := range nextRuns {
		exec, exists := executionMap[taskName]

		taskInfo := gin.H{
			"task_name": taskName,
			"next_run":  nextRun,
		}

		if exists {
			taskInfo["last_run"] = exec.LastRun
			taskInfo["last_successful_run"] = exec.LastSuccessfulRun
			taskInfo["success"] = exec.Success
		} else {
			// Task has never been run
			taskInfo["last_run"] = time.Time{}
			taskInfo["last_successful_run"] = time.Time{}
			taskInfo["success"] = false
		}

		tasks = append(tasks, taskInfo)
	}

	return gin.H{
		"status": "running",
		"tasks":  tasks,
	}
}

// RunKiteTradesTask runs the KITE trades task immediately
func RunKiteTradesTask(db *gorm.DB) gin.H {
	// Run the KITE trades task immediately
	go func() {
		task := &kite.DailyTradesTask{}

		// Update last run time before starting
		if err := task_execution.UpdateLastRun(db, task.Name()); err != nil {
			log.Errorf("Failed to update last run time for task %s: %v", task.Name(), err)
		}

		if err := task.Run(context.Background(), db); err != nil {
			log.Errorf("Manual KITE trades task failed: %v", err)
		} else {
			// Update the last successful run time
			if err := task_execution.UpdateLastSuccessfulRun(db, task.Name()); err != nil {
				log.Errorf("Failed to update last successful run time for task %s: %v", task.Name(), err)
			}
		}
	}()

	return gin.H{"success": true, "message": "KITE trades task started"}
}

// RunPriceUpdateTask runs the price update task immediately
func RunPriceUpdateTask(db *gorm.DB) gin.H {
	// Run the price update task immediately
	go func() {
		task := &prices.DailyPriceUpdateTask{}

		// Update last run time before starting
		if err := task_execution.UpdateLastRun(db, task.Name()); err != nil {
			log.Errorf("Failed to update last run time for task %s: %v", task.Name(), err)
		}

		if err := task.Run(context.Background(), db); err != nil {
			log.Errorf("Manual price update task failed: %v", err)
		} else {
			// Update the last successful run time
			if err := task_execution.UpdateLastSuccessfulRun(db, task.Name()); err != nil {
				log.Errorf("Failed to update last successful run time for task %s: %v", task.Name(), err)
			}
		}
	}()

	return gin.H{"success": true, "message": "Price update task started"}
}

// StopBackgroundScheduler stops the background scheduler
func StopBackgroundScheduler() gin.H {
	scheduler := background.GetScheduler()
	scheduler.Stop()

	return gin.H{"success": true, "message": "Background scheduler stopped"}
}
