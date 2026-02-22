package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/internal/tasks"
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

// ListPlans renders the list of plans with pagination, filtering, and sorting
func (h *PlanHandler) ListPlans(c echo.Context) error {
	// Parse query parameters
	filterOwnerStr := c.QueryParam("filter_owner")
	filterType := c.QueryParam("filter_type")
	sortBy := c.QueryParam("sort_by")
	if sortBy == "" {
		sortBy = "created"
	}
	sortOrder := c.QueryParam("sort_order")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Parse pagination
	pageStr := c.QueryParam("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	pageSize := 15

	// Parse filter values
	var filterOwner uint
	if filterOwnerStr != "" {
		if val, err := strconv.ParseUint(filterOwnerStr, 10, 32); err == nil {
			filterOwner = uint(val)
		}
	}

	// Build base query
	query := h.db.Model(&models.Plan{}).Preload("Owner").Preload("ScheduledTask").Preload("Participants")

	// Apply filters
	if filterOwner > 0 {
		query = query.Where("owner_id = ?", filterOwner)
	}
	if filterType != "" {
		query = query.Where("payment_type = ?", filterType)
	}

	// Get total count
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to count plans")
	}

	// Calculate pagination
	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	// Apply sorting
	switch sortBy {
	case "name":
		query = query.Order("name " + sortOrder)
	case "date":
		query = query.Order("plan_start_date " + sortOrder)
	case "price":
		query = query.Order("total_price " + sortOrder)
	case "created":
		query = query.Order("created_at " + sortOrder)
	default:
		query = query.Order("created_at " + sortOrder)
	}

	// Apply pagination
	query = query.Limit(pageSize).Offset(offset)

	var plans []models.Plan
	if err := query.Find(&plans).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch plans")
	}

	// Fetch all users for filter dropdown
	var allUsers []models.User
	h.db.Find(&allUsers)

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
		FilterOwner: filterOwner,
		FilterType:  filterType,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalCount:  int(totalCount),
		PageSize:    pageSize,
		AllUsers:    allUsers,
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
	renderError := func(errMsg string) error {
		var users []models.User
		h.db.Find(&users)

		breadcrumbs := []shared.Breadcrumb{
			{Title: "Home", URL: "/"},
			{Title: "Plans", URL: "/plans"},
			{Title: "Create Plan", URL: ""},
		}

		priceStr := c.FormValue("total_price")
		totalPrice, _ := strconv.ParseFloat(priceStr, 64)

		participantPortions := make(map[uint]int)
		for _, idStr := range c.Request().Form["participants"] {
			if uid, err := strconv.ParseUint(idStr, 10, 32); err == nil {
				portionStr := c.FormValue("portion_" + idStr)
				portion := 1
				if p, err := strconv.Atoi(portionStr); err == nil && p > 0 {
					portion = p
				}
				participantPortions[uint(uid)] = portion
			}
		}

		recurringInterval := c.FormValue("recurring_interval")
		var recurringIntervalPtr *string
		if recurringInterval != "" {
			recurringIntervalPtr = &recurringInterval
		}

		plan := models.Plan{
			Name:                    c.FormValue("name"),
			TotalPrice:              totalPrice,
			PaymentType:             c.FormValue("payment_type"),
			RecurringInterval:       recurringIntervalPtr,
			AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
		}

		startDateStr := c.FormValue("plan_start_date")
		if startDateStr == "" {
			startDateStr = time.Now().Format("2006-01-02")
		}

		props := pages.PlanFormProps{
			Title:               "Create New Plan",
			ActiveNav:           "plans",
			Breadcrumbs:         breadcrumbs,
			UserEmail:           getStringFromContext(c, "userEmail"),
			UserUID:             getStringFromContext(c, "userUID"),
			IsEdit:              false,
			Plan:                plan,
			FormattedStartDate:  startDateStr,
			AllUsers:            users,
			ParticipantPortions: participantPortions,
			ErrorMessage:        errMsg,
		}

		return pages.PlanForm(props).Render(c.Request().Context(), c.Response())
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return renderError("Plan name is required")
	}

	priceStr := c.FormValue("total_price")
	totalPrice, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || totalPrice < 1000 {
		return renderError("Total price must be at least 1000")
	}

	formParticipants := c.Request().Form["participants"]
	if len(formParticipants) < 1 {
		return renderError("At least one participant is required")
	}

	startDateStr := c.FormValue("plan_start_date")

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

	// Get current user ID for owner
	ownerID := getUintFromContext(c, "userID")

	plan := models.Plan{
		Name:                    name,
		OwnerID:                 ownerID,
		TotalPrice:              totalPrice,
		PaymentType:             paymentType,
		RecurringInterval:       recurringIntervalPtr,
		PlanStartDate:           planStartDate,
		AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
	}

	if err := h.db.Create(&plan).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create plan")
	}

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
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
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
	userID := getUintFromContext(c, "userID")
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid user session")
	}

	id := c.Param("id")
	var plan models.Plan
	if err := h.db.First(&plan, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	// Authorization Check & Data Integrity Logic
	if plan.OwnerID == 0 {
		// Data corruption healing: If plan has no owner (0), assign to current user
		plan.OwnerID = userID
	} else if plan.OwnerID != userID {
		// Strict ownership check: Only owner or Admin can edit
		var user models.User
		if err := h.db.First(&user, userID).Error; err == nil {
			if user.UserType != models.UserTypeAdmin {
				return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to edit this plan")
			}
		}
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

	plan.AllowInvitationAfterPay = c.FormValue("allow_invitation") == "on"

	if err := h.db.Save(&plan).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update plan: "+err.Error())
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

// DeletePlan handles deleting a plan with proper cascade handling
func (h *PlanHandler) DeletePlan(c echo.Context) error {
	id := c.Param("id")
	planID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid plan ID")
	}

	// Use transaction for cascade operations
	err = h.db.Transaction(func(tx *gorm.DB) error {
		// 1. Get the plan first to check it exists
		var plan models.Plan
		if err := tx.Preload("ScheduledTask").First(&plan, planID).Error; err != nil {
			return err
		}

		// 2. Handle payment dues
		var paymentDues []models.PaymentDue
		tx.Preload("UserPayment").Where("plan_id = ?", planID).Find(&paymentDues)

		for _, due := range paymentDues {
			if due.PaymentStatus == models.PaymentStatusPaid {
				// Create refund record if payment was made
				if due.UserPayment != nil {
					refund := models.Refund{
						PlanID:         uint(planID),
						PaymentDueID:   due.ID,
						UserPaymentID:  due.UserPayment.ID,
						UserID:         due.UserID,
						TotalRefund:    due.UserPayment.TotalPay,
						ChannelPayment: due.UserPayment.ChannelPayment,
						RefundDate:     time.Now(),
					}
					if err := tx.Create(&refund).Error; err != nil {
						return err
					}
				}
			}
			// Cancel the payment due regardless
			if err := tx.Model(&due).Update("payment_status", models.PaymentStatusCanceled).Error; err != nil {
				return err
			}
		}

		// 3. Disable scheduled task if exists
		if plan.ScheduledTask != nil {
			if err := tx.Model(&plan.ScheduledTask).Update("status", models.ScheduledTaskStatusDisabled).Error; err != nil {
				return err
			}
		}

		// 4. Delete the plan
		return tx.Delete(&plan).Error
	})

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete plan: "+err.Error())
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
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	return pages.SchedulePopup(plan).Render(c.Request().Context(), c.Response())
}

// SchedulePlan handles scheduling a plan
func (h *PlanHandler) SchedulePlan(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("ScheduledTask").First(&plan, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	due := plan.PlanStartDate
	if plan.ScheduledTaskID != nil && plan.ScheduledTask != nil {
		due = plan.NextDue()
	}

	taskArgs := tasks.ProcessPlanScheduleArgs{
		PlanID:            plan.ID,
		Due:               due,
		RecurringInterval: plan.RecurringInterval,
	}

	createdTask, err := tasks.ProcessPlanScheduleTask.CreateTask(taskArgs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create task args")
	}

	if plan.ScheduledTaskID == nil {
		// Create new task
		if err := h.db.Create(createdTask).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create scheduled task")
		}

		// Update plan
		plan.ScheduledTaskID = &createdTask.ID
		if err := h.db.Save(&plan).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update plan")
		}
	} else {
		// Update existing task
		task := plan.ScheduledTask
		task.TaskName = createdTask.TaskName
		task.Arguments = createdTask.Arguments
		task.Due = createdTask.Due
		task.RecurringInterval = createdTask.RecurringInterval
		task.Status = createdTask.Status
		task.TaskType = createdTask.TaskType
		task.MaxAttempt = createdTask.MaxAttempt
		task.LastRun = nil // Reset last run

		if err := h.db.Save(task).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update scheduled task")
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// DisableSchedulePlan handles disabling a plan's schedule
func (h *PlanHandler) DisableSchedulePlan(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("ScheduledTask").First(&plan, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	if plan.ScheduledTaskID != nil && plan.ScheduledTask != nil {
		plan.ScheduledTask.Status = models.ScheduledTaskStatusDisabled
		if err := h.db.Save(plan.ScheduledTask).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to disable schedule")
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}
