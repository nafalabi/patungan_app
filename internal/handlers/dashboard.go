package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"
)

// DashboardHandler handles dashboard endpoints
type DashboardHandler struct {
	db *gorm.DB
}

// NewDashboardHandler creates a new DashboardHandler
func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

// Dashboard renders the dashboard page
func (h *DashboardHandler) Dashboard(c echo.Context) error {
	userID := getUintFromContext(c, "userID")
	userEmail := getStringFromContext(c, "userEmail")
	userUID := getStringFromContext(c, "userUID")

	// Fetch current user to get role
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch user profile")
	}

	// Stats variables
	var totalActivePlans int64
	var pendingDuesCount int64
	var pendingAmount float64
	var paidAmount float64
	var upcomingDues []models.PaymentDue

	// Logic based on role
	if user.UserType == models.UserTypeAdmin {
		// Admin sees everything
		// 1. Total Active Plans (Plans that are not deleted)
		h.db.Model(&models.Plan{}).Count(&totalActivePlans)

		// 2. Payment Stats (Global)
		h.db.Model(&models.PaymentDue{}).Where("payment_status != ?", models.PaymentStatusPaid).Count(&pendingDuesCount)

		var pendingResult struct{ Total float64 }
		h.db.Model(&models.PaymentDue{}).Where("payment_status != ?", models.PaymentStatusPaid).Select("COALESCE(SUM(calculated_pay_amount), 0) as total").Scan(&pendingResult)
		pendingAmount = pendingResult.Total

		var paidResult struct{ Total float64 }
		h.db.Model(&models.PaymentDue{}).Where("payment_status = ?", models.PaymentStatusPaid).Select("COALESCE(SUM(calculated_pay_amount), 0) as total").Scan(&paidResult)
		paidAmount = paidResult.Total

		// 3. Upcoming Dues (Global)
		h.db.Preload("Plan").Preload("User").
			Where("payment_status != ?", models.PaymentStatusPaid).
			Order("due_date asc").
			Limit(5).
			Find(&upcomingDues)

	} else {
		// PlanCreator & Member see their own data
		// 1. Total Active Plans (Owner OR Participant)
		// This requires a join or subquery. Simplified: Plans where OwnerID = userID OR ID IN (SELECT plan_id FROM plan_participants WHERE user_id = userID)
		h.db.Model(&models.Plan{}).
			Joins("LEFT JOIN plan_participants ON plan_participants.plan_id = plans.id").
			Where("plans.owner_id = ? OR plan_participants.user_id = ?", userID, userID).
			Distinct("plans.id").
			Count(&totalActivePlans)

		// 2. Payment Stats (My Dues)
		h.db.Model(&models.PaymentDue{}).Where("user_id = ? AND payment_status != ?", userID, models.PaymentStatusPaid).Count(&pendingDuesCount)

		var pendingResult struct{ Total float64 }
		h.db.Model(&models.PaymentDue{}).Where("user_id = ? AND payment_status != ?", userID, models.PaymentStatusPaid).Select("COALESCE(SUM(calculated_pay_amount), 0) as total").Scan(&pendingResult)
		pendingAmount = pendingResult.Total

		var paidResult struct{ Total float64 }
		h.db.Model(&models.PaymentDue{}).Where("user_id = ? AND payment_status = ?", userID, models.PaymentStatusPaid).Select("COALESCE(SUM(calculated_pay_amount), 0) as total").Scan(&paidResult)
		paidAmount = paidResult.Total

		// 3. Upcoming Dues (My Dues)
		h.db.Preload("Plan").Preload("User").
			Where("user_id = ? AND payment_status != ?", userID, models.PaymentStatusPaid).
			Order("due_date asc").
			Limit(5).
			Find(&upcomingDues)
	}

	// Breadcrumbs: Home > Dashboard
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Dashboard", URL: ""}, // Current page
	}

	props := pages.DashboardProps{
		Title:            "Dashboard",
		ActiveNav:        "dashboard",
		Breadcrumbs:      breadcrumbs,
		UserEmail:        userEmail,
		UserUID:          userUID,
		CurrentUserType:  string(user.UserType),
		TotalActivePlans: int(totalActivePlans),
		PendingDuesCount: int(pendingDuesCount),
		PendingAmount:    pendingAmount,
		PaidAmount:       paidAmount,
		UpcomingDues:     upcomingDues,
	}

	return pages.Dashboard(props).Render(c.Request().Context(), c.Response())
}

// Helper to safely get string from context
func getStringFromContext(c echo.Context, key string) string {
	val := c.Get(key)
	if val == nil {
		return ""
	}
	strVal, ok := val.(string)
	if !ok {
		return ""
	}
	return strVal
}

func getUintFromContext(c echo.Context, key string) uint {
	val := c.Get(key)
	if val == nil {
		return 0
	}
	uintVal, ok := val.(uint)
	if !ok {
		return 0
	}
	return uintVal
}
