package models

import "gorm.io/gorm"

type Settings struct {
	gorm.Model
	ActivePaymentGateway PaymentGateway `gorm:"type:varchar(50);default:'midtrans'" json:"active_payment_gateway"`
	
	// Midtrans Config
	MidtransMerchantID   string `json:"midtrans_merchant_id"`
	MidtransServerKey    string `json:"midtrans_server_key"`
	MidtransClientKey    string `json:"midtrans_client_key"`
	MidtransIsProduction bool   `gorm:"default:false" json:"midtrans_is_production"`
	
	// Mayar Config
	MayarAPIKey          string `json:"mayar_api_key"`
	MayarIsProduction    bool   `gorm:"default:false" json:"mayar_is_production"`
}
