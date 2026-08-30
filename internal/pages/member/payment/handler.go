package payment

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	payment "patungan_app_echo/internal/modules/payment"
	types "patungan_app_echo/internal/template/types"
)

type MemberPaymentHandler struct {
	payments *payment.Service
}

func NewMemberPaymentHandler(payments *payment.Service) *MemberPaymentHandler {
	return &MemberPaymentHandler{payments: payments}
}

// ListPayments renders the member's payment dues list
func (h *MemberPaymentHandler) ListPayments(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")
	statusFilter := c.QueryParam("status")

	dues, totalPending, totalPaid, err := h.payments.MemberDuesSummary(userID, statusFilter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Payments", URL: ""},
	}

	props := MemberPaymentsProps{
		Title:        "My Payments",
		ActiveNav:    "payments",
		Breadcrumbs:  breadcrumbs,
		UserEmail:    userEmail,
		UserUID:      userUID,
		StatusFilter: statusFilter,
		PaymentDues:  dues,
		TotalPending: totalPending,
		TotalPaid:    totalPaid,
	}

	return MemberPayments(props).Render(c.Request().Context(), c.Response())
}
