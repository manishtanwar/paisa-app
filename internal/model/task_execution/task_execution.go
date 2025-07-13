package task_execution

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// TaskExecution tracks when tasks were last executed
type TaskExecution struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TaskName          string    `gorm:"uniqueIndex;not null" json:"task_name"`
	LastRun           time.Time `gorm:"not null" json:"last_run"`
	LastSuccessfulRun time.Time `json:"last_successful_run"`
	Success           bool      `gorm:"default:false" json:"success"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// isDatabaseLockedError checks if the error is a database locked error
func isDatabaseLockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "database is locked") ||
		strings.Contains(err.Error(), "database table is locked") ||
		strings.Contains(err.Error(), "busy")
}

// retryOnLockedDatabase retries the operation if the database is locked
func retryOnLockedDatabase(operation func() error) error {
	maxRetries := 5
	backoff := 10 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		if isDatabaseLockedError(err) {
			if i < maxRetries-1 {
				time.Sleep(backoff)
				backoff *= 2 // Exponential backoff
				continue
			}
		}

		return err
	}

	return nil
}

// GetLastRun retrieves the last run time for a task
func GetLastRun(db *gorm.DB, taskName string) (time.Time, error) {
	var execution TaskExecution

	err := retryOnLockedDatabase(func() error {
		return db.Where("task_name = ?", taskName).First(&execution).Error
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return zero time if task has never been run
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	return execution.LastRun, nil
}

// GetLastSuccessfulRun retrieves the last successful run time for a task
func GetLastSuccessfulRun(db *gorm.DB, taskName string) (time.Time, error) {
	var execution TaskExecution

	err := retryOnLockedDatabase(func() error {
		return db.Where("task_name = ?", taskName).First(&execution).Error
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return zero time if task has never been run successfully
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	return execution.LastSuccessfulRun, nil
}

// UpdateLastRun updates the last run time for a task
func UpdateLastRun(db *gorm.DB, taskName string) error {
	now := time.Now()

	return retryOnLockedDatabase(func() error {
		// Use upsert to create or update the record
		return db.Transaction(func(tx *gorm.DB) error {
			var execution TaskExecution
			err := tx.Where("task_name = ?", taskName).First(&execution).Error

			if err == gorm.ErrRecordNotFound {
				// Create new record
				execution = TaskExecution{
					TaskName:  taskName,
					LastRun:   now,
					Success:   false,
					CreatedAt: now,
					UpdatedAt: now,
				}
				return tx.Create(&execution).Error
			} else if err != nil {
				return err
			}

			// Update existing record
			execution.LastRun = now
			execution.Success = false
			execution.UpdatedAt = now
			return tx.Save(&execution).Error
		})
	})
}

// UpdateLastSuccessfulRun updates the last successful run time for a task
func UpdateLastSuccessfulRun(db *gorm.DB, taskName string) error {
	now := time.Now()

	return retryOnLockedDatabase(func() error {
		// Use upsert to create or update the record
		return db.Transaction(func(tx *gorm.DB) error {
			var execution TaskExecution
			err := tx.Where("task_name = ?", taskName).First(&execution).Error

			if err == gorm.ErrRecordNotFound {
				// Create new record
				execution = TaskExecution{
					TaskName:          taskName,
					LastRun:           now,
					LastSuccessfulRun: now,
					Success:           true,
					CreatedAt:         now,
					UpdatedAt:         now,
				}
				return tx.Create(&execution).Error
			} else if err != nil {
				return err
			}

			// Update existing record
			execution.LastRun = now
			execution.LastSuccessfulRun = now
			execution.Success = true
			execution.UpdatedAt = now
			return tx.Save(&execution).Error
		})
	})
}

// ShouldRunToday checks if a task should run today based on its last successful execution
func ShouldRunToday(db *gorm.DB, taskName string) (bool, error) {
	lastSuccessfulRun, err := GetLastSuccessfulRun(db, taskName)
	if err != nil {
		return false, err
	}

	if lastSuccessfulRun.IsZero() {
		// Task has never been run successfully, should run today
		return true, nil
	}

	// Check if last successful run was today
	now := time.Now()
	lastRunDate := time.Date(lastSuccessfulRun.Year(), lastSuccessfulRun.Month(), lastSuccessfulRun.Day(), 0, 0, 0, 0, lastSuccessfulRun.Location())
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	return lastRunDate.Before(todayDate), nil
}
