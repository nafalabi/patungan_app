package models

import (
	"time"

	"github.com/teambition/rrule-go"
	"gorm.io/gorm"
)

// ScheduledTaskStatus represents the status of a scheduled task
type ScheduledTaskStatus string

const (
	ScheduledTaskStatusActive   ScheduledTaskStatus = "active"
	ScheduledTaskStatusDone     ScheduledTaskStatus = "done"
	ScheduledTaskStatusFailure  ScheduledTaskStatus = "failure"
	ScheduledTaskStatusDisabled ScheduledTaskStatus = "disabled"
)

// ScheduledTaskType represents the type of scheduled task
type ScheduledTaskType string

const (
	ScheduledTaskTypeOneTime   ScheduledTaskType = "onetime"
	ScheduledTaskTypeRecurring ScheduledTaskType = "recurring"
)

// ScheduledTask tracks tasks that need to be run at a specific time
type ScheduledTask struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	TaskName          string                 `gorm:"type:varchar(255)" json:"task_name"`
	Arguments         map[string]interface{} `gorm:"serializer:json" json:"arguments"`
	LastRun           *time.Time             `json:"last_run"`
	Due               time.Time              `gorm:"index:idx_scheduled_tasks_status_due,priority:2,where:deleted_at IS NULL" json:"due"`
	RecurringInterval *string                `gorm:"type:text" json:"recurring_interval"`
	Status            ScheduledTaskStatus    `gorm:"type:varchar(20);index:idx_scheduled_tasks_status_due,priority:1,where:deleted_at IS NULL" json:"status"`
	TaskType          ScheduledTaskType      `gorm:"type:varchar(20);default:'onetime'" json:"task_type"`
	MaxAttempt        int                    `json:"max_attempt"`
}

// NextDue calculates the next due date for the scheduled task
func (t ScheduledTask) NextDue() time.Time {
	if t.TaskType == ScheduledTaskTypeOneTime {
		return t.Due
	}

	if t.RecurringInterval != nil && *t.RecurringInterval != "" {
		rule, err := rrule.StrToRRule(*t.RecurringInterval)
		if err == nil {
			rule.DTStart(t.Due)
			next := rule.After(time.Now(), true)
			if !next.IsZero() {
				return next
			}
		}
	}
	// Fallback to current Due if parsing fails
	return t.Due
}

// ScheduledTaskHistory tracks the execution history of scheduled tasks
type ScheduledTaskHistory struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	ScheduledTaskID uint           `gorm:"index" json:"scheduled_task_id"`

	TaskName      string                 `gorm:"type:varchar(255)" json:"task_name"`
	RunAt         time.Time              `json:"run_at"`
	Runtime       int                    `json:"runtime"` // in milliseconds? assuming int is fine
	Status        string                 `gorm:"type:varchar(50)" json:"status"`
	AttemptNumber int                    `json:"attempt_number"`
	Arguments     map[string]interface{} `gorm:"serializer:json" json:"arguments"`
	Result        map[string]interface{} `gorm:"serializer:json" json:"result"`
}
