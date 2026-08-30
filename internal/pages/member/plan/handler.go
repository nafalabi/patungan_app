package plan

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	plan "patungan_app_echo/internal/modules/plan"
	types "patungan_app_echo/internal/template/types"
)

type MemberPlanHandler struct {
	plans *plan.Service
}

func NewMemberPlanHandler(plans *plan.Service) *MemberPlanHandler {
	return &MemberPlanHandler{plans: plans}
}

// ListPlans renders the list of plans enrolled by the current member
func (h *MemberPlanHandler) ListPlans(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")

	enrolledPlans, err := h.plans.EnrolledPlans(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load plans")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Plans", URL: ""},
	}

	props := MemberPlansListProps{
		Title:         "My Plans",
		ActiveNav:     "plans",
		Breadcrumbs:   breadcrumbs,
		UserEmail:     middleware.GetString(c, "userEmail"),
		UserUID:       middleware.GetString(c, "userUID"),
		EnrolledPlans: enrolledPlans,
	}

	return MemberPlansList(props).Render(c.Request().Context(), c.Response())
}

// ShowPlan renders the detailed view of a plan for a member
func (h *MemberPlanHandler) ShowPlan(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")

	planID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid plan ID")
	}

	// Verify that user is a participant in this plan (or admin)
	userType, _ := c.Get("userType").(models.UserType)
	detail, billingPeriods, err := h.plans.DetailForUser(uint(planID), userID, userType == models.UserTypeAdmin)
	if err != nil {
		if errors.Is(err, plan.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "You are not enrolled in this plan")
		}
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load plan")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Plans", URL: "/member/plans"},
		{Title: detail.Plan.Name, URL: ""},
	}

	props := MemberPlanDetailProps{
		Title:          detail.Plan.Name + " - Plan Details",
		ActiveNav:      "plans",
		Breadcrumbs:    breadcrumbs,
		UserEmail:      middleware.GetString(c, "userEmail"),
		UserUID:        middleware.GetString(c, "userUID"),
		CurrentUserID:  userID,
		Plan:           *detail,
		BillingPeriods: billingPeriods,
	}

	return MemberPlanDetail(props).Render(c.Request().Context(), c.Response())
}
