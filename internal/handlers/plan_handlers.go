package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"
)

type PlanHandler struct {
	db    *gorm.DB
	cache *services.RedisCache
}

func NewPlanHandler(db *gorm.DB, cache *services.RedisCache) *PlanHandler {
	return &PlanHandler{db: db, cache: cache}
}

// ListPlans renders the list of plans
func (h *PlanHandler) ListPlans(c echo.Context) error {
	var plans []models.Plan
	if err := h.db.Preload("ScheduledTask").Find(&plans).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch plans")
	}

	// Breadcrumbs: Home > Plans
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: ""},
	}

	props := pages.PlansListProps{
		Title:       "Plan Management",
		ActiveNav:   "plans",
		Breadcrumbs: breadcrumbs,
		UserEmail:   getStringFromContext(c, "userEmail"),
		UserUID:     getStringFromContext(c, "userUID"),
		Plans:       plans,
	}

	return pages.PlansList(props).Render(c.Request().Context(), c.Response())
}

// CreatePlanPage renders the create plan form
func (h *PlanHandler) CreatePlanPage(c echo.Context) error {
	// Fetch all users for participant selection
	var users []models.User
	h.db.Find(&users)

	// Breadcrumbs: Home > Plans > Create
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/plans"},
		{Title: "Create Plan", URL: ""},
	}

	props := pages.PlanFormProps{
		Title:              "Create New Plan",
		ActiveNav:          "plans",
		Breadcrumbs:        breadcrumbs,
		UserEmail:          getStringFromContext(c, "userEmail"),
		UserUID:            getStringFromContext(c, "userUID"),
		IsEdit:             false,
		FormattedStartDate: time.Now().Format("2006-01-02"),
		AllUsers:           users,

		ParticipantPortions: make(map[uint]int),
	}

	return pages.PlanForm(props).Render(c.Request().Context(), c.Response())
}

// StorePlan handles the creation of a new plan
func (h *PlanHandler) StorePlan(c echo.Context) error {
	name := c.FormValue("name")
	priceStr := c.FormValue("total_price")

	startDateStr := c.FormValue("plan_start_date")

	totalPrice, _ := strconv.ParseFloat(priceStr, 64)

	// Basic parsing - assuming standard date format YYYY-MM-DD from HTML date input
	planStartDate, err := timeFromForm(startDateStr)
	if err != nil {
		// handle error appropriately, maybe re-render form with error
	}

	paymentType := c.FormValue("payment_type")
	recurringInterval := c.FormValue("recurring_interval")

	var recurringIntervalPtr *string
	if paymentType == "recurring" && recurringInterval != "" {
		recurringIntervalPtr = &recurringInterval
	}

	plan := models.Plan{
		Name:                    name,
		TotalPrice:              totalPrice,
		PaymentType:             paymentType,
		RecurringInterval:       recurringIntervalPtr,
		PlanStartDate:           planStartDate,
		IsActive:                c.FormValue("is_active") == "on",
		AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
	}

	if err := h.db.Create(&plan).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create plan")
	}

	// Handle participants
	// Handle participants
	participantIDs := c.Request().Form["participants"]
	if len(participantIDs) > 0 {
		var participants []models.PlanParticipant
		for _, idStr := range participantIDs {
			uid, err := strconv.ParseUint(idStr, 10, 32)
			if err == nil {
				// Parse portion specific for this user
				portionStr := c.FormValue("portion_" + idStr)
				portion := 1
				if p, err := strconv.Atoi(portionStr); err == nil && p >= 0 {
					portion = p
				}

				participants = append(participants, models.PlanParticipant{
					UserID:  uint(uid),
					Portion: portion,
				})
			}
		}
		if len(participants) > 0 {
			plan.Participants = participants
			if err := h.db.Save(&plan).Error; err != nil {
				// log error but continue
			}
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// EditPlanPage renders the edit plan form
func (h *PlanHandler) EditPlanPage(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("Participants.User").First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	// Fetch all users for participant selection
	var allUsers []models.User
	h.db.Find(&allUsers)

	// Build selected participants map
	// Map from UserID -> Portion
	participantPortions := make(map[uint]int)
	for _, p := range plan.Participants {
		participantPortions[p.UserID] = p.Portion
	}

	// Breadcrumbs: Home > Plans > Edit
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/plans"},
		{Title: "Edit Plan", URL: ""},
	}

	props := pages.PlanFormProps{
		Title:              "Edit Plan",
		ActiveNav:          "plans",
		Breadcrumbs:        breadcrumbs,
		UserEmail:          getStringFromContext(c, "userEmail"),
		UserUID:            getStringFromContext(c, "userUID"),
		IsEdit:             true,
		Plan:               plan,
		FormattedStartDate: plan.PlanStartDate.Format("2006-01-02"),
		AllUsers:           allUsers,

		ParticipantPortions: participantPortions,
	}

	return pages.PlanForm(props).Render(c.Request().Context(), c.Response())
}

// UpdatePlan handles updating an existing plan
func (h *PlanHandler) UpdatePlan(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	plan.Name = c.FormValue("name")
	priceStr := c.FormValue("total_price")
	plan.TotalPrice, _ = strconv.ParseFloat(priceStr, 64)
	plan.PaymentType = c.FormValue("payment_type")
	recurringInterval := c.FormValue("recurring_interval")

	if plan.PaymentType == "recurring" && recurringInterval != "" {
		plan.RecurringInterval = &recurringInterval
	} else {
		plan.RecurringInterval = nil
	}

	startDateStr := c.FormValue("plan_start_date")
	if startDateStr != "" {
		plan.PlanStartDate, _ = timeFromForm(startDateStr)
	}

	plan.IsActive = c.FormValue("is_active") == "on"
	plan.AllowInvitationAfterPay = c.FormValue("allow_invitation") == "on"

	if err := h.db.Save(&plan).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update plan")
	}

	// Handle participants update
	// Handle participants update
	participantIDs := c.Request().Form["participants"]
	var newParticipants []models.PlanParticipant

	for _, idStr := range participantIDs {
		uid, err := strconv.ParseUint(idStr, 10, 32)
		if err == nil {
			// Parse portion specific for this user
			portionStr := c.FormValue("portion_" + idStr)
			portion := 1
			if p, err := strconv.Atoi(portionStr); err == nil && p >= 0 {
				portion = p
			}

			newParticipants = append(newParticipants, models.PlanParticipant{
				PlanID:  plan.ID,
				UserID:  uint(uid),
				Portion: portion,
			})
		}
	}

	// Transaction to replace participants
	h.db.Transaction(func(tx *gorm.DB) error {
		// Remove existing participants
		if err := tx.Where("plan_id = ?", plan.ID).Delete(&models.PlanParticipant{}).Error; err != nil {
			return err
		}
		// Add new ones
		if len(newParticipants) > 0 {
			if err := tx.Create(&newParticipants).Error; err != nil {
				return err
			}
		}
		return nil
	})

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// DeletePlan handles deleting a plan
func (h *PlanHandler) DeletePlan(c echo.Context) error {
	id := c.Param("id")
	if err := h.db.Delete(&models.Plan{}, id).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete plan")
	}
	return c.Redirect(http.StatusSeeOther, "/plans")
}

