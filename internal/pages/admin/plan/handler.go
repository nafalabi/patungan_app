package plan

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	plan "patungan_app_echo/internal/modules/plan"
	types "patungan_app_echo/internal/template/types"
)

type PlanHandler struct {
	plans *plan.Service
}

func NewPlanHandler(plans *plan.Service) *PlanHandler {
	return &PlanHandler{plans: plans}
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
	page := 1
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	pageSize := 15

	// Parse filter values
	var filterOwner uint
	if filterOwnerStr != "" {
		if val, err := strconv.ParseUint(filterOwnerStr, 10, 32); err == nil {
			filterOwner = uint(val)
		}
	}

	summaries, totalCount, err := h.plans.List(plan.ListParams{
		FilterOwner: filterOwner,
		FilterType:  filterType,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch plans")
	}

	// Calculate pagination
	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// Fetch all users for filter dropdown
	allUsers, _ := h.plans.ListUsers()

	// Breadcrumbs: Home > Plans
	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: ""},
	}

	props := PlansListProps{
		Title:       "Plan Management",
		ActiveNav:   "plans",
		Breadcrumbs: breadcrumbs,
		UserEmail:   middleware.GetString(c, "userEmail"),
		UserUID:     middleware.GetString(c, "userUID"),
		Plans:       summaries,
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

	return PlansList(props).Render(c.Request().Context(), c.Response())
}

// CreatePlanPage renders the create plan form
func (h *PlanHandler) CreatePlanPage(c echo.Context) error {
	// Fetch all users for participant selection
	allUsers, _ := h.plans.ListUsers()

	// Breadcrumbs: Home > Plans > Create
	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/admin/plans"},
		{Title: "Create Plan", URL: ""},
	}

	props := PlanFormProps{
		Title:              "Create New Plan",
		ActiveNav:          "plans",
		Breadcrumbs:        breadcrumbs,
		UserEmail:          middleware.GetString(c, "userEmail"),
		UserUID:            middleware.GetString(c, "userUID"),
		IsEdit:             false,
		FormattedStartDate: time.Now().Format("2006-01-02"),
		AllUsers:           allUsers,

		ParticipantPortions: make(map[uint]int),
	}

	return PlanForm(props).Render(c.Request().Context(), c.Response())
}

// StorePlan handles the creation of a new plan
func (h *PlanHandler) StorePlan(c echo.Context) error {
	renderError := func(errMsg string) error {
		allUsers, _ := h.plans.ListUsers()

		breadcrumbs := []types.Breadcrumb{
			{Title: "Home", URL: "/"},
			{Title: "Plans", URL: "/admin/plans"},
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

		startDateStr := c.FormValue("plan_start_date")
		if startDateStr == "" {
			startDateStr = time.Now().Format("2006-01-02")
		}

		props := PlanFormProps{
			Title:                   "Create New Plan",
			ActiveNav:               "plans",
			Breadcrumbs:             breadcrumbs,
			UserEmail:               middleware.GetString(c, "userEmail"),
			UserUID:                 middleware.GetString(c, "userUID"),
			IsEdit:                  false,
			PlanName:                c.FormValue("name"),
			TotalPrice:              totalPrice,
			PaymentType:             c.FormValue("payment_type"),
			RecurringInterval:       recurringIntervalPtr,
			AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
			FormattedStartDate:      startDateStr,
			AllUsers:                allUsers,
			ParticipantPortions:     participantPortions,
			ErrorMessage:            errMsg,
		}

		return PlanForm(props).Render(c.Request().Context(), c.Response())
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

	participantIDs := c.Request().Form["participants"]
	if len(participantIDs) < 1 {
		return renderError("At least one participant is required")
	}

	startDateStr := c.FormValue("plan_start_date")

	// Basic parsing - assuming standard date format YYYY-MM-DD from HTML date input
	planStartDate, _ := timeFromForm(startDateStr)

	paymentType := c.FormValue("payment_type")
	recurringInterval := c.FormValue("recurring_interval")

	var recurringIntervalPtr *string
	if paymentType == "recurring" && recurringInterval != "" {
		recurringIntervalPtr = &recurringInterval
	}

	// Get current user ID for owner
	ownerID := middleware.GetUint(c, "userID")

	participants := parseParticipantPortions(c, participantIDs)

	if err := h.plans.Create(name, ownerID, totalPrice, paymentType, recurringIntervalPtr, planStartDate, c.FormValue("allow_invitation") == "on", participants); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create plan")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/plans")
}

// EditPlanPage renders the edit plan form
func (h *PlanHandler) EditPlanPage(c echo.Context) error {
	id, ok := parsePlanID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	p, participantPortions, err := h.plans.GetForEdit(id)
	if err != nil {
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch plan")
	}

	// Fetch all users for participant selection
	allUsers, _ := h.plans.ListUsers()

	// Breadcrumbs: Home > Plans > Edit
	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Plans", URL: "/admin/plans"},
		{Title: "Edit Plan", URL: ""},
	}

	props := PlanFormProps{
		Title:                   "Edit Plan",
		ActiveNav:               "plans",
		Breadcrumbs:             breadcrumbs,
		UserEmail:               middleware.GetString(c, "userEmail"),
		UserUID:                 middleware.GetString(c, "userUID"),
		IsEdit:                  true,
		PlanID:                  p.ID,
		PlanName:                p.Name,
		TotalPrice:              p.TotalPrice,
		PaymentType:             p.PaymentType,
		RecurringInterval:       p.RecurringInterval,
		AllowInvitationAfterPay: p.AllowInvitationAfterPay,
		FormattedStartDate:      p.PlanStartDate.Format("2006-01-02"),
		AllUsers:                allUsers,

		ParticipantPortions: participantPortions,
	}

	return PlanForm(props).Render(c.Request().Context(), c.Response())
}

