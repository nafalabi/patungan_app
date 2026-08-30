package payment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/payment_gateway"
)

var ErrAlreadyPaid = errors.New("payment already made")

type Service struct {
	dues     DueRepo
	sessions SessionRepo
	gateway  GatewayClient
}

func NewService(dues DueRepo, sessions SessionRepo, gateway GatewayClient) *Service {
	return &Service{dues: dues, sessions: sessions, gateway: gateway}
}

// CheckActiveSession returns (nil, nil) when there is no active session.
func (s *Service) CheckActiveSession(paymentDueID uint) (*models.PaymentSession, error) {
	return s.sessions.FindLatestActive(paymentDueID)
}

type InitiatePaymentRequest struct {
	Due             *models.PaymentDue
	ForceNew        bool
	CallbackURL     string
	GatewayOverride models.PaymentGateway
}

type InitiatePaymentResult struct {
	Token       string
	RedirectURL string
	Gateway     models.PaymentGateway
	IsExisting  bool
}

// InitiatePayment starts or resumes a payment session. Returns ErrAlreadyPaid
// when the gateway reports the transaction settled.
func (s *Service) InitiatePayment(req InitiatePaymentRequest) (*InitiatePaymentResult, error) {
	existingSession, err := s.CheckActiveSession(req.Due.ID)
	if err != nil {
		return nil, err
	}

	if existingSession != nil {
		identifier := s.gateway.GetTransactionIdentifier(existingSession.PaymentGateway, existingSession.OrderID, existingSession.ResponseMetadata)

		statusResp, err := s.gateway.CheckTransaction(identifier, existingSession.PaymentGateway)
		if err == nil {
			if statusResp.TransactionStatus == payment_gateway.StatusSettlement || statusResp.TransactionStatus == payment_gateway.StatusCapture {
				return nil, ErrAlreadyPaid
			}

			if statusResp.TransactionStatus == payment_gateway.StatusDeny ||
				statusResp.TransactionStatus == payment_gateway.StatusExpire ||
				statusResp.TransactionStatus == payment_gateway.StatusCancel ||
				statusResp.TransactionStatus == payment_gateway.StatusFailure {
				existingSession.IsActive = false
				s.sessions.Save(existingSession)
			} else if req.ForceNew {
				s.gateway.CancelTransaction(existingSession.OrderID, existingSession.PaymentGateway)
				existingSession.IsActive = false
				s.sessions.Save(existingSession)
			} else {
				var resp payment_gateway.PaymentResponse
				if err := json.Unmarshal(existingSession.ResponseMetadata, &resp); err == nil {
					return &InitiatePaymentResult{
						Token:       resp.Token,
						RedirectURL: resp.RedirectURL,
						Gateway:     existingSession.PaymentGateway,
						IsExisting:  true,
					}, nil
				}
				existingSession.IsActive = false
				s.sessions.Save(existingSession)
			}
		} else {
			existingSession.IsActive = false
			s.sessions.Save(existingSession)
		}
	}

	// Create new transaction
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

	resp, selectedGateway, err := s.gateway.CreateTransaction(gwReq, req.GatewayOverride)
	if err != nil {
		return nil, err
	}

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
	if err := s.sessions.Create(&session); err != nil {
		return nil, err
	}

	return &InitiatePaymentResult{
		Token:       resp.Token,
		RedirectURL: resp.RedirectURL,
		Gateway:     selectedGateway,
		IsExisting:  false,
	}, nil
}

// VerifyPaymentStatus checks the gateway for the latest active session and
// updates local state. Returns nil when there is no active session.
func (s *Service) VerifyPaymentStatus(dueID uint) error {
	session, err := s.sessions.FindLatestActive(dueID)
	if err != nil || session == nil {
		return err
	}

	identifier := s.gateway.GetTransactionIdentifier(session.PaymentGateway, session.OrderID, session.ResponseMetadata)

	resp, err := s.gateway.CheckTransaction(identifier, session.PaymentGateway)
	if err != nil {
		return err
	}

	due, err := s.dues.FindByID(dueID)
	if err != nil || due == nil {
		if err == nil {
			err = fmt.Errorf("payment due %d not found", dueID)
		}
		return err
	}

	s.HandleTransactionStatus(due, session.OrderID, string(resp.TransactionStatus), resp.FraudStatus, resp.PaymentType, resp.GrossAmount)
	return nil
}

func (s *Service) HandleTransactionStatus(due *models.PaymentDue, orderID, transactionStatus, fraudStatus, paymentType, grossAmount string) {
	status := strings.ToLower(transactionStatus)
	switch status {
	case "capture", "success", "paid", "settlement":
		if fraudStatus == "" || fraudStatus == "accept" {
			s.MarkAsPaid(due, map[string]interface{}{
				"payment_type": paymentType,
				"gross_amount": grossAmount,
			})
		}
	case "deny", "expire", "cancel", "failure", "failed":
		if session, err := s.sessions.FindByOrderID(orderID); err == nil && session != nil {
			session.IsActive = false
			s.sessions.Save(session)
		}
	}
}

func (s *Service) MarkAsPaid(due *models.PaymentDue, payload map[string]interface{}) {
	if due.PaymentStatus == models.PaymentStatusPaid {
		return
	}

	due.PaymentStatus = models.PaymentStatusPaid
	if err := s.dues.Save(due); err != nil {
		return
	}

	paymentType, _ := payload["payment_type"].(string)
	paymentGatewayStr, ok := payload["payment_gateway"].(string)
	var paymentGateway models.PaymentGateway
	if ok {
		paymentGateway = models.PaymentGateway(paymentGatewayStr)
	} else if session, err := s.sessions.FindLatestActive(due.ID); err == nil && session != nil {
		paymentGateway = session.PaymentGateway
	} else {
		paymentGateway = models.PaymentGatewayMidtrans
	}

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
	s.dues.CreatePaymentRecord(&userPayment)
}
