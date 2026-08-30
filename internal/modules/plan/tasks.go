package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// ProcessPlanScheduleArgs defines the arguments for a plan schedule task
type ProcessPlanScheduleArgs struct {
	PlanID            uint      `json:"plan_id"`
	Due               time.Time `json:"-"`
	RecurringInterval *string   `json:"-"`
}

// ProcessPlanScheduleTaskDef encapsulates the plan schedule processing logic.
// Execution is delegated to the plan Service.
type ProcessPlanScheduleTaskDef struct {
	svc *Service
}

// NewProcessScheduleTask builds the task definition bound to a Service. The
// composition root (cmd/worker) registers its HandleExecution with the
// scheduler registry.
func NewProcessScheduleTask(svc *Service) *ProcessPlanScheduleTaskDef {
	return &ProcessPlanScheduleTaskDef{svc: svc}
}

// TaskID returns the unique identifier for this task
func (t *ProcessPlanScheduleTaskDef) TaskID() string {
	return "process_plan_schedule"
}

// CreateTask builds a task record for this task
func (t *ProcessPlanScheduleTaskDef) CreateTask(args ProcessPlanScheduleArgs) (*models.ScheduledTask, error) {
	taskType := models.ScheduledTaskTypeOneTime
	if args.RecurringInterval != nil && *args.RecurringInterval != "" {
		taskType = models.ScheduledTaskTypeRecurring
	}
	return models.BuildScheduledTask(t.TaskID(), args, args.Due, args.RecurringInterval, taskType, 3)
}

// HandleExecution handles the processing of plan schedules
func (t *ProcessPlanScheduleTaskDef) HandleExecution(ctx context.Context, db *gorm.DB, task models.ScheduledTask) (map[string]interface{}, error) {
	argsBytes, err := json.Marshal(task.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var parsedArgs ProcessPlanScheduleArgs
	if err := json.Unmarshal(argsBytes, &parsedArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	return t.svc.ProcessSchedule(parsedArgs.PlanID, task.Due)
}
