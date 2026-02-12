package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"
)

type PaymentDueHandler struct {
	db    *gorm.DB
	cache *services.RedisCache
}

func NewPaymentDueHandler(db *gorm.DB, cache *services.RedisCache) *PaymentDueHandler {
	return &PaymentDueHandler{db: db, cache: cache}
}

// ListPaymentDues renders the list of payment dues with filtering and sorting
func (h *PaymentDueHandler) ListPaymentDues(c echo.Context) error {
	// Parse query parameters
	viewMode := c.QueryParam("view")
	if viewMode == "" {
		viewMode = "plans"
	}

	filterPlanStr := c.QueryParam("filter_plan")
	filterUserStr := c.QueryParam("filter_user")
	sortBy := c.QueryParam("sort_by")
	if sortBy == "" {
		sortBy = "due_date"
	}
	sortOrder := c.QueryParam("sort_order")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Parse pagination parameters
	pageStr := c.QueryParam("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	pageSize := 20 // Items per page

	// Parse filter values
	var filterPlan, filterUser uint
	if filterPlanStr != "" {
		if val, err := strconv.ParseUint(filterPlanStr, 10, 32); err == nil {
			filterPlan = uint(val)
		}
	}
	if filterUserStr != "" {
		if val, err := strconv.ParseUint(filterUserStr, 10, 32); err == nil {
			filterUser = uint(val)
		}
	}

	// Build base query with filters
	query := h.db.Model(&models.PaymentDue{}).Preload("Plan").Preload("User")

	if filterPlan > 0 {
		query = query.Where("plan_id = ?", filterPlan)
	}
	if filterUser > 0 {
		query = query.Where("user_id = ?", filterUser)
	}

	// Get total count for pagination
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to count payment dues")
	}

	// Calculate pagination values
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
	case "plan":
		// Join with plans table to sort by plan name
		query = query.Joins("JOIN plans ON plans.id = payment_dues.plan_id").
			Order("plans.name " + sortOrder)
	case "user":
		// Join with users table to sort by user name
		query = query.Joins("JOIN users ON users.id = payment_dues.user_id").
			Order("users.name " + sortOrder)
	case "due_date":
		query = query.Order("due_date " + sortOrder)
	default:
		query = query.Order("id " + sortOrder)
	}

	// Apply pagination
	query = query.Limit(pageSize).Offset(offset)

	var paymentDues []models.PaymentDue
	if err := query.Find(&paymentDues).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch payment dues")
	}

	// Fetch all plans and users for filter dropdowns
	var allPlans []models.Plan
	var allUsers []models.User
	h.db.Find(&allPlans)
	h.db.Find(&allUsers)

	// Group data based on view mode
	var planWithDues []pages.PlanWithDues
	var userWithDues []pages.UserWithDues

	if viewMode == "plans" {
		// Group by plans
		planMap := make(map[uint]*pages.PlanWithDues)
		for _, due := range paymentDues {
			if _, exists := planMap[due.PlanID]; !exists {
				planMap[due.PlanID] = &pages.PlanWithDues{
					Plan: due.Plan,
					Dues: []models.PaymentDue{},
				}
			}
			planMap[due.PlanID].Dues = append(planMap[due.PlanID].Dues, due)
		}

		// Convert map to slice
		for _, pwd := range planMap {
			planWithDues = append(planWithDues, *pwd)
		}
	} else {
		// Group by users
		userMap := make(map[uint]*pages.UserWithDues)
		for _, due := range paymentDues {
			if _, exists := userMap[due.UserID]; !exists {
				userMap[due.UserID] = &pages.UserWithDues{
					User: due.User,
					Dues: []models.PaymentDue{},
				}
			}
			userMap[due.UserID].Dues = append(userMap[due.UserID].Dues, due)
		}

		// Convert map to slice
		for _, uwd := range userMap {
			userWithDues = append(userWithDues, *uwd)
		}
	}

	// Breadcrumbs: Home > Payment Dues
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Payment Dues", URL: ""},
	}

	props := pages.PaymentDuesProps{
		Title:        "Payment Dues",
		ActiveNav:    "payment-dues",
		Breadcrumbs:  breadcrumbs,
		UserEmail:    getStringFromContext(c, "userEmail"),
		UserUID:      getStringFromContext(c, "userUID"),
		PlanWithDues: planWithDues,
		UserWithDues: userWithDues,
		ViewMode:     viewMode,
		FilterPlan:   filterPlan,
		FilterUser:   filterUser,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
		AllPlans:     allPlans,
		AllUsers:     allUsers,
		// Pagination
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalCount:  int(totalCount),
		PageSize:    pageSize,
	}

	return pages.PaymentDues(props).Render(c.Request().Context(), c.Response())
}
