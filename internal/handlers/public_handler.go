package handlers

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/web/templates/pages"

	"github.com/midtrans/midtrans-go"
)

type PublicHandler struct {
	db             *gorm.DB
	cache          *services.RedisCache
	midtransClient *services.MidtransService
	paymentService *services.PaymentService
}

func NewPublicHandler(db *gorm.DB, cache *services.RedisCache, midtransClient *services.MidtransService, paymentService *services.PaymentService) *PublicHandler {
	if midtransClient == nil {
		// Initialize Midtrans if not provided (fallback)
		midtransClient = services.NewMidtransService()
	}
	return &PublicHandler{db: db, cache: cache, midtransClient: midtransClient, paymentService: paymentService}
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

	props := pages.PublicPaymentDueProps{
		Title:             "Payment Due Details",
		Due:               due,
		MidtransClientKey: midtrans.ClientKey,
	}

	return pages.PublicPaymentDue(props).Render(c.Request().Context(), c.Response())
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

	// Initiate Payment using PaymentService
	forceNew := c.QueryParam("force_new") == "true"
	callbackURL := getEnv("APP_URL", "http://localhost:8080") + "/p/" + uuid

	result, err := h.paymentService.InitiatePayment(&due, forceNew, callbackURL)
	if err != nil {
		if err.Error() == "payment already made" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Payment is already made. Please refresh the page."})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to initiate payment: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":        result.Token,
		"redirect_url": result.RedirectURL,
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
