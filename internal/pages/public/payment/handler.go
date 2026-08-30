package payment

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/models"
	payment "patungan_app_echo/internal/modules/payment"
)

type PublicHandler struct {
	payments *payment.Service
}

func NewPublicHandler(payments *payment.Service) *PublicHandler {
	return &PublicHandler{payments: payments}
}

// ShowPaymentDue renders the public payment due page
func (h *PublicHandler) ShowPaymentDue(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	due, err := h.payments.GetDueByUUID(uuid)
	if err != nil || due == nil {
		log.Printf("Failed to find payment due with UUID %s: %v", uuid, err)
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	props := PublicPaymentDueProps{
		Title:             "Payment Due Details",
		Due:               *due,
		MidtransClientKey: os.Getenv("MIDTRANS_CLIENT_KEY"),
	}

	return PublicPaymentDue(props).Render(c.Request().Context(), c.Response())
}

// InitiatePayment handles the creation of a Snap transaction for public access
func (h *PublicHandler) InitiatePayment(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	due, err := h.payments.GetDueModelByUUID(uuid)
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	if due.PaymentStatus == models.PaymentStatusPaid {
		return echo.NewHTTPError(http.StatusBadRequest, "Payment due is already paid")
	}
	if due.PaymentStatus == models.PaymentStatusCanceled {
		return echo.NewHTTPError(http.StatusBadRequest, "Payment due has been canceled")
	}

	// Initiate Payment
	forceNew := c.QueryParam("force_new") == "true"
	callbackURL := getEnv("APP_URL", "http://localhost:8080") + "/p/" + uuid

	result, err := h.payments.InitiatePayment(payment.InitiatePaymentRequest{
		Due:         due,
		ForceNew:    forceNew,
		CallbackURL: callbackURL,
	})
	if err != nil {
		if errors.Is(err, payment.ErrAlreadyPaid) {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Payment is already made. Please check the status."})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to initiate payment: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":        result.Token,
		"redirect_url": result.RedirectURL,
		"gateway":      result.Gateway,
	})
}

// CheckActiveSession checks if there is an active payment session for a public due
func (h *PublicHandler) CheckActiveSession(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	due, err := h.payments.GetDueByUUID(uuid)
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	session, err := h.payments.CheckActiveSession(due.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check session: "+err.Error())
	}

	active := session != nil
	return c.JSON(http.StatusOK, map[string]interface{}{
		"active": active,
	})
}

// CheckStatus checks the payment status and returns the current state
func (h *PublicHandler) CheckStatus(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	due, err := h.payments.GetDueByUUID(uuid)
	if err != nil || due == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// Verify status; proceed to return current status from DB on failure
	if err := h.payments.VerifyPaymentStatus(due.ID); err != nil {
		log.Printf("Failed to verify payment status for due %d: %v", due.ID, err)
	}

	// Re-fetch to get latest status
	latest, err := h.payments.GetDueByUUID(uuid)
	if err != nil || latest == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch payment due")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":         latest.Status,
		"payment_status": latest.Status, // redundancy for frontend convenience
	})
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
