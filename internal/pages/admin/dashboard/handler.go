package dashboard

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	payment "patungan_app_echo/internal/modules/payment"
	user "patungan_app_echo/internal/modules/user"
	types "patungan_app_echo/internal/template/types"
)

type DashboardHandler struct {
	payments *payment.Service
	users    *user.Service
}

func NewDashboardHandler(payments *payment.Service, users *user.Service) *DashboardHandler {
	return &DashboardHandler{payments: payments, users: users}
}

// Dashboard renders the dashboard page
func (h *DashboardHandler) Dashboard(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")

	// Fetch current user to get role
	currentUser, err := h.users.Get(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch user profile")
	}

	stats, err := h.payments.AdminDashboardStats()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load dashboard stats")
	}

	// Breadcrumbs: Home > Dashboard
	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Dashboard", URL: ""}, // Current page
	}

	props := DashboardProps{
		Title:            "Dashboard",
		ActiveNav:        "dashboard",
		Breadcrumbs:      breadcrumbs,
		UserEmail:        userEmail,
		UserUID:          userUID,
		UserID:           userID,
		CurrentUserType:  currentUser.UserType,
		TotalActivePlans: stats.TotalActivePlans,
		PendingDuesCount: stats.PendingDuesCount,
		PendingAmount:    stats.PendingAmount,
		PaidAmount:       stats.PaidAmount,
		UpcomingDues:     stats.UpcomingDues,
	}

	return Dashboard(props).Render(c.Request().Context(), c.Response())
}
