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

	PlanID              uint    `gorm:"index" json:"plan_id"`
	UserID              uint    `gorm:"index" json:"user_id"`
	Portion             int     `json:"portion"`
	CalculatedPayAmount float64 `gorm:"type:decimal(15,2)" json:"calculated_pay_amount"`
	PaymentStatus       string  `gorm:"type:varchar(50)" json:"payment_status"` // e.g., "pending", "paid", "overdue"

	// Relationships
	Plan        Plan         `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	User        User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	UserPayment *UserPayment `gorm:"foreignKey:PaymentDueID" json:"user_payment,omitempty"`
	Refund      *Refund      `gorm:"foreignKey:PaymentDueID" json:"refund,omitempty"`
}
