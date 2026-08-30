package plan

import (
	"time"

	"patungan_app_echo/internal/models"
)

// ListParams carries the admin plans-list query (filters, sort, pagination).
type ListParams struct {
	FilterOwner uint
	FilterType  string
	SortBy      string
	SortOrder   string
	Page        int
	PageSize    int
}

// PlanSummary is the list-page projection of a plan.
type PlanSummary struct {
	ID         uint
	Name       string
	OwnerName  string
	TotalPrice float64
	StartDate  time.Time
}

// PlanDetail is the member/admin detail-page projection of a plan.
type PlanDetail struct {
	Plan         PlanSummary
	Participants []ParticipantView
}

// ParticipantView is one participant row on the detail page.
type ParticipantView struct {
	UserID  uint
	Name    string
	Email   string
	Portion int
}

// PeriodView is one billing period with its dues.
type PeriodView struct {
	ID      uint
	Name    string
	DueDate time.Time
	Dues    []DueView
}

// DueView is one due inside a billing period.
type DueView struct {
	ID       uint
	UUID     string
	Amount   float64
	Status   string
	UserID   uint
	UserName string
}

// UpdateInput carries the already-validated form fields for Update.
type UpdateInput struct {
	Name                    string
	TotalPrice              float64
	PaymentType             string
	RecurringInterval       *string
	PlanStartDate           time.Time
	AllowInvitationAfterPay bool
	Participants            []models.PlanParticipant
}
