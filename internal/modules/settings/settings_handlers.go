package settings

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"patungan_app_echo/internal/models"
	settings_pages "patungan_app_echo/internal/modules/settings/pages"
	"patungan_app_echo/internal/services/payment_gateway"
	"patungan_app_echo/internal/shared/utils"
	types "patungan_app_echo/internal/template/types"
)

type SettingsHandler struct {
	db             *gorm.DB
	gatewayManager *payment_gateway.GatewayManager
}

func NewSettingsHandler(db *gorm.DB, gatewayManager *payment_gateway.GatewayManager) *SettingsHandler {
	return &SettingsHandler{db: db, gatewayManager: gatewayManager}
}

func (h *SettingsHandler) GetSettings(c echo.Context) error {
	// Authorization Check
	userType, ok := c.Get("userType").(models.UserType)
	if !ok || userType != models.UserTypeAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Only admins can access settings")
	}

	settings, err := h.gatewayManager.GetSettings()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch settings")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "models.Settings", URL: ""},
	}

	props := settings_pages.PaymentSettingsProps{
		Title:       "Application models.Settings",
		ActiveNav:   "settings",
		Breadcrumbs: breadcrumbs,
		UserEmail:   utils.GetStringFromContext(c, "userEmail"),
		Settings:    *settings,
	}

	return settings_pages.PaymentSettings(props).Render(c.Request().Context(), c.Response())
}

func (h *SettingsHandler) UpdateSettings(c echo.Context) error {
	// Authorization Check
	userType, ok := c.Get("userType").(models.UserType)
	if !ok || userType != models.UserTypeAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Only admins can update settings")
	}

	settings, err := h.gatewayManager.GetSettings()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch settings")
	}

	// Update fields from form
	settings.ActivePaymentGateway = models.PaymentGateway(c.FormValue("active_payment_gateway"))
	settings.MidtransMerchantID = c.FormValue("midtrans_merchant_id")
	settings.MidtransServerKey = c.FormValue("midtrans_server_key")
	settings.MidtransClientKey = c.FormValue("midtrans_client_key")
	settings.MidtransIsProduction = c.FormValue("midtrans_is_production") == "true"
	settings.MayarAPIKey = c.FormValue("mayar_api_key")
	settings.MayarIsProduction = c.FormValue("mayar_is_production") == "true"

	if err := h.db.Save(settings).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update settings")
	}

	// For HTMX, we could return a success message or redirect
	if c.Request().Header.Get("HX-Request") != "" {
		return c.String(http.StatusOK, "models.Settings updated successfully")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/settings")
}
