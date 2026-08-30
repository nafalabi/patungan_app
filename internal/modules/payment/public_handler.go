package payment

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"patungan_app_echo/internal/models"
	payment_pages "patungan_app_echo/internal/modules/payment/pages"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/services/payment_service"
)

type PublicHandler struct {
	db             *gorm.DB
	cache          *cache.RedisCache
	paymentService *payment_service.PaymentService
}

func NewPublicHandler(db *gorm.DB, cache *cache.RedisCache, paymentService *payment_service.PaymentService) *PublicHandler {
	return &PublicHandler{db: db, cache: cache, paymentService: paymentService}
}

// ShowPaymentDue renders the public payment due page
func (h *PublicHandler) ShowPaymentDue(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").Where("uuid = ?", uuid).First(&due).Error; err != nil {
		log.Printf("Failed to find payment due with UUID %s: %v", uuid, err)
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	props := payment_pages.PublicPaymentDueProps{
		Title:             "Payment Due Details",
		Due:               due,
		MidtransClientKey: os.Getenv("MIDTRANS_CLIENT_KEY"),
	}

	return payment_pages.PublicPaymentDue(props).Render(c.Request().Context(), c.Response())
}

// InitiatePayment handles the creation of a Snap transaction for public access
func (h *PublicHandler) InitiatePayment(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	var due models.PaymentDue
	if err := h.db.Preload("Plan").Preload("User").Where("uuid = ?", uuid).First(&due).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	if due.PaymentStatus == models.PaymentStatusPaid {
		return echo.NewHTTPError(http.StatusBadRequest, "Payment due is already paid")
	}
	if due.PaymentStatus == models.PaymentStatusCanceled {
		return echo.NewHTTPError(http.StatusBadRequest, "Payment due has been canceled")
	}

	// Initiate Payment using PaymentService
	forceNew := c.QueryParam("force_new") == "true"
	callbackURL := getEnv("APP_URL", "http://localhost:8080") + "/p/" + uuid

	result, err := h.paymentService.InitiatePayment(payment_service.InitiatePaymentRequest{
		Due:         &due,
		ForceNew:    forceNew,
		CallbackURL: callbackURL,
	})
	if err != nil {
		if err.Error() == "payment already made" {
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

	var due models.PaymentDue
	if err := h.db.Where("uuid = ?", uuid).First(&due).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	session, err := h.paymentService.CheckActiveSession(due.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check session: "+err.Error())
	}

	if session != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"active": true,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"active": false,
	})
}

// CheckStatus checks the payment status and returns the current state
func (h *PublicHandler) CheckStatus(c echo.Context) error {
	uuid := c.Param("uuid")
	if uuid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment due UUID")
	}

	var due models.PaymentDue
	if err := h.db.Where("uuid = ?", uuid).First(&due).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Payment due not found")
	}

	// Verify status with PaymentService
	if err := h.paymentService.VerifyPaymentStatus(due.ID); err != nil {
		// Log error but proceed to return current status from DB
		log.Printf("Failed to verify payment status for due %d: %v", due.ID, err)
	}

	// Re-fetch to get latest status
	if err := h.db.First(&due, due.ID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch payment due")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":         due.PaymentStatus,
		"payment_status": due.PaymentStatus, // redundancy for frontend convenience
	})
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
