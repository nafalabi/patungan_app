package services

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

type PaymentService struct {
	db             *gorm.DB
	midtransClient *MidtransService
}

func NewPaymentService(db *gorm.DB, midtransClient *MidtransService) *PaymentService {
	return &PaymentService{
		db:             db,
		midtransClient: midtransClient,
	}
}

// CheckActiveSession checks if there is an active session for the given due ID
// Returns the session if active and valid, otherwise nil or error
func (s *PaymentService) CheckActiveSession(paymentDueID uint) (*models.PaymentSession, error) {
	var existingSession models.PaymentSession
	err := s.db.Where("payment_due_id = ? AND is_active = ?", paymentDueID, true).Order("created_at desc").First(&existingSession).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active session
		}
		return nil, err
	}
	return &existingSession, nil
}

// InitiatePaymentResult holds the result of an initiation attempt
type InitiatePaymentResult struct {
	Token       string
	RedirectURL string
	IsExisting  bool
}

// InitiatePayment handles the logic for starting or resuming a payment session
func (s *PaymentService) InitiatePayment(due *models.PaymentDue, forceNew bool, callbackURL string) (*InitiatePaymentResult, error) {
	// 1. Check for existing active session
	existingSession, err := s.CheckActiveSession(due.ID)
	if err != nil {
		return nil, err
	}

	if existingSession != nil {
		// active session exists, check status with Midtrans
		statusResp, err := s.midtransClient.CheckTransaction(existingSession.OrderID)
		if err == nil {
			// Case 1: Payment already successful
			if statusResp.TransactionStatus == "settlement" || statusResp.TransactionStatus == "capture" {
				return nil, fmt.Errorf("payment already made")
			}

			// Case 2: Payment failed/expired/canceled
			if statusResp.TransactionStatus == "deny" || statusResp.TransactionStatus == "expire" || statusResp.TransactionStatus == "cancel" || statusResp.TransactionStatus == "failure" {
				// Deactivate local session
				existingSession.IsActive = false
				s.db.Save(existingSession)
				// Proceed to create new
			} else {
				// Case 3: Payment is Pending
				if forceNew {
					// Cancel at Midtrans
					s.midtransClient.CancelTransaction(existingSession.OrderID)
					existingSession.IsActive = false
					s.db.Save(existingSession)
					// Proceed to create new
				} else {
					// Reuse existing
					var midtransResp snap.Response
					if err := json.Unmarshal(existingSession.ResponseMetadata, &midtransResp); err == nil {
						return &InitiatePaymentResult{
							Token:       midtransResp.Token,
							RedirectURL: midtransResp.RedirectURL,
							IsExisting:  true,
						}, nil
					}
					// If unmarshal fails, treat as broken
					existingSession.IsActive = false
					s.db.Save(existingSession)
				}
			}
		} else {
			// Check failed, assume session is invalid/broken locally
			existingSession.IsActive = false
			s.db.Save(existingSession)
		}
	}

	// 2. Create New Transaction
	orderID := fmt.Sprintf("payment-due-%d-%d", due.ID, time.Now().Unix())

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(due.CalculatedPayAmount),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: due.User.Name,
			Email: due.User.Email,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    fmt.Sprintf("plan-%d", due.PlanID),
				Name:  fmt.Sprintf("Payment for %s", due.Plan.Name),
				Price: int64(due.CalculatedPayAmount),
				Qty:   1,
			},
		},
		Callbacks: &snap.Callbacks{
			Finish: callbackURL,
		},
	}

	resp, err := s.midtransClient.CreateTransaction(orderID, int64(due.CalculatedPayAmount), req)
	if err != nil {
		return nil, err
	}

	// 3. Create Session Record
	reqBytes, _ := json.Marshal(req)
	respBytes, _ := json.Marshal(resp)

	session := models.PaymentSession{
		PlanID:           due.PlanID,
		PaymentDueID:     due.ID,
		UserID:           due.UserID,
		PaymentGateway:   models.PaymentGatewayMidtrans,
		OrderID:          orderID,
		IsActive:         true,
		RequestMetadata:  reqBytes,
		ResponseMetadata: respBytes,
	}
	s.db.Create(&session)

	return &InitiatePaymentResult{
		Token:       resp.Token,
		RedirectURL: resp.RedirectURL,
		IsExisting:  false,
	}, nil
}