// Helper to parse date from HTML input type="date"
func timeFromForm(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

// GetSchedulePopup renders the schedule popup for a plan
func (h *PlanHandler) GetSchedulePopup(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("ScheduledTask").First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	return pages.SchedulePopup(plan).Render(c.Request().Context(), c.Response())
}

// SchedulePlan handles scheduling a plan
func (h *PlanHandler) SchedulePlan(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("ScheduledTask").First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	taskName := "process_plan_schedule"
	arguments := map[string]interface{}{"plan_id": plan.ID}
	due := plan.NextDue()

	status := models.ScheduledTaskStatusActive

	var taskType models.ScheduledTaskType
	if plan.PaymentType == "recurring" {
		taskType = models.ScheduledTaskTypeRecurring
	} else {
		taskType = models.ScheduledTaskTypeOneTime
	}

	if plan.ScheduledTaskID == nil {
		// Create new task
		task := models.ScheduledTask{
			TaskName:          taskName,
			Arguments:         arguments,
			Due:               due,
			RecurringInterval: plan.RecurringInterval,
			Status:            status,
			TaskType:          taskType,
			MaxAttempt:        3,
		}
		if err := h.db.Create(&task).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to create scheduled task")
		}

		// Update plan
		plan.ScheduledTaskID = &task.ID
		if err := h.db.Save(&plan).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to update plan")
		}
	} else {
		// Update existing task
		task := plan.ScheduledTask
		task.TaskName = taskName
		task.Arguments = arguments
		task.Due = due
		task.RecurringInterval = plan.RecurringInterval
		task.Status = status
		task.TaskType = taskType
		task.MaxAttempt = 3
		task.LastRun = nil // Reset last run

		if err := h.db.Save(task).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to update scheduled task")
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// DisableSchedulePlan handles disabling a plan's schedule
func (h *PlanHandler) DisableSchedulePlan(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("ScheduledTask").First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	if plan.ScheduledTaskID != nil && plan.ScheduledTask != nil {
		plan.ScheduledTask.Status = models.ScheduledTaskStatusDisabled
		if err := h.db.Save(plan.ScheduledTask).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to disable schedule")
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}