// UpdatePlan handles updating an existing plan
func (h *PlanHandler) UpdatePlan(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid user session")
	}

	id, ok := parsePlanID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Plan name is required")
	}

	priceStr := c.FormValue("total_price")
	totalPrice, _ := strconv.ParseFloat(priceStr, 64)

	recurringInterval := c.FormValue("recurring_interval")
	var recurringIntervalPtr *string
	if c.FormValue("payment_type") == "recurring" && recurringInterval != "" {
		recurringIntervalPtr = &recurringInterval
	}

	var planStartDate time.Time
	if startDateStr := c.FormValue("plan_start_date"); startDateStr != "" {
		planStartDate, _ = timeFromForm(startDateStr)
	}

	input := plan.UpdateInput{
		Name:                    name,
		TotalPrice:              totalPrice,
		PaymentType:             c.FormValue("payment_type"),
		RecurringInterval:       recurringIntervalPtr,
		PlanStartDate:           planStartDate,
		AllowInvitationAfterPay: c.FormValue("allow_invitation") == "on",
		Participants:            parseParticipantPortions(c, c.Request().Form["participants"]),
	}

	if err := h.plans.Update(id, userID, input); err != nil {
		if errors.Is(err, plan.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to edit this plan")
		}
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update plan")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/plans")
}

// DeletePlan handles deleting a plan with proper cascade handling
func (h *PlanHandler) DeletePlan(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid plan ID")
	}

	if err := h.plans.Delete(uint(id)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete plan")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/plans")
}

// Helper to parse date from HTML input type="date"
func timeFromForm(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

// parsePlanID parses the :id route param; ok=false maps to the handler's
// not-found behavior for lookups.
func parsePlanID(c echo.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// parseParticipantPortions builds the participant list from form values,
// parsing the per-user portion inputs.
func parseParticipantPortions(c echo.Context, participantIDs []string) []models.PlanParticipant {
	var participants []models.PlanParticipant
	for _, idStr := range participantIDs {
		uid, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			continue
		}
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
	return participants
}

// GetSchedulePopup renders the schedule popup for a plan
func (h *PlanHandler) GetSchedulePopup(c echo.Context) error {
	id, ok := parsePlanID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	view, err := h.plans.ScheduleView(id)
	if err != nil {
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch plan")
	}

	return SchedulePopup(view).Render(c.Request().Context(), c.Response())
}

// SchedulePlan handles scheduling a plan
func (h *PlanHandler) SchedulePlan(c echo.Context) error {
	id, ok := parsePlanID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	if err := h.plans.Schedule(id); err != nil {
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update scheduled task")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/plans")
}

// DisableSchedulePlan handles disabling a plan's schedule
func (h *PlanHandler) DisableSchedulePlan(c echo.Context) error {
	id, ok := parsePlanID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
	}

	if err := h.plans.DisableSchedule(id); err != nil {
		if errors.Is(err, plan.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Plan not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to disable schedule")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/plans")
}
