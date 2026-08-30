package admin

import (
	"github.com/labstack/echo/v4"

	payment "patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	settings "patungan_app_echo/internal/modules/settings"
	user "patungan_app_echo/internal/modules/user"
	paymentpages "patungan_app_echo/internal/pages/admin/payment"
	planpages "patungan_app_echo/internal/pages/admin/plan"
	settingspages "patungan_app_echo/internal/pages/admin/settings"
	userpages "patungan_app_echo/internal/pages/admin/user"
)

type Deps struct {
	Payments *payment.Service
	Plans    *plan.Service
	Users    *user.Service
	Settings *settings.Service
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

	uh := userpages.NewUserHandler(deps.Users)
	adminGroup.GET("/users", uh.ListUsers)
	adminGroup.GET("/users/create", uh.CreateUserPage)
	adminGroup.POST("/users", uh.StoreUser)
	adminGroup.GET("/users/:id/edit", uh.EditUserPage)
	adminGroup.POST("/users/:id/update", uh.UpdateUser)
	adminGroup.POST("/users/:id/delete", uh.DeleteUser)

	// Admin User Preference (HTMX)
	adminGroup.GET("/users/:id/preference", uh.GetUserPreference)
	adminGroup.PUT("/users/:id/preference", uh.UpdateUserPreference)

	sh := settingspages.NewSettingsHandler(deps.Settings)
	adminGroup.GET("/settings", sh.GetSettings)
	adminGroup.POST("/settings", sh.UpdateSettings)
}
