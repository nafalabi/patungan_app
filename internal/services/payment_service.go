package services

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// VerifyPaymentStatus checks the status of a payment due with Midtrans and updates local state
func (s *PaymentService) VerifyPaymentStatus(dueID uint) error {
	// 1. Find latest active session for this due
	var session models.PaymentSession
	if err := s.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // No active session to verify
		}
		return err
	}

	// 2. Call Midtrans Check Transaction
	resp, err := s.midtransClient.CheckTransaction(session.OrderID)
	if err != nil {
		return err
	}

	// 3. Process Response & Update Local State
	var due models.PaymentDue
	if err := s.db.First(&due, dueID).Error; err != nil {
		return err
	}

	s.HandleTransactionStatus(&due, session.OrderID, resp.TransactionStatus, resp.FraudStatus, resp.PaymentType, resp.GrossAmount)

	return nil
}

func (s *PaymentService) HandleTransactionStatus(due *models.PaymentDue, orderID, transactionStatus, fraudStatus, paymentType, grossAmount string) {
	switch transactionStatus {
	case "capture":
		switch fraudStatus {
		case "accept":
			s.MarkAsPaid(due, map[string]interface{}{
				"payment_type": paymentType,
				"gross_amount": grossAmount,
			})
		case "deny", "challenge":
			// do nothing
		}
	case "settlement":
		s.MarkAsPaid(due, map[string]interface{}{
			"payment_type": paymentType,
			"gross_amount": grossAmount,
		})
	case "deny", "expire", "cancel", "failure":
		var session models.PaymentSession
		if err := s.db.Where("order_id = ?", orderID).First(&session).Error; err == nil {
			session.IsActive = false
			s.db.Save(&session)
		}
	}
}

func (s *PaymentService) MarkAsPaid(due *models.PaymentDue, payload map[string]interface{}) {
	if due.PaymentStatus == models.PaymentStatusPaid {
		return
	}

	// 1. Update PaymentDue status
	due.PaymentStatus = models.PaymentStatusPaid
	s.db.Save(due)

	// 2. Create UserPayment record
	paymentType, _ := payload["payment_type"].(string)
	paymentGatewayStr, ok := payload["payment_gateway"].(string)
	var paymentGateway models.PaymentGateway
	if ok {
		paymentGateway = models.PaymentGateway(paymentGatewayStr)
	} else {
		paymentGateway = models.PaymentGatewayMidtrans // Default to midtrans for existing calls
	}

	// Helper to get float from interface safely
	var grossAmt float64
	if val, ok := payload["gross_amount"].(string); ok {
		grossAmt, _ = strconv.ParseFloat(val, 64)
	} else if val, ok := payload["gross_amount"].(float64); ok {
		grossAmt = val
	}

	userPayment := models.UserPayment{
		PlanID:         due.PlanID,
		PaymentDueID:   due.ID,
		UserID:         due.UserID,
		TotalPay:       grossAmt,
		ChannelPayment: paymentType,
		PaymentGateway: paymentGateway,
		PaymentDate:    time.Now(),
	}
	s.db.Create(&userPayment)
}
