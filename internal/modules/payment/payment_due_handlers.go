package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	payment_pages "patungan_app_echo/internal/modules/payment/pages"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/services/payment_service"
	types "patungan_app_echo/internal/template/types"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type PaymentDueHandler struct {
	db             *gorm.DB
	cache          *cache.RedisCache
	paymentService *payment_service.PaymentService
}

func NewPaymentDueHandler(db *gorm.DB, cache *cache.RedisCache, paymentService *payment_service.PaymentService) *PaymentDueHandler {
	return &PaymentDueHandler{db: db, cache: cache, paymentService: paymentService}
}

// ListPaymentDues is the main entry point that routes to specialized renderers
func (h *PaymentDueHandler) ListPaymentDues(c echo.Context) error {
	viewMode := c.QueryParam("view")
	if viewMode == "" {
		viewMode = "all"
	}

	switch viewMode {
	case "plans":
		return h.renderByPlans(c)
	case "periods":
		return h.renderByPeriods(c)
	case "users":
		return h.renderByUsers(c)
	default:
		return h.renderAll(c)
	}
}

// renderAll handles the classic flat list view with pagination
func (h *PaymentDueHandler) renderAll(c echo.Context) error {
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

	page := 1
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	pageSize := 20

	query := h.db.Model(&models.PaymentDue{}).
		Preload("Plan").Preload("User").Preload("BillingPeriod").
		Where("payment_status != ?", models.PaymentStatusCanceled)

	if filterPlanStr != "" {
		query = query.Where("plan_id = ?", filterPlanStr)
	}
	if filterUserStr != "" {
		query = query.Where("user_id = ?", filterUserStr)
	}

	var totalCount int64
	query.Count(&totalCount)

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	offset := (page - 1) * pageSize

	// Sorting
	switch sortBy {
	case "plan":
		query = query.Joins("JOIN plans ON plans.id = payment_dues.plan_id").Order("plans.name " + sortOrder)
	case "user":
		query = query.Joins("JOIN users ON users.id = payment_dues.user_id").Order("users.name " + sortOrder)
	default:
		query = query.Order(sortBy + " " + sortOrder)
	}

	var dues []models.PaymentDue
	query.Limit(pageSize).Offset(offset).Find(&dues)

	return h.renderPage(c, "all", payment_pages.PaymentDuesProps{
		FlatDues:    dues,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalCount:  int(totalCount),
		PageSize:    pageSize,
		SortBy:      sortBy,
		SortOrder:   sortOrder,
	})
}

// renderByPlans handles grouping by plan, showing only latest period
func (h *PaymentDueHandler) renderByPlans(c echo.Context) error {
	limit := 5
	offset := 0
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
		offset = o
	}

	// Subquery to find plans with latest due date activity
	var plans []models.Plan
	h.db.Table("plans").
		Select("plans.*").
		Joins("JOIN (SELECT plan_id, MAX(due_date) as latest FROM payment_dues GROUP BY plan_id) as ld ON ld.plan_id = plans.id").
		Order("ld.latest DESC, plans.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&plans)

	var planWithDues []payment_pages.PlanWithDues
	for _, plan := range plans {
		// Get the latest period for this plan
		var latestPeriod models.PaymentBillingPeriod
		err := h.db.Table("payment_billing_periods").
			Joins("JOIN payment_dues ON payment_dues.payment_billing_period_id = payment_billing_periods.id").
			Where("payment_dues.plan_id = ?", plan.ID).
			Order("payment_billing_periods.due_date DESC").
			First(&latestPeriod).Error

		if err == nil {
			var dues []models.PaymentDue
			h.db.Preload("User").Preload("BillingPeriod").
				Where("plan_id = ? AND payment_billing_period_id = ?", plan.ID, latestPeriod.ID).
				Find(&dues)

			planWithDues = append(planWithDues, payment_pages.PlanWithDues{
				Plan: plan,
				Periods: []payment_pages.PeriodWithDues{
					{Period: latestPeriod, Dues: dues},
				},
			})
		} else {
			// Handle dues without period
			var dues []models.PaymentDue
			h.db.Preload("User").
				Where("plan_id = ? AND payment_billing_period_id = 0", plan.ID).
				Find(&dues)
			if len(dues) > 0 {
				planWithDues = append(planWithDues, payment_pages.PlanWithDues{
					Plan: plan,
					Periods: []payment_pages.PeriodWithDues{
						{Period: models.PaymentBillingPeriod{ID: 0}, Dues: dues},
					},
				})
			}
		}
	}

	nextOffset := offset + limit
	if len(plans) < limit {
		nextOffset = 0 // No more
	}

	// If HTMX partial, render just the list items and the new OOB button
	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := payment_pages.PlanListItems(planWithDues, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return payment_pages.LoadMorePlansButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "plans", payment_pages.PaymentDuesProps{
		PlanWithDues: planWithDues,
		NextOffset:   nextOffset,
	})
}

