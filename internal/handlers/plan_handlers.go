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
	if err := h.db.Find(&plans).Error; err != nil {
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
		SelectedUserIDs:    make(map[uint]bool),
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
	participantIDs := c.Request().Form["participants"]
	if len(participantIDs) > 0 {
		var users []models.User
		for _, idStr := range participantIDs {
			id, err := strconv.ParseUint(idStr, 10, 32)
			if err == nil {
				users = append(users, models.User{ID: uint(id)})
			}
		}
		if len(users) > 0 {
			h.db.Model(&plan).Association("Users").Replace(users)
		}
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// EditPlanPage renders the edit plan form
func (h *PlanHandler) EditPlanPage(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.Preload("Users").First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	// Fetch all users for participant selection
	var allUsers []models.User
	h.db.Find(&allUsers)

	// Build selected user IDs map
	selectedUserIDs := make(map[uint]bool)
	for _, user := range plan.Users {
		selectedUserIDs[user.ID] = true
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
		SelectedUserIDs:    selectedUserIDs,
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
	participantIDs := c.Request().Form["participants"]
	var users []models.User
	for _, idStr := range participantIDs {
		uid, err := strconv.ParseUint(idStr, 10, 32)
		if err == nil {
			users = append(users, models.User{ID: uint(uid)})
		}
	}
	h.db.Model(&plan).Association("Users").Replace(users)

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
