package models

import (
	"time"

	"gorm.io/gorm"
)

// UserPayment records a payment made by a specific user for a specific due
type UserPayment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	PlanID         uint           `gorm:"index" json:"plan_id"`
	PaymentDueID   uint           `gorm:"index" json:"payment_due_id"`
	UserID         uint           `gorm:"index" json:"user_id"`
	TotalPay       float64        `gorm:"type:decimal(15,2)" json:"total_pay"`
	PaymentGateway PaymentGateway `gorm:"type:varchar(50)" json:"payment_gateway"`  // e.g., "midtrans", "manual"
	ChannelPayment string         `gorm:"type:varchar(100)" json:"channel_payment"` // e.g., "bank_transfer", "e-wallet"
	PaymentDate    time.Time      `json:"payment_date"`

	// Relationships
	Plan       Plan       `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	PaymentDue PaymentDue `gorm:"foreignKey:PaymentDueID" json:"payment_due,omitempty"`
	User       User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Refunds    []Refund   `gorm:"foreignKey:UserPaymentID" json:"refunds,omitempty"`
}
