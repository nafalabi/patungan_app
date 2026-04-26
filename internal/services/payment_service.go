package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/payment_gateway"
)

type PaymentService struct {
	db             *gorm.DB
	gatewayManager *payment_gateway.GatewayManager
}

func NewPaymentService(db *gorm.DB, gatewayManager *payment_gateway.GatewayManager) *PaymentService {
	return &PaymentService{
		db:             db,
		gatewayManager: gatewayManager,
	}
}

// CheckActiveSession checks if there is an active session for the given due ID
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

type InitiatePaymentResult struct {
	Token       string
	RedirectURL string
	Gateway     models.PaymentGateway
	IsExisting  bool
}

type InitiatePaymentRequest struct {
	Due             *models.PaymentDue
	ForceNew        bool
	CallbackURL     string
	GatewayOverride models.PaymentGateway
}

// InitiatePayment handles the logic for starting or resuming a payment session
func (s *PaymentService) InitiatePayment(req InitiatePaymentRequest) (*InitiatePaymentResult, error) {
	// 1. Check for existing active session
	existingSession, err := s.CheckActiveSession(req.Due.ID)
	if err != nil {
		return nil, err
	}

	if existingSession != nil {
		// Get correct identifier for the gateway
		identifier := s.gatewayManager.GetTransactionIdentifier(existingSession.PaymentGateway, existingSession.OrderID, existingSession.ResponseMetadata)

		// active session exists, check status via Manager
		statusResp, err := s.gatewayManager.CheckTransaction(identifier, existingSession.PaymentGateway)
		if err == nil {
			// Case 1: Payment already successful
			if statusResp.TransactionStatus == payment_gateway.StatusSettlement || statusResp.TransactionStatus == payment_gateway.StatusCapture {
				return nil, fmt.Errorf("payment already made")
			}

			// Case 2: Payment failed/expired/canceled
			if statusResp.TransactionStatus == payment_gateway.StatusDeny ||
				statusResp.TransactionStatus == payment_gateway.StatusExpire ||
				statusResp.TransactionStatus == payment_gateway.StatusCancel ||
				statusResp.TransactionStatus == payment_gateway.StatusFailure {
				// Deactivate local session
				existingSession.IsActive = false
				s.db.Save(existingSession)
				// Proceed to create new
			} else {
				// Case 3: Payment is Pending
				if req.ForceNew {
					// Cancel via Manager
					s.gatewayManager.CancelTransaction(existingSession.OrderID, existingSession.PaymentGateway)
					existingSession.IsActive = false
					s.db.Save(existingSession)
					// Proceed to create new
				} else {
					// Reuse existing
					var resp payment_gateway.PaymentResponse
					if err := json.Unmarshal(existingSession.ResponseMetadata, &resp); err == nil {
						return &InitiatePaymentResult{
							Token:       resp.Token,
							RedirectURL: resp.RedirectURL,
							Gateway:     existingSession.PaymentGateway,
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
	orderID := fmt.Sprintf("payment-due-%d-%d", req.Due.ID, time.Now().Unix())

	gwReq := &payment_gateway.PaymentRequest{
		OrderID: orderID,
		Amount:  int64(req.Due.CalculatedPayAmount),
		Customer: payment_gateway.CustomerDetails{
			Name:  req.Due.User.Name,
			Email: req.Due.User.Email,
		},
		Items: []payment_gateway.ItemDetails{
			{
				ID:    fmt.Sprintf("plan-%d", req.Due.PlanID),
				Name:  fmt.Sprintf("Payment for %s", req.Due.Plan.Name),
				Price: int64(req.Due.CalculatedPayAmount),
				Qty:   1,
			},
		},
		CallbackURL: req.CallbackURL,
	}

	resp, selectedGateway, err := s.gatewayManager.CreateTransaction(gwReq, req.GatewayOverride)
	if err != nil {
		return nil, err
	}

	// 3. Create Session Record
	reqBytes, _ := json.Marshal(gwReq)
	respBytes, _ := json.Marshal(resp)

	session := models.PaymentSession{
		PlanID:           req.Due.PlanID,
		PaymentDueID:     req.Due.ID,
		UserID:           req.Due.UserID,
		PaymentGateway:   selectedGateway,
		OrderID:          orderID,
		IsActive:         true,
		RequestMetadata:  reqBytes,
		ResponseMetadata: respBytes,
	}
	s.db.Create(&session)

	return &InitiatePaymentResult{
		Token:       resp.Token,
		RedirectURL: resp.RedirectURL,
		Gateway:     selectedGateway,
		IsExisting:  false,
	}, nil
}

// VerifyPaymentStatus checks the status of a payment due via the Manager and updates local state
func (s *PaymentService) VerifyPaymentStatus(dueID uint) error {
	// 1. Find latest active session for this due
	var session models.PaymentSession
	if err := s.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // No active session to verify
		}
		return err
	}

	// 2. Call Manager Check Transaction
	identifier := s.gatewayManager.GetTransactionIdentifier(session.PaymentGateway, session.OrderID, session.ResponseMetadata)

	resp, err := s.gatewayManager.CheckTransaction(identifier, session.PaymentGateway)
	if err != nil {
		return err
	}

	// 3. Process Response & Update Local State
	var due models.PaymentDue
	if err := s.db.First(&due, dueID).Error; err != nil {
		return err
	}

	s.HandleTransactionStatus(&due, session.OrderID, string(resp.TransactionStatus), resp.FraudStatus, resp.PaymentType, resp.GrossAmount)

	return nil
}

func (s *PaymentService) HandleTransactionStatus(due *models.PaymentDue, orderID, transactionStatus, fraudStatus, paymentType, grossAmount string) {
	switch transactionStatus {
	case "capture", "success": // "success" is for Mayar, "capture" for Midtrans
		if fraudStatus == "" || fraudStatus == "accept" {
			s.MarkAsPaid(due, map[string]interface{}{
				"payment_type": paymentType,
				"gross_amount": grossAmount,
			})
		}
	case "settlement":
		s.MarkAsPaid(due, map[string]interface{}{
			"payment_type": paymentType,
			"gross_amount": grossAmount,
		})
	case "deny", "expire", "cancel", "failure", "failed":
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
		// Try to find the session to get the gateway
		var session models.PaymentSession
		if err := s.db.Where("payment_due_id = ? AND is_active = ?", due.ID, true).Order("created_at desc").First(&session).Error; err == nil {
			paymentGateway = session.PaymentGateway
		} else {
			paymentGateway = models.PaymentGatewayMidtrans // Default fallback
		}
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
