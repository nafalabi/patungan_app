package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
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
}

func NewPaymentDueHandler(db *gorm.DB, cache *services.RedisCache, midtransClient *services.MidtransService) *PaymentDueHandler {
	return &PaymentDueHandler{db: db, cache: cache, midtransClient: midtransClient}
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
	showCanceled := c.QueryParam("show_canceled") == "true"
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
		ShowCanceled: showCanceled,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
		AllPlans:     allPlans,
		AllUsers:     allUsers,
		// Pagination
		CurrentPage:       page,
		TotalPages:        totalPages,
		TotalCount:        int(totalCount),
		PageSize:          pageSize,
		CurrentUserID:     getUintFromContext(c, "userID"),
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

	// 3.5 Check for existing active session
	forceNew := c.QueryParam("force_new") == "true"
	var existingSession models.PaymentSession
	if err := h.db.Where("payment_due_id = ? AND is_active = ?", due.ID, true).Order("created_at desc").First(&existingSession).Error; err == nil {
		if forceNew {
			// Deactivate existing session
			existingSession.IsActive = false
			h.db.Save(&existingSession)
		} else {
			// Return existing session
			var midtransResp snap.Response
			if err := json.Unmarshal(existingSession.ResponseMetadata, &midtransResp); err == nil {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"token":        midtransResp.Token,
					"redirect_url": midtransResp.RedirectURL,
				})
			}
		}
	}

	// 4. Create Snap Transaction
	// Generate unique Order ID: payment-due-{id}-{timestamp}
	orderID := fmt.Sprintf("payment-due-%d-%d", due.ID, time.Now().Unix())

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(due.CalculatedPayAmount),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: due.User.Name,
			Email: due.User.Email,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    fmt.Sprintf("plan-%d", due.PlanID),
				Name:  fmt.Sprintf("Payment for %s", due.Plan.Name),
				Price: int64(due.CalculatedPayAmount),
				Qty:   1,
			},
		},
		// Callbacks: &snap.Callbacks{
		// 	Finish: "https://google.com",
		// },
	}

	resp, err := h.midtransClient.CreateTransaction(orderID, int64(due.CalculatedPayAmount), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create transaction: "+err.Error())
	}

	// 5. Create PaymentSession
	reqBytes, _ := json.Marshal(req)
	respBytes, _ := json.Marshal(resp)

	session := models.PaymentSession{
		PlanID:           due.PlanID,
		PaymentDueID:     due.ID,
		UserID:           currentUserID,
		PaymentGateway:   models.PaymentGatewayMidtrans,
		OrderID:          orderID,
		IsActive:         true,
		RequestMetadata:  reqBytes,
		ResponseMetadata: respBytes,
	}
	h.db.Create(&session)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":        resp.Token,
		"redirect_url": resp.RedirectURL,
	})
}

// CheckActiveSession checks if there is an active payment session for a due
func (h *PaymentDueHandler) CheckActiveSession(c echo.Context) error {
	id := c.Param("id")
	dueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID")
	}

	var existingSession models.PaymentSession
	if err := h.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&existingSession).Error; err == nil {
		var midtransResp snap.Response
		if err := json.Unmarshal(existingSession.ResponseMetadata, &midtransResp); err == nil {
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

	// Helper to get float from interface safely
	// Gross amount is usually string in JSON payload from Midtrans?
	// Check doc: gross_amount is string.
	grossAmtStr, _ := payload["gross_amount"].(string)
	grossAmt, _ := strconv.ParseFloat(grossAmtStr, 64)

	userPayment := models.UserPayment{
		PlanID:         due.PlanID,
		PaymentDueID:   due.ID,
		UserID:         due.UserID,
		TotalPay:       grossAmt,
		ChannelPayment: paymentType,
		PaymentDate:    time.Now(),
	}
	h.db.Create(&userPayment)
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

	return pages.PaymentDueItem(due, displayMode, currentUserID).Render(c.Request().Context(), c.Response())
}
