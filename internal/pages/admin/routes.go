package admin

import (
	"github.com/labstack/echo/v4"

	payment "patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	paymentpages "patungan_app_echo/internal/pages/admin/payment"
	planpages "patungan_app_echo/internal/pages/admin/plan"
)

type Deps struct {
	Payments *payment.Service
	Plans    *plan.Service
}

func RegisterRoutes(e *echo.Echo, adminGroup *echo.Group, deps Deps) {
	h := paymentpages.NewPaymentDueHandler(deps.Payments)

	// Webhooks live outside the admin group
	e.POST("/payments/callback/midtrans", h.MidtransCallback)
	e.POST("/payments/callback/mayar", h.MayarCallback)

	adminGroup.GET("/payment-dues", h.ListPaymentDues)
	adminGroup.GET("/payments/:id/status", h.CheckPaymentStatus)
	adminGroup.POST("/payments/:id/mark-complete", h.HandleMarkAsComplete)

	ph := planpages.NewPlanHandler(deps.Plans)
	adminGroup.GET("/plans", ph.ListPlans)
	adminGroup.GET("/plans/create", ph.CreatePlanPage)
	adminGroup.POST("/plans", ph.StorePlan)
	adminGroup.GET("/plans/:id/edit", ph.EditPlanPage)
	adminGroup.POST("/plans/:id/update", ph.UpdatePlan)
	adminGroup.POST("/plans/:id/delete", ph.DeletePlan)
	adminGroup.GET("/plans/:id/schedule-popup", ph.GetSchedulePopup)
	adminGroup.POST("/plans/:id/schedule", ph.SchedulePlan)
	adminGroup.POST("/plans/:id/disable-schedule", ph.DisableSchedulePlan)
}
