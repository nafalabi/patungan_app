package payment

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	payment "patungan_app_echo/internal/modules/payment"
	types "patungan_app_echo/internal/template/types"
)

type PaymentDueHandler struct {
	payments *payment.Service
}

func NewPaymentDueHandler(payments *payment.Service) *PaymentDueHandler {
	return &PaymentDueHandler{payments: payments}
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

	res, err := h.payments.ListDuesFlat(payment.ListFlatParams{
		FilterPlan: parseUintParam(c.QueryParam("filter_plan")),
		FilterUser: parseUintParam(c.QueryParam("filter_user")),
		SortBy:     sortBy,
		SortOrder:  sortOrder,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	return h.renderPage(c, "all", PaymentDuesProps{
		FlatDues:    res.Dues,
		CurrentPage: res.CurrentPage,
		TotalPages:  res.TotalPages,
		TotalCount:  res.TotalCount,
		PageSize:    res.PageSize,
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

	planDues, nextOffset, err := h.payments.ListDuesByPlans(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	// If HTMX partial, render just the list items and the new OOB button
	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := PlanListItems(planDues, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return LoadMorePlansButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "plans", PaymentDuesProps{
		PlanDues:   planDues,
		NextOffset: nextOffset,
	})
}

// renderByPeriods handles grouping by period, then plans
func (h *PaymentDueHandler) renderByPeriods(c echo.Context) error {
	limit := 3
	offset := 0
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
		offset = o
	}

	periodPlans, nextOffset, err := h.payments.ListDuesByPeriods(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := PeriodListItems(periodPlans, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return LoadMorePeriodsButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "periods", PaymentDuesProps{
		PeriodPlans: periodPlans,
		NextOffset:  nextOffset,
	})
}

// renderByUsers handles grouping by users, then plans
func (h *PaymentDueHandler) renderByUsers(c echo.Context) error {
	limit := 5
	offset := 0
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil {
		offset = o
	}

	userDues, nextOffset, err := h.payments.ListDuesByUsers(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		userID := middleware.GetUint(c, "userID")
		userType := h.getUserType(c)
		if err := UserListItems(userDues, userID, userType).Render(c.Request().Context(), c.Response()); err != nil {
			return err
		}
		return LoadMoreUsersButton(nextOffset).Render(c.Request().Context(), c.Response())
	}

	return h.renderPage(c, "users", PaymentDuesProps{
		UserDues:   userDues,
		NextOffset: nextOffset,
	})
}

// Helper to fill common props and render
func (h *PaymentDueHandler) renderPage(c echo.Context, viewMode string, props PaymentDuesProps) error {
	filterPlan := parseUintParam(c.QueryParam("filter_plan"))
	filterUser := parseUintParam(c.QueryParam("filter_user"))

	filterOpts, err := h.payments.FilterOptions()
	if err != nil {
		filterOpts = payment.FilterOptions{}
	}

	props.Title = "Payment Dues"
	props.ActiveNav = "payment-dues"
	props.Breadcrumbs = []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Payment Dues", URL: "/admin/payment-dues"},
	}
	props.UserEmail = middleware.GetString(c, "userEmail")
	props.UserUIDString = middleware.GetString(c, "userUID")
	props.CurrentUserID = middleware.GetUint(c, "userID")
	props.CurrentUserType = h.getUserType(c)
	props.MidtransClientKey = os.Getenv("MIDTRANS_CLIENT_KEY")
	props.ViewMode = viewMode
	props.FilterPlan = filterPlan
	props.FilterUser = filterUser
	props.AllPlans = filterOpts.Plans
	props.AllUsers = filterOpts.Users

	return PaymentDues(props).Render(c.Request().Context(), c.Response())
}

func (h *PaymentDueHandler) getUserType(c echo.Context) models.UserType {
	if val := c.Get("userType"); val != nil {
		if ut, ok := val.(models.UserType); ok {
			return ut
		}
	}
	return models.UserTypeMember
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

	// 2. Mark as Paid (skip if already paid)
	if err := h.payments.MarkDueComplete(uint(dueID)); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 3. Return updated component
	due, err := h.payments.GetDueForRender(uint(dueID))
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to refresh payment due")
	}

	currentUserID := middleware.GetUint(c, "userID")
	// Retrieve display mode from query or default
	displayMode := c.QueryParam("display_mode")
	if displayMode == "" {
		displayMode = "admin" // Assuming admin view since admin triggers it
	}

	return PaymentDueItem(*due, displayMode, currentUserID, models.UserTypeAdmin).Render(c.Request().Context(), c.Response())
}

// CheckPaymentStatus checks the status of a payment due with the gateway
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

	// Verify with the gateway; proceed to show current state even if the check fails
	_ = h.payments.VerifyPaymentStatus(uint(dueID))

	// Reload due with associations for rendering
	due, err := h.payments.GetDueForRender(uint(dueID))
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	currentUserType := h.getUserType(c)

	return PaymentDueItem(*due, displayMode, currentUserID, currentUserType).Render(c.Request().Context(), c.Response())
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
	_ = h.payments.CreateCallbackHistory(&history)

	// Extract necessary fields
	orderID, _ := notificationPayload["order_id"].(string)
	transactionStatus, _ := notificationPayload["transaction_status"].(string)
	fraudStatus, _ := notificationPayload["fraud_status"].(string)

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
	due, err := h.payments.GetDueModelByID(uint(dueID))
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// Handle status
	h.payments.HandleTransactionStatus(due, orderID, transactionStatus, fraudStatus, notificationPayload["payment_type"].(string), notificationPayload["gross_amount"].(string))

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
	_ = h.payments.CreateCallbackHistory(&history)

	// 2. Extract Data (Mayar often wraps in a 'data' field)
	data, ok := notificationPayload["data"].(map[string]interface{})
	if !ok {
		data = notificationPayload
	}

	description, _ := data["description"].(string)
	if description == "" {
		description, _ = data["productDescription"].(string)
	}
	orderID := strings.TrimPrefix(description, "Payment for ")

	// 3. Parse Order ID to get PaymentDueID
	// Format: payment-due-{id}-{timestamp}
	parts := strings.Split(orderID, "-")
	if len(parts) < 3 {
		// Fallback: search for session by Token if description doesn't match
		paymentID, _ := data["id"].(string)
		session, err := h.payments.FindLatestByGatewayMetadata(models.PaymentGatewayMayar, paymentID)
		if err == nil && session != nil {
			orderID = session.OrderID
			parts = strings.Split(orderID, "-")
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid order ID format and session not found")
		}
	}

	dueIDStr := parts[2]
	dueID, err := strconv.ParseUint(dueIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due ID in order ID")
	}

	// 4. Fetch PaymentDue
	due, err := h.payments.GetDueModelByID(uint(dueID))
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// 5. Verify status with Mayar server (Double Check)
	if err := h.payments.VerifyPaymentStatus(uint(dueID)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to verify payment status with Mayar")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func parseUintParam(s string) uint {
	if v, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint(v)
	}
	return 0
}
