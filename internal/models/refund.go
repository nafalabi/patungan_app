package models

import (
	"time"

	"gorm.io/gorm"
)

// Refund records a refund issued to a user
type Refund struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	PlanID         uint           `gorm:"index" json:"plan_id"`
	PaymentDueID   uint           `gorm:"index" json:"payment_due_id"`
	UserPaymentID  uint           `gorm:"index" json:"user_payment_id"`
	UserID         uint           `gorm:"index" json:"user_id"`
	TotalRefund    float64        `gorm:"type:decimal(15,2)" json:"total_refund"`
	PaymentGateway PaymentGateway `gorm:"type:varchar(50)" json:"payment_gateway"`
	ChannelPayment string         `gorm:"type:varchar(100)" json:"channel_payment"`
	RefundDate     time.Time      `json:"refund_date"`

	// Relationships
	Plan        Plan        `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	PaymentDue  PaymentDue  `gorm:"foreignKey:PaymentDueID" json:"payment_due,omitempty"`
	UserPayment UserPayment `gorm:"foreignKey:UserPaymentID" json:"user_payment,omitempty"`
	User        User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
