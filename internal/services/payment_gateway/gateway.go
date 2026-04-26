package payment_gateway

import (
	"encoding/json"
	"fmt"
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

type TransactionStatus string

const (
	StatusPending    TransactionStatus = "pending"
	StatusSettlement TransactionStatus = "settlement"
	StatusCapture    TransactionStatus = "capture"
	StatusDeny       TransactionStatus = "deny"
	StatusCancel     TransactionStatus = "cancel"
	StatusExpire     TransactionStatus = "expire"
	StatusFailure    TransactionStatus = "failure"
	StatusUnknown    TransactionStatus = "unknown"
)

type PaymentRequest struct {
	OrderID     string
	Amount      int64
	Customer    CustomerDetails
	Items       []ItemDetails
	CallbackURL string
}

type CustomerDetails struct {
	Name  string
	Email string
}

type ItemDetails struct {
	ID    string
	Name  string
	Price int64
	Qty   int
}

type PaymentResponse struct {
	Token       string
	RedirectURL string
	RawResponse []byte
}

type TransactionStatusResponse struct {
	TransactionStatus TransactionStatus
	OrderID           string
	GrossAmount       string
	PaymentType       string
	FraudStatus       string
	RawResponse       []byte
}

type Gateway interface {
	CreateTransaction(req *PaymentRequest) (*PaymentResponse, error)
	CheckTransaction(orderID string) (*TransactionStatusResponse, error)
	CancelTransaction(orderID string) error
	VerifyNotification(payload []byte, headers map[string]string) (bool, error)
}

// GatewayManager is a stateful object that orchestrates gateway selection
type GatewayManager struct {
	db *gorm.DB
}

func NewGatewayManager(db *gorm.DB) *GatewayManager {
	return &GatewayManager{db: db}
}

// GetSettings fetches the singleton settings record
func (m *GatewayManager) GetSettings() (*models.Settings, error) {
	var settings models.Settings
	if err := m.db.First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

// getInternalGateway fetches the current settings and returns the appropriate gateway implementation
func (m *GatewayManager) getInternalGateway(gatewayOverride models.PaymentGateway) (Gateway, models.PaymentGateway, error) {
	settings, err := m.GetSettings()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch gateway settings: %v", err)
	}

	selectedGateway := gatewayOverride
	if selectedGateway == "" {
		selectedGateway = settings.ActivePaymentGateway
	}

	switch selectedGateway {
	case models.PaymentGatewayMidtrans:
		return NewMidtransGateway(MidtransConfig{
			MerchantID:   settings.MidtransMerchantID,
			ServerKey:    settings.MidtransServerKey,
			ClientKey:    settings.MidtransClientKey,
			IsProduction: settings.MidtransIsProduction,
		}), selectedGateway, nil
	case models.PaymentGatewayMayar:
		return NewMayarGateway(MayarConfig{
			APIKey:       settings.MayarAPIKey,
			IsProduction: settings.MayarIsProduction,
		}), selectedGateway, nil
	default:
		return nil, "", fmt.Errorf("unsupported gateway: %s", selectedGateway)
	}
}

func (m *GatewayManager) CreateTransaction(req *PaymentRequest, gatewayOverride models.PaymentGateway) (*PaymentResponse, models.PaymentGateway, error) {
	gw, selected, err := m.getInternalGateway(gatewayOverride)
	if err != nil {
		return nil, "", err
	}
	resp, err := gw.CreateTransaction(req)
	return resp, selected, err
}

func (m *GatewayManager) CheckTransaction(orderID string, gateway models.PaymentGateway) (*TransactionStatusResponse, error) {
	gw, _, err := m.getInternalGateway(gateway)
	if err != nil {
		return nil, err
	}
	return gw.CheckTransaction(orderID)
}

func (m *GatewayManager) CancelTransaction(orderID string, gateway models.PaymentGateway) error {
	gw, _, err := m.getInternalGateway(gateway)
	if err != nil {
		return err
	}
	return gw.CancelTransaction(orderID)
}

func (m *GatewayManager) VerifyNotification(payload []byte, headers map[string]string, gateway models.PaymentGateway) (bool, error) {
	gw, _, err := m.getInternalGateway(gateway)
	if err != nil {
		return false, err
	}
	return gw.VerifyNotification(payload, headers)
}

// GetTransactionIdentifier returns the correct identifier for a transaction based on the gateway.
// Some gateways (like Mayar) use their own internal IDs for status checks.
func (m *GatewayManager) GetTransactionIdentifier(gateway models.PaymentGateway, orderID string, responseMetadata []byte) string {
	if gateway == models.PaymentGatewayMayar {
		var metadata struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(responseMetadata, &metadata); err == nil && metadata.Token != "" {
			return metadata.Token
		}
	}
	return orderID
}
