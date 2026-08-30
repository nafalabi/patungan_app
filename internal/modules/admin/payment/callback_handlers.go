package payment

import (
	"encoding/json"
	"net/http"
	"patungan_app_echo/internal/models"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

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

	// 5. Verify status with Mayar server (Double Check)
	// For security, we don't rely on the callback payload and instead check directly with Mayar
	if err := h.paymentService.VerifyPaymentStatus(uint(dueID)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to verify payment status with Mayar")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
