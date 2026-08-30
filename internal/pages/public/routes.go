package public

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/models"
	paymentpages "patungan_app_echo/internal/pages/public/payment"
)

// RegisterRoutes registers the public payment-due routes and the root
// role-based redirect on the root router. requireAuth resolves the
// authenticated user (if any) for the root redirect only.
func RegisterRoutes(e *echo.Echo, h *paymentpages.PublicHandler, requireAuth echo.MiddlewareFunc) {
	e.GET("/p/:uuid", h.ShowPaymentDue)
	e.POST("/p/:uuid/initiate", h.InitiatePayment)
	e.GET("/p/:uuid/active-session", h.CheckActiveSession)
	e.GET("/p/:uuid/status", h.CheckStatus)

	// Redirect root to role-based dashboard
	e.GET("/", func(c echo.Context) error {
		userType, ok := c.Get("userType").(models.UserType)
		if ok && userType == models.UserTypeAdmin {
			return c.Redirect(http.StatusTemporaryRedirect, "/admin/dashboard")
		}
		return c.Redirect(http.StatusTemporaryRedirect, "/member/dashboard")
	}, requireAuth)
}
