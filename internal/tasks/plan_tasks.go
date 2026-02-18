package tasks

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
)

// ProcessPlanScheduleHandler handles the processing of plan schedules
func ProcessPlanScheduleHandler(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	planIDFloat, ok := args["plan_id"].(float64)
	if !ok {
		// Try other types
		if val, ok := args["plan_id"].(int); ok {
			planIDFloat = float64(val)
		} else if val, ok := args["plan_id"].(uint); ok {
			planIDFloat = float64(val)
		} else {
			return nil, fmt.Errorf("plan_id not provided or invalid")
		}
	}
	planID := uint(planIDFloat)

	var plan models.Plan
	if err := db.Preload("Participants").Preload("ScheduledTask").First(&plan, planID).Error; err != nil {
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
	}

	return map[string]interface{}{
		"status":         "success",
		"created_count":  len(createdDues),
		"total_portions": totalPortions,
	}, nil
}
