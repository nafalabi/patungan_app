package payment

import (
	"net/http"
	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	payment_pages "patungan_app_echo/internal/modules/members/payment/pages"
	"patungan_app_echo/internal/services/payment_service"
	types "patungan_app_echo/internal/template/types"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type MemberPaymentHandler struct {
	db             *gorm.DB
	paymentService *payment_service.PaymentService
}

func NewMemberPaymentHandler(db *gorm.DB, paymentService *payment_service.PaymentService) *MemberPaymentHandler {
	return &MemberPaymentHandler{db: db, paymentService: paymentService}
}

// ListPayments renders the member's payment dues list
func (h *MemberPaymentHandler) ListPayments(c echo.Context) error {
	userID := middleware.GetUint(c, "userID")
	userEmail := middleware.GetString(c, "userEmail")
	userUID := middleware.GetString(c, "userUID")
	statusFilter := c.QueryParam("status")

	query := h.db.Where("user_id = ?", userID).Preload("Plan")
	if statusFilter == "pending" {
		query = query.Where("payment_status = ?", models.PaymentStatusPending)
	} else if statusFilter == "paid" {
		query = query.Where("payment_status = ?", models.PaymentStatusPaid)
	}

	var dues []models.PaymentDue
	if err := query.Order("due_date DESC").Find(&dues).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load payment dues")
	}

	// Calculate totals for summary cards
	var totalPending int64
	var totalPaid int64

	var allUserDues []models.PaymentDue
	h.db.Where("user_id = ?", userID).Find(&allUserDues)
	for _, d := range allUserDues {
		if d.PaymentStatus == models.PaymentStatusPending {
			totalPending += d.CalculatedPayAmount
		} else if d.PaymentStatus == models.PaymentStatusPaid {
			totalPaid += d.CalculatedPayAmount
		}
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/member/dashboard"},
		{Title: "My Payments", URL: ""},
	}

	props := payment_pages.MemberPaymentsProps{
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

	return payment_pages.MemberPayments(props).Render(c.Request().Context(), c.Response())
}
