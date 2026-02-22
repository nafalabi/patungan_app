package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
)

// ProcessPlanScheduleArgs defines the arguments for a plan schedule task
type ProcessPlanScheduleArgs struct {
	PlanID            uint      `json:"plan_id"`
	Due               time.Time `json:"-"`
	RecurringInterval *string   `json:"-"`
}

// ProcessPlanScheduleTaskDef encapsulates the plan schedule processing logic
type ProcessPlanScheduleTaskDef struct{}

// TaskID returns the unique identifier for this task
func (t *ProcessPlanScheduleTaskDef) TaskID() string {
	return "process_plan_schedule"
}

// CreateTask builds a ScheduledTask record for this task
func (t *ProcessPlanScheduleTaskDef) CreateTask(args ProcessPlanScheduleArgs) (*models.ScheduledTask, error) {
	taskType := models.ScheduledTaskTypeOneTime
	if args.RecurringInterval != nil && *args.RecurringInterval != "" {
		taskType = models.ScheduledTaskTypeRecurring
	}
	return BuildScheduledTask(t.TaskID(), args, args.Due, args.RecurringInterval, taskType, 3)
}

// HandleExecution handles the processing of plan schedules
func (t *ProcessPlanScheduleTaskDef) HandleExecution(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var parsedArgs ProcessPlanScheduleArgs
	if err := json.Unmarshal(argsBytes, &parsedArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	planID := parsedArgs.PlanID

	var plan models.Plan
	if err := db.Preload("Participants.User").Preload("ScheduledTask").First(&plan, planID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch plan: %w", err)
	}

	if len(plan.Participants) == 0 {
		return map[string]interface{}{"status": "skipped", "message": "No participants in plan"}, nil
	}

	totalPortions := 0
	for _, p := range plan.Participants {
		totalPortions += p.Portion
	}

	if totalPortions == 0 {
		return nil, fmt.Errorf("total portions is 0")
	}

	pricePerPortion := plan.TotalPrice / float64(totalPortions)

	var createdDues []uint
	var notificationUsers []NotificationUser

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:8080"
	}

	for _, p := range plan.Participants {
		amount := pricePerPortion * float64(p.Portion)

		due := models.PaymentDue{
			PlanID:              plan.ID,
			UserID:              p.UserID,
			Portion:             p.Portion,
			CalculatedPayAmount: amount,
			PaymentStatus:       models.PaymentStatusPending,
			DueDate:             plan.ScheduledTask.Due,
			UUID:                uuid.New().String(),
		}
		if err := db.Create(&due).Error; err != nil {
			log.Printf("Failed to create PaymentDue for user %d: %v", p.UserID, err)
			continue
		}
		createdDues = append(createdDues, due.ID)

		paymentLink := fmt.Sprintf("%s/p/%s", appBaseURL, due.UUID)

		notificationUsers = append(notificationUsers, NotificationUser{
			UserID:      p.UserID,
			Username:    p.User.Name,
			Email:       p.User.Email,
			PhoneNumber: p.User.Phone,
			PaymentLink: paymentLink,
		})
	}

	if len(notificationUsers) > 0 {
		notifArgs := SendNotificationArgs{
			Users:         notificationUsers,
			NotifTemplate: "Halo $name, tagihan untuk plan $plan_name sudah jatuh tempo. Yuk segera dibayar di $paymentlink",
			Subject:       "Tagihan Plan " + plan.Name,
			PlanName:      plan.Name,
			Amount:        pricePerPortion,
			DueDate:       plan.ScheduledTask.Due.Format("02 Jan 2006"),
		}

		notifTask, err := SendNotificationTask.CreateTask(notifArgs)
		if err != nil {
			log.Printf("Failed to create notification task args: %v", err)
		} else {
			if err := db.Create(notifTask).Error; err != nil {
				log.Printf("Failed to create notification task: %v", err)
			}
		}

		// Serialize the argument explicitly as requested for logging
		serializedArgs, _ := json.Marshal(notifArgs)
		log.Printf("[Task ProcessPlanSchedule] Generated notification args: %s", string(serializedArgs))

		return map[string]interface{}{
			"status":            "success",
			"created_count":     len(createdDues),
			"total_portions":    totalPortions,
			"notification_args": string(serializedArgs),
		}, nil
	}

	return map[string]interface{}{
		"status":         "success",
		"created_count":  len(createdDues),
		"total_portions": totalPortions,
	}, nil
}

// ProcessPlanScheduleTask is the singleton instance of ProcessPlanScheduleTaskDef
var ProcessPlanScheduleTask = &ProcessPlanScheduleTaskDef{}
