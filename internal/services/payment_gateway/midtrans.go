package payment_gateway

import (
	"encoding/json"
	"fmt"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

type MidtransGateway struct {
	SnapClient snap.Client
	CoreClient coreapi.Client
	ServerKey  string
}

type MidtransConfig struct {
	MerchantID   string
	ServerKey    string
	ClientKey    string
	IsProduction bool
}

func NewMidtransGateway(config MidtransConfig) *MidtransGateway {
	env := midtrans.Sandbox
	if config.IsProduction {
		env = midtrans.Production
	}

	var s snap.Client
	s.New(config.ServerKey, env)

	var c coreapi.Client
	c.New(config.ServerKey, env)

	// Set Default Options
	midtrans.ServerKey = config.ServerKey
	midtrans.ClientKey = config.ClientKey
	midtrans.Environment = env

	return &MidtransGateway{
		SnapClient: s,
		CoreClient: c,
		ServerKey:  config.ServerKey,
	}
}

func (g *MidtransGateway) CreateTransaction(req *PaymentRequest) (*PaymentResponse, error) {
	var items []midtrans.ItemDetails
	for _, it := range req.Items {
		items = append(items, midtrans.ItemDetails{
			ID:    it.ID,
			Name:  it.Name,
			Price: it.Price,
			Qty:   int32(it.Qty),
		})
	}

	midtransReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.Amount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: req.Customer.Name,
			Email: req.Customer.Email,
		},
		Items: &items,
		Callbacks: &snap.Callbacks{
			Finish: req.CallbackURL,
		},
	}

	resp, err := g.SnapClient.CreateTransaction(midtransReq)
	if err != nil {
		return nil, fmt.Errorf("midtrans create transaction error: %v", err)
	}

	raw, _ := json.Marshal(resp)
	return &PaymentResponse{
		Token:       resp.Token,
		RedirectURL: resp.RedirectURL,
		RawResponse: raw,
	}, nil
}

func (g *MidtransGateway) CheckTransaction(orderID string) (*TransactionStatusResponse, error) {
	resp, err := g.CoreClient.CheckTransaction(orderID)
	if err != nil {
		return nil, fmt.Errorf("midtrans check transaction error: %v", err)
	}

	raw, _ := json.Marshal(resp)
	return &TransactionStatusResponse{
		TransactionStatus: TransactionStatus(resp.TransactionStatus),
		OrderID:           resp.OrderID,
		GrossAmount:       resp.GrossAmount,
		PaymentType:       resp.PaymentType,
		FraudStatus:       resp.FraudStatus,
		RawResponse:       raw,
	}, nil
}

func (g *MidtransGateway) CancelTransaction(orderID string) error {
	_, err := g.CoreClient.CancelTransaction(orderID)
	if err != nil {
		return fmt.Errorf("midtrans cancel transaction error: %v", err)
	}
	return nil
}

func (g *MidtransGateway) VerifyNotification(payload []byte, headers map[string]string) (bool, error) {
	// Midtrans signature verification could be implemented here
	return true, nil
}
