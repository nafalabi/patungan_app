package payment

import (
	"time"

	"patungan_app_echo/internal/models"
)

type PlanOption struct {
	ID         uint
	Name       string
	TotalPrice float64
}

type UserOption struct {
	ID    uint
	Name  string
	Email string
}

type PeriodOption struct {
	ID      uint
	Name    string
	DueDate time.Time
}

// DueItem is the flat view of one payment due for any dues list.
type DueItem struct {
	ID        uint
	UUID      string
	Portion   int
	DueDate   time.Time
	Amount    float64
	Status    string
	PlanID    uint
	PlanName  string
	UserID    uint
	UserName  string
	UserEmail string
	PeriodID  uint

	// Extra view fields kept so pages render exactly as before.
	PlanPaymentType string
	PeriodName      string
}

type PeriodDues struct {
	Period PeriodOption
	Dues   []DueItem
}

type PlanDuesGroup struct {
	Plan    PlanOption
	Periods []PeriodDues
}

type PlanDuesInPeriod struct {
	Plan PlanOption
	Dues []DueItem
}

type PeriodPlansGroup struct {
	Period PeriodOption
	Plans  []PlanDuesInPeriod
}

type UserDuesGroup struct {
	User UserOption
	Dues []DueItem
}

type FlatDuesResult struct {
	Dues        []DueItem
	CurrentPage int
	TotalPages  int
	TotalCount  int
	PageSize    int
}

type FilterOptions struct {
	Plans []PlanOption
	Users []UserOption
}

type ListFlatParams struct {
	FilterPlan uint
	FilterUser uint
	SortBy     string // "due_date" | "plan" | "user" | column name
	SortOrder  string // "asc" | "desc"
	Page       int
	PageSize   int
}

// AdminStats is the admin dashboard projection (global numbers).
type AdminStats struct {
	TotalActivePlans int
	PendingDuesCount int
	PendingAmount    float64
	PaidAmount       float64
	UpcomingDues     []DueItem
}

// UserStats is the member dashboard projection for one user's dues. The
// enrolled plans count is filled by the caller (plan.Service.EnrolledPlans).
type UserStats struct {
	ActivePlansCount int
	PendingCount     int
	PendingAmount    float64
	PaidThisMonth    float64
	PendingDues      []DueItem
}

func mapPlan(p models.Plan) PlanOption {
	return PlanOption{ID: p.ID, Name: p.Name, TotalPrice: p.TotalPrice}
}

func mapUser(u models.User) UserOption {
	return UserOption{ID: u.ID, Name: u.Name, Email: u.Email}
}

func mapPeriod(p models.PaymentBillingPeriod) PeriodOption {
	return PeriodOption{ID: p.ID, Name: p.Name, DueDate: p.DueDate}
}

func mapDue(d models.PaymentDue) DueItem {
	item := DueItem{
		ID: d.ID, UUID: d.UUID, Portion: d.Portion, DueDate: d.DueDate,
		Amount: d.CalculatedPayAmount, Status: d.PaymentStatus,
		PlanID: d.PlanID, UserID: d.UserID, PeriodID: d.PaymentBillingPeriodID,
	}
	if d.Plan.ID != 0 {
		item.PlanName = d.Plan.Name
		item.PlanPaymentType = d.Plan.PaymentType
	}
	if d.User.ID != 0 {
		item.UserName = d.User.Name
		item.UserEmail = d.User.Email
	}
	if d.BillingPeriod != nil && d.BillingPeriod.ID != 0 {
		item.PeriodName = d.BillingPeriod.Name
	}
	return item
}

func mapDues(dues []models.PaymentDue) []DueItem {
	items := make([]DueItem, 0, len(dues))
	for _, d := range dues {
		items = append(items, mapDue(d))
	}
	return items
}
