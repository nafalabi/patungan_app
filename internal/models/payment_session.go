package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type PaymentSession struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	PlanID           uint            `json:"plan_id"`
	PaymentDueID     uint            `json:"payment_due_id"`
	UserID           uint            `json:"user_id"`
	PaymentGateway   PaymentGateway  `gorm:"type:varchar(50);not null" json:"payment_gateway"`
	OrderID          string          `gorm:"type:varchar(100);index" json:"order_id"`
	IsActive         bool            `gorm:"default:true" json:"is_active"`
	RequestMetadata  json.RawMessage `gorm:"type:jsonb" json:"request_metadata"`
	ResponseMetadata json.RawMessage `gorm:"type:jsonb" json:"response_metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}
