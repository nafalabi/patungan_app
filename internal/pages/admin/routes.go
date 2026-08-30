package admin

import (
	"github.com/labstack/echo/v4"

	payment "patungan_app_echo/internal/modules/payment"
	paymentpages "patungan_app_echo/internal/pages/admin/payment"
)

type Deps struct {
	Payments *payment.Service
}

func RegisterRoutes(e *echo.Echo, adminGroup *echo.Group, deps Deps) {
	h := paymentpages.NewPaymentDueHandler(deps.Payments)

	// Webhooks live outside the admin group
	e.POST("/payments/callback/midtrans", h.MidtransCallback)
	e.POST("/payments/callback/mayar", h.MayarCallback)

	adminGroup.GET("/payment-dues", h.ListPaymentDues)
	adminGroup.GET("/payments/:id/status", h.CheckPaymentStatus)
	adminGroup.POST("/payments/:id/mark-complete", h.HandleMarkAsComplete)
}
