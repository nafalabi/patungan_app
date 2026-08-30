package dashboard

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	payment "patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	types "patungan_app_echo/internal/template/types"
)

type MemberDashboardHandler struct {
	payments *payment.Service
	plans    *plan.Service
}

func NewMemberDashboardHandler(payments *payment.Service, plans *plan.Service) *MemberDashboardHandler {
	return &MemberDashboardHandler{payments: payments, plans: plans}
}

// Dashboard renders the member personal dashboard
func (h *MemberDashboardHandler) Dashboard(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")
	userName := middleware.GetString(c, "userName")

	// Get enrolled plans (excluding soft-deleted participant associations)
	enrolledPlans, err := h.plans.EnrolledPlans(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load enrolled plans")
	}

	stats, err := h.payments.UserDashboardStats(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "Dashboard", URL: ""},
	}

	stats.ActivePlansCount = len(enrolledPlans)

	props := MemberDashboardProps{
		Title:            "Member Dashboard",
		ActiveNav:        "dashboard",
		Breadcrumbs:      breadcrumbs,
		UserEmail:        userEmail,
		UserUID:          userUID,
		UserName:         userName,
		ActivePlansCount: stats.ActivePlansCount,
		PendingAmount:    stats.PendingAmount,
		PendingCount:     stats.PendingCount,
		PaidThisMonth:    stats.PaidThisMonth,
		PendingDues:      stats.PendingDues,
		EnrolledPlans:    enrolledPlans,
	}

	return MemberDashboard(props).Render(c.Request().Context(), c.Response())
}
