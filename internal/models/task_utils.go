package models

import (
	"encoding/json"
	"fmt"
	"time"
)

func BuildScheduledTask(taskName string, args interface{}, due time.Time, recurringInterval *string, taskType ScheduledTaskType, maxAttempt int) (*ScheduledTask, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var mapArgs map[string]interface{}
	if err := json.Unmarshal(argsBytes, &mapArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into map: %w", err)
	}

	return &ScheduledTask{
		TaskName:          taskName,
		Arguments:         mapArgs,
		Due:               due,
		RecurringInterval: recurringInterval,
		Status:            ScheduledTaskStatusActive,
		TaskType:          taskType,
		MaxAttempt:        maxAttempt,
	}, nil
}
