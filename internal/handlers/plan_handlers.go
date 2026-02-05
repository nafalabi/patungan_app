package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
)

type PlanHandler struct {
	db *gorm.DB
}

func NewPlanHandler(db *gorm.DB) *PlanHandler {
	return &PlanHandler{db: db}
}

// ListPlans renders the list of plans
func (h *PlanHandler) ListPlans(c echo.Context) error {
	var plans []models.Plan
	if err := h.db.Find(&plans).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch plans")
	}

	// Breadcrumbs: Home > Plans
	breadcrumbs := []Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: ""},
	}

	return c.Render(http.StatusOK, "plans_list.html", map[string]interface{}{
		"Plans":       plans,
		"Title":       "Plan Management",
		"ActiveNav":   "plans",
		"Breadcrumbs": breadcrumbs,
	})
}

// CreatePlanPage renders the create plan form
func (h *PlanHandler) CreatePlanPage(c echo.Context) error {
	// Breadcrumbs: Home > Plans > Create
	breadcrumbs := []Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/plans"},
		{Title: "Create Plan", URL: ""},
	}

	return c.Render(http.StatusOK, "plan_form.html", map[string]interface{}{
		"Title":              "Create New Plan",
		"IsEdit":             false,
		"FormattedStartDate": time.Now().Format("2006-01-02"),
		"ActiveNav":          "plans",
		"Breadcrumbs":        breadcrumbs,
	})
}

// StorePlan handles the creation of a new plan
func (h *PlanHandler) StorePlan(c echo.Context) error {
	name := c.FormValue("name")
	priceStr := c.FormValue("total_price")
	interval := c.FormValue("pay_interval")
	startDateStr := c.FormValue("plan_start_date")

	totalPrice, _ := strconv.ParseFloat(priceStr, 64)

	// Basic parsing - assuming standard date format YYYY-MM-DD from HTML date input
	planStartDate, err := timeFromForm(startDateStr)
	if err != nil {
		// handle error appropriately, maybe re-render form with error
	}

	plan := models.Plan{
		Name:                    name,
		TotalPrice:              totalPrice,
		PayInterval:             interval,
		PlanStartDate:           planStartDate,
		IsActive:                c.FormValue("is_active") == "on",
		AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
	}

	if err := h.db.Create(&plan).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create plan")
	}

	return c.Redirect(http.StatusSeeOther, "/plans")
}

// EditPlanPage renders the edit plan form
func (h *PlanHandler) EditPlanPage(c echo.Context) error {
	id := c.Param("id")
	var plan models.Plan
	if err := h.db.First(&plan, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	// Breadcrumbs: Home > Plans > Edit
	breadcrumbs := []Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/plans"},
		{Title: "Edit Plan", URL: ""},
	}

	return c.Render(http.StatusOK, "plan_form.html", map[string]interface{}{
		"Title":              "Edit Plan",
		"Plan":               plan,
		"IsEdit":             true,
		"FormattedStartDate": plan.PlanStartDate.Format("2006-01-02"),
		"ActiveNav":          "plans",
		"Breadcrumbs":        breadcrumbs,
	})
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
	plan.PayInterval = c.FormValue("pay_interval")

	startDateStr := c.FormValue("plan_start_date")
	if startDateStr != "" {
		plan.PlanStartDate, _ = timeFromForm(startDateStr)
	}

	plan.IsActive = c.FormValue("is_active") == "on"
	plan.AllowInvitationAfterPay = c.FormValue("allow_invitation") == "on"

	if err := h.db.Save(&plan).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update plan")
	}

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
