package payment_gateway

import "encoding/json"

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
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
	Token        string          `json:"token"`
	RedirectURL  string          `json:"redirect_url"`
	RawResponse  json.RawMessage `json:"raw_response"` // To store the full gateway response
}

type TransactionStatusResponse struct {
	TransactionStatus TransactionStatus
	OrderID           string
	GrossAmount       string
	PaymentType       string
	FraudStatus       string
	RawResponse       json.RawMessage
}

type Gateway interface {
	CreateTransaction(req *PaymentRequest) (*PaymentResponse, error)
	CheckTransaction(orderID string) (*TransactionStatusResponse, error)
	CancelTransaction(orderID string) error
}
