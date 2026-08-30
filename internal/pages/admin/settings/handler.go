package settings

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	settings "patungan_app_echo/internal/modules/settings"
	types "patungan_app_echo/internal/template/types"
)

type SettingsHandler struct {
	settings *settings.Service
}

func NewSettingsHandler(settings *settings.Service) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) GetSettings(c echo.Context) error {
	// Authorization Check
	userType, ok := c.Get("userType").(models.UserType)
	if !ok || userType != models.UserTypeAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Only admins can access settings")
	}

	view, err := h.settings.Get()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch settings")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Settings", URL: ""},
	}

	props := PaymentSettingsProps{
		Title:       "Application Settings",
		ActiveNav:   "settings",
		Breadcrumbs: breadcrumbs,
		UserEmail:   middleware.GetString(c, "userEmail"),
		Settings:    view,
	}

	return PaymentSettings(props).Render(c.Request().Context(), c.Response())
}

func (h *SettingsHandler) UpdateSettings(c echo.Context) error {
	// Authorization Check
	userType, ok := c.Get("userType").(models.UserType)
	if !ok || userType != models.UserTypeAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Only admins can update settings")
	}

	input := settings.UpdateInput{
		ActiveGateway:        c.FormValue("active_payment_gateway"),
		MidtransMerchantID:   c.FormValue("midtrans_merchant_id"),
		MidtransServerKey:    c.FormValue("midtrans_server_key"),
		MidtransClientKey:    c.FormValue("midtrans_client_key"),
		MidtransIsProduction: c.FormValue("midtrans_is_production") == "true",
		MayarAPIKey:          c.FormValue("mayar_api_key"),
		MayarIsProduction:    c.FormValue("mayar_is_production") == "true",
	}

	if err := h.settings.Update(input); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update settings")
	}

	// For HTMX, we could return a success message or redirect
	if c.Request().Header.Get("HX-Request") != "" {
		return c.String(http.StatusOK, "Settings updated successfully")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/settings")
}
