package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type PaymentGateway string

const (
	PaymentGatewayMidtrans PaymentGateway = "midtrans"
	PaymentGatewayManual   PaymentGateway = "manual"
)

type PaymentCallbackHistory struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	PaymentGateway PaymentGateway  `gorm:"type:varchar(50);not null" json:"payment_gateway"`
	Metadata       json.RawMessage `gorm:"type:jsonb" json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}
