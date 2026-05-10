package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentBillingPeriod groups payment dues for a specific billing cycle
type PaymentBillingPeriod struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	PlanID  uint      `gorm:"index" json:"plan_id"`
	Name    string    `gorm:"type:varchar(255)" json:"name"` // e.g., "May 2024"
	DueDate time.Time `gorm:"index" json:"due_date"`

	// Relationships
	Plan Plan         `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	Dues []PaymentDue `gorm:"foreignKey:PaymentBillingPeriodID" json:"dues,omitempty"`
}
