package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"patungan_app_echo/internal/models"
	payment_pages "patungan_app_echo/internal/modules/payment/pages"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/services/payment_service"
	"patungan_app_echo/internal/shared/utils"
	shared "patungan_app_echo/web/templates/shared"
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

// ListPaymentDues renders the list of payment dues with filtering and sorting
func (h *PaymentDueHandler) ListPaymentDues(c echo.Context) error {
	// Parse query parameters
	viewMode := c.QueryParam("view")
	if viewMode == "" {
		viewMode = "all"
	}

	filterPlanStr := c.QueryParam("filter_plan")
	filterUserStr := c.QueryParam("filter_user")
	showCanceled := c.QueryParam("show_canceled") == "true"
	sortBy := c.QueryParam("sort_by")
	if sortBy == "" {
		sortBy = "created_at"
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
	// Hide canceled dues by default
	if !showCanceled {
		query = query.Where("payment_status != ?", models.PaymentStatusCanceled)
	}

	// Get total count for pagination
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to count payment dues")
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
	case "created_at":
		query = query.Order("created_at " + sortOrder)
	default:
		query = query.Order("id " + sortOrder)
	}

	// Apply pagination
	query = query.Limit(pageSize).Offset(offset)

	var paymentDues []models.PaymentDue
	if err := query.Find(&paymentDues).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch payment dues")
	}

	// Fetch all plans and users for filter dropdowns
	var allPlans []models.Plan
	var allUsers []models.User
	h.db.Find(&allPlans)
	h.db.Find(&allUsers)

	// Group data based on view mode
	// Group data based on view mode
	var planWithDues []payment_pages.PlanWithDues
	var userWithDues []payment_pages.UserWithDues
	var flatDues []models.PaymentDue

	if viewMode == "plans" {
		// Group by plans
		planMap := make(map[uint]*payment_pages.PlanWithDues)
		for _, due := range paymentDues {
			if _, exists := planMap[due.PlanID]; !exists {
				planMap[due.PlanID] = &payment_pages.PlanWithDues{
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
	} else if viewMode == "users" {
		// Group by users
		userMap := make(map[uint]*payment_pages.UserWithDues)
		for _, due := range paymentDues {
			if _, exists := userMap[due.UserID]; !exists {
				userMap[due.UserID] = &payment_pages.UserWithDues{
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
	} else {
		// Flat view (all)
		flatDues = paymentDues
	}

	// Breadcrumbs: Home > Payment Dues
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Payment Dues", URL: ""},
	}

	// Get current user type
	var currentUserType models.UserType
	if val := c.Get("userType"); val != nil {
		if ut, ok := val.(models.UserType); ok {
			currentUserType = ut
		}
	}

	props := payment_pages.PaymentDuesProps{
		Title:         "Payment Dues",
		ActiveNav:     "payment-dues",
		Breadcrumbs:   breadcrumbs,
		UserEmail:     utils.GetStringFromContext(c, "userEmail"),
		UserUIDString: utils.GetStringFromContext(c, "userUID"),
		PlanWithDues:  planWithDues,
		UserWithDues:  userWithDues,
		FlatDues:      flatDues,
		ViewMode:      viewMode,
		FilterPlan:    filterPlan,
		FilterUser:    filterUser,
		ShowCanceled:  showCanceled,
		SortBy:        sortBy,
		SortOrder:     sortOrder,
		AllPlans:      allPlans,
		AllUsers:      allUsers,
		// Pagination
		CurrentPage:       page,
		TotalPages:        totalPages,
		TotalCount:        int(totalCount),
		PageSize:          pageSize,
		CurrentUserID:     utils.GetUintFromContext(c, "userID"),
		CurrentUserType:   currentUserType,
		MidtransClientKey: os.Getenv("MIDTRANS_CLIENT_KEY"),
	}

	return payment_pages.PaymentDues(props).Render(c.Request().Context(), c.Response())
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

	currentUserID := utils.GetUintFromContext(c, "userID")
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
	currentUserID := utils.GetUintFromContext(c, "userID")

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
