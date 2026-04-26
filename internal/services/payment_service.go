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
	db *gorm.DB
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{
		db: db,
	}
}

// GetSettings fetches the singleton settings record
func (s *PaymentService) GetSettings() (*models.Settings, error) {
	var settings models.Settings
	if err := s.db.First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *PaymentService) getGateway(gateway models.PaymentGateway) (payment_gateway.Gateway, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch settings: %v", err)
	}

	switch gateway {
	case models.PaymentGatewayMidtrans:
		return payment_gateway.NewMidtransGateway(payment_gateway.MidtransConfig{
			MerchantID:   settings.MidtransMerchantID,
			ServerKey:    settings.MidtransServerKey,
			ClientKey:    settings.MidtransClientKey,
			IsProduction: settings.MidtransIsProduction,
		}), nil
	case models.PaymentGatewayMayar:
		return payment_gateway.NewMayarGateway(payment_gateway.MayarConfig{
			APIKey:       settings.MayarAPIKey,
			IsProduction: settings.MayarIsProduction,
		}), nil
	default:
		return nil, fmt.Errorf("gateway %s not supported", gateway)
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
	IsExisting  bool
}

// InitiatePayment handles the logic for starting or resuming a payment session
func (s *PaymentService) InitiatePayment(due *models.PaymentDue, forceNew bool, callbackURL string, gateway models.PaymentGateway) (*InitiatePaymentResult, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return nil, err
	}

	// Use active gateway from settings if not specified
	if gateway == "" {
		gateway = settings.ActivePaymentGateway
	}

	g, err := s.getGateway(gateway)
	if err != nil {
		return nil, err
	}

	// 1. Check for existing active session
	existingSession, err := s.CheckActiveSession(due.ID)
	if err != nil {
		return nil, err
	}

	if existingSession != nil {
		// active session exists, check status with Gateway
		statusResp, err := g.CheckTransaction(existingSession.OrderID)
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
				if forceNew {
					// Cancel at Gateway
					g.CancelTransaction(existingSession.OrderID)
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

	req := &payment_gateway.PaymentRequest{
		OrderID:  orderID,
		Amount:   int64(due.CalculatedPayAmount),
		Customer: payment_gateway.CustomerDetails{
			Name:  due.User.Name,
			Email: due.User.Email,
		},
		Items: []payment_gateway.ItemDetails{
			{
				ID:    fmt.Sprintf("plan-%d", due.PlanID),
				Name:  fmt.Sprintf("Payment for %s", due.Plan.Name),
				Price: int64(due.CalculatedPayAmount),
				Qty:   1,
			},
		},
		CallbackURL: callbackURL,
	}

	resp, err := g.CreateTransaction(req)
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
		PaymentGateway:   gateway,
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

// VerifyPaymentStatus checks the status of a payment due with the Gateway and updates local state
func (s *PaymentService) VerifyPaymentStatus(dueID uint) error {
	// 1. Find latest active session for this due
	var session models.PaymentSession
	if err := s.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // No active session to verify
		}
		return err
	}

	// 2. Call Gateway Check Transaction
	g, err := s.getGateway(session.PaymentGateway)
	if err != nil {
		return err
	}

	resp, err := g.CheckTransaction(session.OrderID)
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
