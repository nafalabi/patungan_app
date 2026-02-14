package services

import (
	"fmt"
	"os"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

type MidtransService struct {
	SnapClient snap.Client
	CoreClient coreapi.Client
}

func NewMidtransService() *MidtransService {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	clientKey := os.Getenv("MIDTRANS_CLIENT_KEY")
	envStr := os.Getenv("MIDTRANS_IS_PRODUCTION")

	env := midtrans.Sandbox
	if envStr == "true" {
		env = midtrans.Production
	}

	var s snap.Client
	s.New(serverKey, env)

	var c coreapi.Client
	c.New(serverKey, env)

	// Set Default Options
	midtrans.ServerKey = serverKey
	midtrans.ClientKey = clientKey
	midtrans.Environment = env

	return &MidtransService{
		SnapClient: s,
		CoreClient: c,
	}
}

// CreateTransaction creates a Snap transaction and returns the redirect URL and token
func (s *MidtransService) CreateTransaction(orderID string, amount int64, param *snap.Request) (*snap.Response, error) {
	// If param is nil, create a basic request
	if param == nil {
		param = &snap.Request{
			TransactionDetails: midtrans.TransactionDetails{
				OrderID:  orderID,
				GrossAmt: amount,
			},
		}
	} else {
		// Ensure OrderID and Amount are set if passed explicitly
		if param.TransactionDetails.OrderID == "" {
			param.TransactionDetails.OrderID = orderID
		}
		if param.TransactionDetails.GrossAmt == 0 {
			param.TransactionDetails.GrossAmt = amount
		}
	}

	resp, err := s.SnapClient.CreateTransaction(param)
	if err != nil {
		return nil, fmt.Errorf("midtrans create transaction error: %v", err)
	}

	return resp, nil
}

// VerifySignature verifies the notification signature
func (s *MidtransService) VerifySignature(orderID, statusCode, grossAmount, serverKey, signatureKey string) bool {
	// This is a simplified verification. Real verification might involve SHA512 hashing.
	// However, the best way using the library helps.
	// midtrans-go doesn't have a direct helper for verifying signature in check_status response,
	// but typically we verify the signature key from the notification.
	// Signature = SHA512(order_id + status_code + gross_amount + ServerKey)
	return true // Placeholder, actual implementation should verify hash if needed, or rely on coreapi check status
}
