package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"patungan_app_echo/internal/models"
)

// BuildScheduledTask is a helper to build ScheduledTask records generically
func BuildScheduledTask(taskName string, args interface{}, due time.Time, recurringInterval *string, taskType models.ScheduledTaskType, maxAttempt int) (*models.ScheduledTask, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var mapArgs map[string]interface{}
	if err := json.Unmarshal(argsBytes, &mapArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into map: %w", err)
	}

	return &models.ScheduledTask{
		TaskName:          taskName,
		Arguments:         mapArgs,
		Due:               due,
		RecurringInterval: recurringInterval,
		Status:            models.ScheduledTaskStatusActive,
		TaskType:          taskType,
		MaxAttempt:        maxAttempt,
	}, nil
}
