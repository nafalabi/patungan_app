package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
)

// sendNotificationTaskID is the registry name for the notification task.
const sendNotificationTaskID = "send_notification"

// NotificationUser represents the user in the notification payload
type NotificationUser struct {
	UserID      interface{} `json:"userId"` // Can be string or int
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phonenumber"`
	PaymentLink string      `json:"payment_link"`
}

// SendNotificationArgs defines the arguments for a notification task
type SendNotificationArgs struct {
	Users         []NotificationUser `json:"users"`
	NotifTemplate string             `json:"notiftemplate"`
	Subject       string             `json:"subject"`
	PlanName      string             `json:"plan_name"`
	Amount        float64            `json:"amount"`
	DueDate       string             `json:"due_date"`
	AttemptCount  int                `json:"attempt_count"`
	// MaxAttempts caps retry rescheduling. The worker injects the task
	// record's max_attempt into the arguments before execution, so this is
	// populated on worker-driven runs; Send falls back to 3 when unset.
	MaxAttempts int `json:"max_attempt"`
}

// SendNotificationTaskDef encapsulates the notification task logic. Execution
// is delegated to the notification Service; CreateTask only builds task
// records and needs no Service.
type SendNotificationTaskDef struct {
	svc *Service
}

// NewSendNotificationTask builds the task definition bound to a notification
// Service. Use this (not the package-level SendNotificationTask var) when a
// runnable handler is needed, e.g. at the composition root in cmd/worker.
func NewSendNotificationTask(svc *Service) *SendNotificationTaskDef {
	return &SendNotificationTaskDef{svc: svc}
}

// TaskID returns the unique identifier for this task
func (t *SendNotificationTaskDef) TaskID() string {
	return sendNotificationTaskID
}

// CreateTask builds a ScheduledTask record for this task
func (t *SendNotificationTaskDef) CreateTask(args SendNotificationArgs) (*models.ScheduledTask, error) {
	return models.BuildScheduledTask(t.TaskID(), args, time.Now(), nil, models.ScheduledTaskTypeOneTime, 3)
}

// HandleExecution handles sending notifications based on user preference by
// delegating to the bound Service.
func (t *SendNotificationTaskDef) HandleExecution(ctx context.Context, db *gorm.DB, task models.ScheduledTask) (map[string]interface{}, error) {
	if t.svc == nil {
		return nil, fmt.Errorf("notification task is not bound to a service; construct it with NewSendNotificationTask")
	}

	argsBytes, err := json.Marshal(task.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var parsedArgs SendNotificationArgs
	if err := json.Unmarshal(argsBytes, &parsedArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	return t.svc.Send(parsedArgs)
}

// SendNotificationTask is a package-level default kept so existing callers
// (plan.Service) can build task records via CreateTask. Its Service is nil:
// HandleExecution on it returns an error, so bind a runnable definition with
// NewSendNotificationTask instead.
var SendNotificationTask = &SendNotificationTaskDef{}
