package payment_test

import (
	"encoding/json"
	"errors"
	"testing"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/payment"
	"patungan_app_echo/internal/services/payment_gateway"
)

type fakeDueRepo struct {
	due             models.PaymentDue
	paymentsCreated []models.UserPayment
}

func (f *fakeDueRepo) FindByID(id uint) (*models.PaymentDue, error) { d := f.due; return &d, nil }
func (f *fakeDueRepo) Save(due *models.PaymentDue) error            { f.due = *due; return nil }
func (f *fakeDueRepo) Create(due *models.PaymentDue) error          { return nil }
func (f *fakeDueRepo) CreatePaymentRecord(p *models.UserPayment) error {
	f.paymentsCreated = append(f.paymentsCreated, *p)
	return nil
}

type fakeSessionRepo struct {
	active  *models.PaymentSession
	byOrder map[string]*models.PaymentSession
	created []*models.PaymentSession
}

func (f *fakeSessionRepo) FindLatestActive(dueID uint) (*models.PaymentSession, error) {
	return f.active, nil
}
func (f *fakeSessionRepo) FindByOrderID(orderID string) (*models.PaymentSession, error) {
	if s, ok := f.byOrder[orderID]; ok {
		return s, nil
	}
	return nil, nil
}
func (f *fakeSessionRepo) Save(s *models.PaymentSession) error { return nil }
func (f *fakeSessionRepo) Create(s *models.PaymentSession) error {
	f.created = append(f.created, s)
	return nil
}

type fakeGateway struct {
	checkErr   error
	checkResp  *payment_gateway.TransactionStatusResponse
	createResp *payment_gateway.PaymentResponse
	createGW   models.PaymentGateway
	cancelled  []string
}

func (f *fakeGateway) CreateTransaction(req *payment_gateway.PaymentRequest, gw models.PaymentGateway) (*payment_gateway.PaymentResponse, models.PaymentGateway, error) {
	return f.createResp, f.createGW, nil
}
func (f *fakeGateway) CheckTransaction(orderID string, gw models.PaymentGateway) (*payment_gateway.TransactionStatusResponse, error) {
	return f.checkResp, f.checkErr
}
func (f *fakeGateway) CancelTransaction(orderID string, gw models.PaymentGateway) error {
	f.cancelled = append(f.cancelled, orderID)
	return nil
}
func (f *fakeGateway) GetTransactionIdentifier(gw models.PaymentGateway, orderID string, meta []byte) string {
	return orderID
}

func newTestService(due models.PaymentDue, sess *models.PaymentSession, gw *fakeGateway) (*payment.Service, *fakeSessionRepo) {
	sessions := &fakeSessionRepo{active: sess}
	svc := payment.NewService(&fakeDueRepo{due: due}, sessions, gw)
	return svc, sessions
}

func activeSession(dueID uint) *models.PaymentSession {
	return &models.PaymentSession{
		PaymentDueID: dueID, PaymentGateway: models.PaymentGatewayMidtrans,
		OrderID: "payment-due-1-123", IsActive: true,
	}
}

func TestInitiatePayment_ReusesExistingPendingSession(t *testing.T) {
	due := models.PaymentDue{ID: 1, PlanID: 2, UserID: 3, CalculatedPayAmount: 1000}
	sess := activeSession(1)
	sess.ResponseMetadata = json.RawMessage(`{"token":"tok-1","redirect_url":"https://pay"}`)
	gw := &fakeGateway{checkResp: &payment_gateway.TransactionStatusResponse{TransactionStatus: payment_gateway.StatusPending}}

	svc, sessions := newTestService(due, sess, gw)
	res, err := svc.InitiatePayment(payment.InitiatePaymentRequest{Due: &due})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsExisting || res.Token != "tok-1" {
		t.Fatalf("want reuse, got %+v", res)
	}
	if len(sessions.created) != 0 {
		t.Fatalf("should not create a session, got %d", len(sessions.created))
	}
}

func TestInitiatePayment_RejectsWhenAlreadyPaid(t *testing.T) {
	due := models.PaymentDue{ID: 1}
	sess := activeSession(1)
	gw := &fakeGateway{checkResp: &payment_gateway.TransactionStatusResponse{TransactionStatus: payment_gateway.StatusSettlement}}

	svc, _ := newTestService(due, sess, gw)
	_, err := svc.InitiatePayment(payment.InitiatePaymentRequest{Due: &due})

	if !errors.Is(err, payment.ErrAlreadyPaid) {
		t.Fatalf("want ErrAlreadyPaid, got %v", err)
	}
}

func TestInitiatePayment_ForceNewCancelsPendingSession(t *testing.T) {
	due := models.PaymentDue{ID: 1, PlanID: 2, UserID: 3, CalculatedPayAmount: 1000}
	sess := activeSession(1)
	gw := &fakeGateway{
		checkResp:  &payment_gateway.TransactionStatusResponse{TransactionStatus: payment_gateway.StatusPending},
		createResp: &payment_gateway.PaymentResponse{Token: "tok-2", RedirectURL: "https://pay2"},
		createGW:   models.PaymentGatewayMidtrans,
	}

	svc, sessions := newTestService(due, sess, gw)
	res, err := svc.InitiatePayment(payment.InitiatePaymentRequest{Due: &due, ForceNew: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsExisting || res.Token != "tok-2" {
		t.Fatalf("want new transaction, got %+v", res)
	}
	if len(gw.cancelled) != 1 || gw.cancelled[0] != "payment-due-1-123" {
		t.Fatalf("want old session cancelled, got %v", gw.cancelled)
	}
	if len(sessions.created) != 1 {
		t.Fatalf("want 1 new session, got %d", len(sessions.created))
	}
}

func TestMarkAsPaid_CreatesUserPaymentOnce(t *testing.T) {
	due := models.PaymentDue{ID: 1, PlanID: 2, UserID: 3, CalculatedPayAmount: 5000, PaymentStatus: models.PaymentStatusPending}
	repo := &fakeDueRepo{due: due}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	svc.MarkAsPaid(&due, map[string]interface{}{"payment_type": "manual", "gross_amount": "5000", "payment_gateway": "manual"})

	if repo.due.PaymentStatus != models.PaymentStatusPaid {
		t.Fatalf("want paid, got %s", repo.due.PaymentStatus)
	}
	if len(repo.paymentsCreated) != 1 {
		t.Fatalf("want 1 payment record, got %d", len(repo.paymentsCreated))
	}
	if repo.paymentsCreated[0].ChannelPayment != "manual" {
		t.Fatalf("want ChannelPayment manual, got %s", repo.paymentsCreated[0].ChannelPayment)
	}
}