// renderByPeriods handles grouping by period, then plans
func (h *PaymentDueHandler) renderByPeriods(c echo.Context) error {
	limit := 3
	offset := 0
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
		offset = o
	}

	var periods []models.PaymentBillingPeriod
	h.db.Order("due_date DESC, id DESC").Limit(limit).Offset(offset).Find(&periods)

	var periodWithPlans []payment_pages.PeriodWithPlans
	for _, period := range periods {
		// Get top 3 plans for this period
		var plans []models.Plan
		h.db.Table("plans").
			Select("plans.*").
			Joins("JOIN payment_dues ON payment_dues.plan_id = plans.id").
			Where("payment_dues.payment_billing_period_id = ?", period.ID).
			Group("plans.id").
			Order("plans.id ASC").
			Limit(3).
			Find(&plans)

		pwp := payment_pages.PeriodWithPlans{Period: period}
		for _, plan := range plans {
			var dues []models.PaymentDue
			h.db.Preload("User").Preload("BillingPeriod").
				Where("payment_billing_period_id = ? AND plan_id = ?", period.ID, plan.ID).
				Find(&dues)

			pwp.Plans = append(pwp.Plans, payment_pages.PlanWithDuesInPeriod{
				Plan: plan,
				Dues: dues,
			})
		}
		periodWithPlans = append(periodWithPlans, pwp)
	}

	nextOffset := offset + limit
	if len(periods) < limit {
		nextOffset = 0
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := payment_pages.PeriodListItems(periodWithPlans, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return payment_pages.LoadMorePeriodsButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "periods", payment_pages.PaymentDuesProps{
		PeriodWithPlans: periodWithPlans,
		NextOffset:      nextOffset,
	})
}

// renderByUsers handles grouping by users, then plans
func (h *PaymentDueHandler) renderByUsers(c echo.Context) error {
	limit := 5
	offset := 0
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
		offset = o
	}

	var users []models.User
	h.db.Table("users").
		Select("users.*").
		Joins("JOIN (SELECT user_id, MAX(due_date) as latest FROM payment_dues GROUP BY user_id) as ld ON ld.user_id = users.id").
		Order("ld.latest DESC, users.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users)

	var userWithDues []payment_pages.UserWithDues
	for _, user := range users {
		var dues []models.PaymentDue
		// Show 3 latest dues per user
		h.db.Preload("Plan").Preload("BillingPeriod").
			Where("user_id = ?", user.ID).
			Order("due_date DESC").
			Limit(3).
			Find(&dues)

		userWithDues = append(userWithDues, payment_pages.UserWithDues{
			User: user,
			Dues: dues,
		})
	}

	nextOffset := offset + limit
	if len(users) < limit {
		nextOffset = 0
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := payment_pages.UserListItems(userWithDues, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return payment_pages.LoadMoreUsersButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "users", payment_pages.PaymentDuesProps{
		UserWithDues: userWithDues,
		NextOffset:   nextOffset,
	})
}

// Helper to fill common props and render
func (h *PaymentDueHandler) renderPage(c echo.Context, viewMode string, props payment_pages.PaymentDuesProps) error {
	// Common filters
	filterPlanStr := c.QueryParam("filter_plan")
	filterUserStr := c.QueryParam("filter_user")
	var filterPlan, filterUser uint
	if fp, err := strconv.ParseUint(filterPlanStr, 10, 32); err == nil {
		filterPlan = uint(fp)
	}
	if fu, err := strconv.ParseUint(filterUserStr, 10, 32); err == nil {
		filterUser = uint(fu)
	}

	var allPlans []models.Plan
	var allUsers []models.User
	h.db.Find(&allPlans)
	h.db.Find(&allUsers)

	props.Title = "Payment Dues"
	props.ActiveNav = "payment-dues"
	props.Breadcrumbs = []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Payment Dues", URL: ""},
	}
	props.UserEmail = middleware.GetString(c, "userEmail")
	props.UserUIDString = middleware.GetString(c, "userUID")
	props.CurrentUserID = middleware.GetUint(c, "userID")
	props.CurrentUserType = h.getUserType(c)
	props.MidtransClientKey = os.Getenv("MIDTRANS_CLIENT_KEY")
	props.ViewMode = viewMode
	props.FilterPlan = filterPlan
	props.FilterUser = filterUser
	props.AllPlans = allPlans
	props.AllUsers = allUsers

	return payment_pages.PaymentDues(props).Render(c.Request().Context(), c.Response())
}

