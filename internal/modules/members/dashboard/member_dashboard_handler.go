package dashboard

import (
	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	dashboard_pages "patungan_app_echo/internal/modules/members/dashboard/pages"
	types "patungan_app_echo/internal/template/types"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type MemberDashboardHandler struct {
	db *gorm.DB
}

func NewMemberDashboardHandler(db *gorm.DB) *MemberDashboardHandler {
	return &MemberDashboardHandler{db: db}
}

// Dashboard renders the member personal dashboard
func (h *MemberDashboardHandler) Dashboard(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")
	userName := middleware.GetString(c, "userName")

	// Get enrolled plans
	var enrolledPlans []models.Plan
	h.db.Joins("JOIN plan_participants ON plan_participants.plan_id = plans.id").
		Where("plan_participants.user_id = ?", userID).
		Preload("Participants").
		Find(&enrolledPlans)

	// Get pending payment dues for this user
	var pendingDues []models.PaymentDue
	h.db.Where("user_id = ? AND payment_status = ?", userID, models.PaymentStatusPending).
		Preload("Plan").
		Order("due_date ASC").
		Find(&pendingDues)

	var pendingAmount float64
	for _, due := range pendingDues {
		pendingAmount += due.CalculatedPayAmount
	}

	// Get paid payments this month for this user
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var paidDuesThisMonth []models.PaymentDue
	h.db.Where("user_id = ? AND payment_status = ? AND updated_at >= ?", userID, models.PaymentStatusPaid, startOfMonth).
		Find(&paidDuesThisMonth)

	var paidThisMonth float64
	for _, due := range paidDuesThisMonth {
		paidThisMonth += due.CalculatedPayAmount
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "Dashboard", URL: ""},
	}

	props := dashboard_pages.MemberDashboardProps{
		Title:            "Member Dashboard",
		ActiveNav:        "dashboard",
		Breadcrumbs:      breadcrumbs,
		UserEmail:        userEmail,
		UserUID:          userUID,
		UserName:         userName,
		ActivePlansCount: len(enrolledPlans),
		PendingAmount:    pendingAmount,
		PendingCount:     len(pendingDues),
		PaidThisMonth:    paidThisMonth,
		PendingDues:      pendingDues,
		EnrolledPlans:    enrolledPlans,
	}

	return dashboard_pages.MemberDashboard(props).Render(c.Request().Context(), c.Response())
}
