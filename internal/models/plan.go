package models

import (
	"time"

	"github.com/teambition/rrule-go"
	"gorm.io/gorm"
)

// Plan represents a subscription or payment plan
type Plan struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name              string    `gorm:"type:varchar(255)" json:"name"`
	TotalPrice        float64   `gorm:"type:decimal(15,2)" json:"total_price"`
	PlanStartDate     time.Time `json:"plan_start_date"`
	PaymentType       string    `gorm:"type:varchar(50);default:'onetime'" json:"payment_type"` // 'onetime' or 'recurring'
	RecurringInterval *string   `gorm:"type:text" json:"recurring_interval"`                    // RFC 5545 RRULE string

	IsActive                bool `gorm:"default:true" json:"is_active"`
	AllowInvitationAfterPay bool `gorm:"default:false" json:"allow_invitation_after_pay"`

	// Relationships
	Users       []User       `gorm:"many2many:plan_user;" json:"users,omitempty"`
	PaymentDues []PaymentDue `gorm:"foreignKey:PlanID" json:"payment_dues,omitempty"`
}

// NextDue calculates the next due date for the plan
func (p Plan) NextDue() time.Time {
	if p.PaymentType == "onetime" {
		return p.PlanStartDate
	}

	if p.RecurringInterval != nil && *p.RecurringInterval != "" {
		rule, err := rrule.StrToRRule(*p.RecurringInterval)
		if err == nil {
			rule.DTStart(p.PlanStartDate)
			// Find next occurrence after now (or include today)
			next := rule.After(time.Now().Add(-24*time.Hour), true)
			if !next.IsZero() {
				return next
			}
		}
	}
	// Fallback to start date if parsing fails or no future date found
	return p.PlanStartDate
}
