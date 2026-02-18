package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

type PaymentDueHandler struct {
	db             *gorm.DB
	cache          *services.RedisCache
	midtransClient *services.MidtransService
	paymentService *services.PaymentService
}

func NewPaymentDueHandler(db *gorm.DB, cache *services.RedisCache, midtransClient *services.MidtransService, paymentService *services.PaymentService) *PaymentDueHandler {
	return &PaymentDueHandler{db: db, cache: cache, midtransClient: midtransClient, paymentService: paymentService}
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
	var planWithDues []pages.PlanWithDues
	var userWithDues []pages.UserWithDues
	var flatDues []models.PaymentDue

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
	} else if viewMode == "users" {
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

	props := pages.PaymentDuesProps{
		Title:         "Payment Dues",
		ActiveNav:     "payment-dues",
		Breadcrumbs:   breadcrumbs,
		UserEmail:     getStringFromContext(c, "userEmail"),
		UserUIDString: getStringFromContext(c, "userUID"),
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
		CurrentUserID:     getUintFromContext(c, "userID"),
		CurrentUserType:   currentUserType,
		MidtransClientKey: midtrans.ClientKey,
	}

	return pages.PaymentDues(props).Render(c.Request().Context(), c.Response())
}

// InitiatePayment handles the creation of a Snap transaction
func (h *PaymentDueHandler) InitiatePayment(c echo.Context) error {
	id := c.Param("id")
	dueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID")
	}

	// 1. Fetch PaymentDue
	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 2. Validate Ownership
	currentUserID := getUintFromContext(c, "userID")
	if due.UserID != currentUserID {
		return echo.NewHTTPError(http.StatusForbidden, "You can only pay for your own dues")
	}

	// 3. Check if already paid
	if due.PaymentStatus == models.PaymentStatusPaid {
		return echo.NewHTTPError(http.StatusBadRequest, "Payment due is already paid")
	}

	// 4. Initiate Payment using PaymentService
	forceNew := c.QueryParam("force_new") == "true"
	callbackURL := getEnv("APP_URL", "http://localhost:8080") + "/payment-dues"

	result, err := h.paymentService.InitiatePayment(&due, forceNew, callbackURL)
	if err != nil {
		if err.Error() == "payment already made" {
			// Specific handling for already paid
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Payment is already made. Please refresh the page."})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to initiate payment: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":        result.Token,
		"redirect_url": result.RedirectURL,
	})
}

// CheckActiveSession checks if there is an active payment session for a due
func (h *PaymentDueHandler) CheckActiveSession(c echo.Context) error {
	id := c.Param("id")
	dueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID")
	}

	session, err := h.paymentService.CheckActiveSession(uint(dueID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check session: "+err.Error())
	}

	if session != nil {
		var midtransResp snap.Response
		if err := json.Unmarshal(session.ResponseMetadata, &midtransResp); err == nil {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"active": true,
				"token":  midtransResp.Token,
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"active": false,
	})
}

// MidtransCallback handles validation of Midtrans notifications
func (h *PaymentDueHandler) MidtransCallback(c echo.Context) error {
	var notificationPayload map[string]interface{}
	if err := c.Bind(&notificationPayload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON payload")
	}

	// Log to PaymentCallbackHistory
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
	signatureKey, _ := notificationPayload["signature_key"].(string)
	statusCode, _ := notificationPayload["status_code"].(string)
	grossAmount, _ := notificationPayload["gross_amount"].(string)

	if !h.midtransClient.VerifySignature(signatureKey, orderID, statusCode, grossAmount) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid Signature")
	}

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

	// Fetch PaymentDue
	var due models.PaymentDue
	if err := h.db.First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// Handle status
	h.handleTransactionStatus(&due, orderID, transactionStatus, fraudStatus, notificationPayload["payment_type"].(string), notificationPayload["gross_amount"].(string))

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *PaymentDueHandler) handleTransactionStatus(due *models.PaymentDue, orderID, transactionStatus, fraudStatus, paymentType, grossAmount string) {
	switch transactionStatus {
	case "capture":
		switch fraudStatus {
		case "accept":
			h.markAsPaid(due, map[string]interface{}{
				"payment_type": paymentType,
				"gross_amount": grossAmount,
			})
		case "deny", "challenge":
			// do nothing
		}
	case "settlement":
		h.markAsPaid(due, map[string]interface{}{
			"payment_type": paymentType,
			"gross_amount": grossAmount,
		})
	case "deny", "expire", "cancel", "failure":
		var session models.PaymentSession
		if err := h.db.Where("order_id = ?", orderID).First(&session).Error; err == nil {
			session.IsActive = false
			h.db.Save(&session)
		}
	}
}

func (h *PaymentDueHandler) markAsPaid(due *models.PaymentDue, payload map[string]interface{}) {
	if due.PaymentStatus == models.PaymentStatusPaid {
		return
	}

	// 1. Update PaymentDue status
	due.PaymentStatus = models.PaymentStatusPaid
	h.db.Save(due)

	// 2. Create UserPayment record
	paymentType, _ := payload["payment_type"].(string)
	paymentGatewayStr, ok := payload["payment_gateway"].(string)
	var paymentGateway models.PaymentGateway
	if ok {
		paymentGateway = models.PaymentGateway(paymentGatewayStr)
	} else {
		paymentGateway = models.PaymentGatewayMidtrans // Default to midtrans for existing calls
	}

	// Helper to get float from interface safely
	// Gross amount is usually string in JSON payload from Midtrans?
	// Check doc: gross_amount is string.
	var grossAmt float64
	if val, ok := payload["gross_amount"].(string); ok {
		grossAmt, _ = strconv.ParseFloat(val, 64)
	} else if val, ok := payload["gross_amount"].(float64); ok {
		grossAmt = val
	}

	userPayment := models.UserPayment{
		PlanID:         due.PlanID,
		PaymentDueID:   due.ID,
		UserID:         due.UserID,
		TotalPay:       grossAmt,
		ChannelPayment: paymentType,
		PaymentGateway: paymentGateway,
		PaymentDate:    time.Now(),
	}
	h.db.Create(&userPayment)
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

	// 2. Fetch PaymentDue
	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").First(&due, dueID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 3. Mark as Paid using helper
	if due.PaymentStatus != models.PaymentStatusPaid {
		h.markAsPaid(&due, map[string]interface{}{
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

	currentUserID := getUintFromContext(c, "userID")
	// Retrieve display mode from query or default
	displayMode := c.QueryParam("display_mode")
	if displayMode == "" {
		displayMode = "admin" // Assuming admin view since admin triggers it
	}

	return pages.PaymentDueItem(due, displayMode, currentUserID, models.UserTypeAdmin).Render(c.Request().Context(), c.Response())
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
	currentUserID := getUintFromContext(c, "userID")

	// 1. Find latest active session for this due
	var session models.PaymentSession
	h.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&session)

	// 2. Call Midtrans Check Transaction only if we have a session
	if session.ID != 0 {
		resp, err := h.midtransClient.CheckTransaction(session.OrderID)
		if err == nil {
			// 3. Process Response & Update Local State
			var due models.PaymentDue
			if err := h.db.First(&due, dueID).Error; err == nil {
				h.handleTransactionStatus(&due, session.OrderID, resp.TransactionStatus, resp.FraudStatus, resp.PaymentType, resp.GrossAmount)
			}
		}
	}

	// 4. Reload PaymentDue with Associations for Rendering
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

	return pages.PaymentDueItem(due, displayMode, currentUserID, currentUserType).Render(c.Request().Context(), c.Response())
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