func (h *PaymentDueHandler) getUserType(c echo.Context) models.UserType {
	if val := c.Get("userType"); val != nil {
		if ut, ok := val.(models.UserType); ok {
			return ut
		}
	}
	return models.UserTypeMember
}

// MidtransCallback handles validation of Midtrans notifications
func (h *PaymentDueHandler) MidtransCallback(c echo.Context) error {
	var notificationPayload map[string]interface{}
	if err := c.Bind(&notificationPayload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON payload")
	}

	// Log to models.PaymentCallbackHistory
	payloadBytes, _ := json.Marshal(notificationPayload)
	history := models.PaymentCallbackHistory{
		PaymentGateway: models.PaymentGatewayMidtrans,
		Metadata:       payloadBytes,
	}
	h.db.Create(&history)

	// Extract necessary fields
	orderID, _ := notificationPayload["order_id"].(string)
	transactionStatus, _ := notificationPayload["transaction_status"].(string)
	fraudStatus, _ := notificationPayload["fraud_status"].(string)
	// signatureKey, _ := notificationPayload["signature_key"].(string)
	// statusCode, _ := notificationPayload["status_code"].(string)
	// grossAmount, _ := notificationPayload["gross_amount"].(string)

	// Note: Signature verification should be done via the gateway interface
	// For now, we'll assume the callback is coming from a trusted source or implement it in the gateway implementation
	// We'll refactor this to use the gateway's VerifyNotification if needed.

	// Quick hack for signature verification using environment key directly if needed
	// But better to move this logic to MidtransGateway

	// Parse Order ID to get PaymentDueID
	// Format: payment-due-{id}-{timestamp}
	parts := strings.Split(orderID, "-")
	if len(parts) < 3 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid order ID format")
	}
	dueIDStr := parts[2] // payment (0), due (1), ID (2), timestamp (3)
	dueID, err := strconv.ParseUint(dueIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID in order ID")
	}

	// Fetch models.PaymentDue
	var due models.PaymentDue
	if err := h.db.First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// Handle status
	h.paymentService.HandleTransactionStatus(&due, orderID, transactionStatus, fraudStatus, notificationPayload["payment_type"].(string), notificationPayload["gross_amount"].(string))

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// MayarCallback handles validation of Mayar notifications
func (h *PaymentDueHandler) MayarCallback(c echo.Context) error {
	var notificationPayload map[string]interface{}
	if err := c.Bind(&notificationPayload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON payload")
	}

	// 1. Log to history
	payloadBytes, _ := json.Marshal(notificationPayload)
	history := models.PaymentCallbackHistory{
		PaymentGateway: models.PaymentGatewayMayar,
		Metadata:       payloadBytes,
	}
	h.db.Create(&history)

	// 2. Extract Data (Mayar often wraps in a 'data' field)
	data, ok := notificationPayload["data"].(map[string]interface{})
	if !ok {
		data = notificationPayload
	}

	description, _ := data["description"].(string)
	orderID := strings.TrimPrefix(description, "Payment for ")

	status, _ := data["status"].(string)
	amount := data["amount"]
	paymentType, _ := data["type"].(string)

	// 3. Parse Order ID to get PaymentDueID
	// Format: payment-due-{id}-{timestamp}
	parts := strings.Split(orderID, "-")
	if len(parts) < 3 {
		// Fallback: search for session by Token if description doesn't match
		paymentID, _ := data["id"].(string)
		var session models.PaymentSession
		if err := h.db.Where("payment_gateway = ? AND response_metadata LIKE ?", models.PaymentGatewayMayar, "%"+paymentID+"%").Order("created_at desc").First(&session).Error; err == nil {
			orderID = session.OrderID
			parts = strings.Split(orderID, "-")
		} else {
			// return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid order ID format and session not found")
		}
	}

	dueIDStr := parts[2]
	dueID, err := strconv.ParseUint(dueIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID in order ID")
	}

	// 4. Fetch models.PaymentDue
	var due models.PaymentDue
	if err := h.db.First(&due, uint(dueID)).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 5. Handle status
	grossAmount := "0"
	if amount != nil {
		grossAmount = fmt.Sprintf("%v", amount)
	}

	h.paymentService.HandleTransactionStatus(&due, orderID, status, "", paymentType, grossAmount)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// HandleMarkAsComplete allows admins to manually mark a payment due as paid
func (h *PaymentDueHandler) HandleMarkAsComplete(c echo.Context) error {
	// 1. Authorization Check
	userType, ok := c.Get("userType").(models.UserType)
	if !ok || userType != models.UserTypeAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Only admins can perform this action")
	}

	id := c.Param("id")
	dueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID")
	}

	// 2. Fetch models.PaymentDue
	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 3. Mark as Paid using helper
	if due.PaymentStatus != models.PaymentStatusPaid {
		h.paymentService.MarkAsPaid(&due, map[string]interface{}{
			"payment_type":    "manual",
			"gross_amount":    due.CalculatedPayAmount,
			"payment_gateway": string(models.PaymentGatewayManual), // Pass as string, helper converts back
		})
	}

	// 4. Return updated component
	// Re-fetch to get fresh state if needed, though markAsPaid updates the struct pointer
	// But we need relations for the template
	if err := h.db.Preload("Plan").Preload("User").First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to refresh payment due")
	}

	currentUserID := middleware.GetUint(c, "userID")
	// Retrieve display mode from query or default
	displayMode := c.QueryParam("display_mode")
	if displayMode == "" {
		displayMode = "admin" // Assuming admin view since admin triggers it
	}

	return payment_pages.PaymentDueItem(due, displayMode, currentUserID, models.UserTypeAdmin).Render(c.Request().Context(), c.Response())
}

// CheckPaymentStatus checks the status of a payment due with Midtrans
func (h *PaymentDueHandler) CheckPaymentStatus(c echo.Context) error {
	id := c.Param("id")
	dueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID")
	}

	displayMode := c.QueryParam("display_mode")
	if displayMode == "" {
		displayMode = "user" // Default fallback
	}
	currentUserID := middleware.GetUint(c, "userID")

	// Use PaymentService to verify status
	if err := h.paymentService.VerifyPaymentStatus(uint(dueID)); err != nil {
		// Log error but proceed to show current state, or return error?
		// For now, let's proceed so user sees something even if check failed (e.g. network issue)
		// Or maybe return error to let user know check failed.
		// h.paymentService.VerifyPaymentStatus already handles common errors.
	}

	// 4. Reload models.PaymentDue with Associations for Rendering
	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	var currentUserType models.UserType
	if val := c.Get("userType"); val != nil {
		if ut, ok := val.(models.UserType); ok {
			currentUserType = ut
		}
	}

	return payment_pages.PaymentDueItem(due, displayMode, currentUserID, currentUserType).Render(c.Request().Context(), c.Response())
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
