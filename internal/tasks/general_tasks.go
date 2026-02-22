package tasks

import (
	"context"
	"log"
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// LogInfoTaskDef encapsulates the log info task
type LogInfoTaskDef struct{}

// TaskID returns the unique identifier for this task
func (t *LogInfoTaskDef) TaskID() string {
	return "log_info"
}

// HandleExecution handles logging information
func (t *LogInfoTaskDef) HandleExecution(ctx context.Context, db *gorm.DB, task models.ScheduledTask) (map[string]interface{}, error) {
	message, ok := task.Arguments["message"].(string)
	if !ok {
		message = "No message provided"
	}
	log.Printf("[Task: log_info] Message: %s", message)

	maxAttempt := task.MaxAttempt

	return map[string]interface{}{
		"status":            "success",
		"message":           message,
		"max_attempts_info": maxAttempt,
	}, nil
}

// LogInfoTask is the singleton instance of LogInfoTaskDef
var LogInfoTask = &LogInfoTaskDef{}
