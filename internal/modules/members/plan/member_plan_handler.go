package plan

import (
	"net/http"
	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	plan_pages "patungan_app_echo/internal/modules/members/plan/pages"
	types "patungan_app_echo/internal/template/types"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type MemberPlanHandler struct {
	db *gorm.DB
}

func NewMemberPlanHandler(db *gorm.DB) *MemberPlanHandler {
	return &MemberPlanHandler{db: db}
}

// ListPlans renders the list of plans enrolled by the current member
func (h *MemberPlanHandler) ListPlans(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")

	var enrolledPlans []models.Plan
	err := h.db.Joins("JOIN plan_participants ON plan_participants.plan_id = plans.id").
		Where("plan_participants.user_id = ?", userID).
		Preload("Owner").
		Preload("Participants.User").
		Order("plans.created_at DESC").
		Find(&enrolledPlans).Error

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load plans")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Plans", URL: ""},
	}

	props := plan_pages.MemberPlansListProps{
		Title:         "My Plans",
		ActiveNav:     "plans",
		Breadcrumbs:   breadcrumbs,
		UserEmail:     userEmail,
		UserUID:       userUID,
		EnrolledPlans: enrolledPlans,
	}

	return plan_pages.MemberPlansList(props).Render(c.Request().Context(), c.Response())
}

// ShowPlan renders the detailed view of a plan for a member
func (h *MemberPlanHandler) ShowPlan(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")

	idStr := c.Param("id")
	planID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid plan ID")
	}

	// Verify that user is a participant in this plan (or admin)
	userType, _ := c.Get("userType").(models.UserType)
	var participant models.PlanParticipant
	if userType != models.UserTypeAdmin {
		if err := h.db.Where("plan_id = ? AND user_id = ?", planID, userID).First(&participant).Error; err != nil {
			return echo.NewHTTPError(http.StatusForbidden, "You are not enrolled in this plan")
		}
	}

	var plan models.Plan
	err = h.db.Preload("Owner").
		Preload("Participants.User").
		First(&plan, planID).Error

	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	var billingPeriods []models.PaymentBillingPeriod
	h.db.Where("plan_id = ?", planID).
		Preload("Dues.User").
		Order("due_date DESC").
		Find(&billingPeriods)

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Plans", URL: "/member/plans"},
		{Title: plan.Name, URL: ""},
	}

	props := plan_pages.MemberPlanDetailProps{
		Title:          plan.Name + " - Plan Details",
		ActiveNav:      "plans",
		Breadcrumbs:    breadcrumbs,
		UserEmail:      userEmail,
		UserUID:        userUID,
		CurrentUserID:  userID,
		Plan:           plan,
		BillingPeriods: billingPeriods,
	}

	return plan_pages.MemberPlanDetail(props).Render(c.Request().Context(), c.Response())
}
