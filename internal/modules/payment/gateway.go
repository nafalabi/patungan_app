package payment

import (
	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/payment_gateway"
)

// GatewayClient abstracts gateway orchestration so business logic can be tested.
type GatewayClient interface {
	CreateTransaction(req *payment_gateway.PaymentRequest, gatewayOverride models.PaymentGateway) (*payment_gateway.PaymentResponse, models.PaymentGateway, error)
	CheckTransaction(orderID string, gateway models.PaymentGateway) (*payment_gateway.TransactionStatusResponse, error)
	CancelTransaction(orderID string, gateway models.PaymentGateway) error
	GetTransactionIdentifier(gateway models.PaymentGateway, orderID string, responseMetadata []byte) string
}

type managerGatewayClient struct {
	m *payment_gateway.GatewayManager
}

// NewGatewayClient adapts the infra GatewayManager (services/ layer).
func NewGatewayClient(m *payment_gateway.GatewayManager) GatewayClient {
	return &managerGatewayClient{m: m}
}

func (c *managerGatewayClient) CreateTransaction(req *payment_gateway.PaymentRequest, o models.PaymentGateway) (*payment_gateway.PaymentResponse, models.PaymentGateway, error) {
	return c.m.CreateTransaction(req, o)
}
func (c *managerGatewayClient) CheckTransaction(orderID string, g models.PaymentGateway) (*payment_gateway.TransactionStatusResponse, error) {
	return c.m.CheckTransaction(orderID, g)
}
func (c *managerGatewayClient) CancelTransaction(orderID string, g models.PaymentGateway) error {
	return c.m.CancelTransaction(orderID, g)
}
func (c *managerGatewayClient) GetTransactionIdentifier(g models.PaymentGateway, orderID string, meta []byte) string {
	return c.m.GetTransactionIdentifier(g, orderID, meta)
}
