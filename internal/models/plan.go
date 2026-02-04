package models

import (
	"time"

	"gorm.io/gorm"
)

// Plan represents a subscription or payment plan
type Plan struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name                    string    `gorm:"type:varchar(255)" json:"name"`
	PlanStartDate           time.Time `json:"plan_start_date"`
	TotalPrice              float64   `gorm:"type:decimal(15,2)" json:"total_price"`
	PayInterval             string    `gorm:"type:varchar(50)" json:"pay_interval"` // e.g., "monthly", "weekly"
	IsActive                bool      `gorm:"default:true" json:"is_active"`
	AllowInvitationAfterPay bool      `gorm:"default:false" json:"allow_invitation_after_pay"`

	// Relationships
	Users       []User       `gorm:"many2many:plan_user;" json:"users,omitempty"`
	PaymentDues []PaymentDue `gorm:"foreignKey:PlanID" json:"payment_dues,omitempty"`
}
