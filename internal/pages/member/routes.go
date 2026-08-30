package member

import (
	"github.com/labstack/echo/v4"

	payment "patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	paymentpages "patungan_app_echo/internal/pages/member/payment"
	planpages "patungan_app_echo/internal/pages/member/plan"
)

type Deps struct {
	Payments *payment.Service
	Plans    *plan.Service
}

func RegisterRoutes(memberGroup *echo.Group, deps Deps) {
	memberGroup.GET("/payments", paymentpages.NewMemberPaymentHandler(deps.Payments).ListPayments)

	ph := planpages.NewMemberPlanHandler(deps.Plans)
	memberGroup.GET("/plans", ph.ListPlans)
	memberGroup.GET("/plans/:id", ph.ShowPlan)
}
