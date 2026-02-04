package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentDue represents a scheduled payment period for a plan
type PaymentDue struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	PlanID        uint   `gorm:"index" json:"plan_id"`
	PaymentPeriod string `gorm:"type:varchar(100)" json:"payment_period"` // e.g., "January 2026"
	PaymentStatus string `gorm:"type:varchar(50)" json:"payment_status"`  // e.g., "pending", "paid", "overdue"

	// Relationships
	Plan         Plan          `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	UserPayments []UserPayment `gorm:"foreignKey:PaymentDueID" json:"user_payments,omitempty"`
	Refunds      []Refund      `gorm:"foreignKey:PaymentDueID" json:"refunds,omitempty"`
}
