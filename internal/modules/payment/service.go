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

// ErrNotFound is returned by MarkDueComplete when the due does not exist.
var ErrNotFound = errors.New("payment due not found")

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

// ListDuesFlat returns a paginated flat list of dues (canceled excluded).
func (s *Service) ListDuesFlat(p ListFlatParams) (FlatDuesResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}

	dues, totalCount, err := s.dues.ListFlat(p)
	if err != nil {
		return FlatDuesResult{}, err
	}

	totalPages := int((totalCount + int64(p.PageSize) - 1) / int64(p.PageSize))
	return FlatDuesResult{
		Dues:        mapDues(dues),
		CurrentPage: p.Page,
		TotalPages:  totalPages,
		TotalCount:  int(totalCount),
		PageSize:    p.PageSize,
	}, nil
}

// ListDuesByPlans groups dues by plan, showing only the latest period per plan.
// Returns the groups and the next offset (0 when exhausted).
func (s *Service) ListDuesByPlans(limit, offset int) ([]PlanDuesGroup, int, error) {
	plans, err := s.dues.ListPlansWithLatestDues(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]PlanDuesGroup, 0, len(plans))
	for _, plan := range plans {
		period, dues, err := s.dues.ListByPlanLatestPeriod(plan.ID)
		if err != nil {
			return nil, 0, err
		}
		if period != nil {
			groups = append(groups, PlanDuesGroup{
				Plan:    mapPlan(plan),
				Periods: []PeriodDues{{Period: mapPeriod(*period), Dues: mapDues(dues)}},
			})
			continue
		}

		// Handle dues without period
		orphans, err := s.dues.ListOrphanDuesByPlan(plan.ID)
		if err != nil {
			return nil, 0, err
		}
		if len(orphans) > 0 {
			groups = append(groups, PlanDuesGroup{
				Plan:    mapPlan(plan),
				Periods: []PeriodDues{{Period: PeriodOption{ID: 0}, Dues: mapDues(orphans)}},
			})
		}
	}

	nextOffset := offset + limit
	if len(plans) < limit {
		nextOffset = 0 // No more
	}
	return groups, nextOffset, nil
}

// ListDuesByPeriods groups dues by billing period, then plans.
func (s *Service) ListDuesByPeriods(limit, offset int) ([]PeriodPlansGroup, int, error) {
	periods, err := s.dues.ListPeriods(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]PeriodPlansGroup, 0, len(periods))
	for _, period := range periods {
		// Get top 3 plans for this period
		plans, err := s.dues.ListTopPlansInPeriod(period.ID, 3)
		if err != nil {
			return nil, 0, err
		}

		group := PeriodPlansGroup{Period: mapPeriod(period)}
		for _, plan := range plans {
			dues, err := s.dues.ListByPeriodAndPlan(period.ID, plan.ID)
			if err != nil {
				return nil, 0, err
			}
			group.Plans = append(group.Plans, PlanDuesInPeriod{Plan: mapPlan(plan), Dues: mapDues(dues)})
		}
		groups = append(groups, group)
	}

	nextOffset := offset + limit
	if len(periods) < limit {
		nextOffset = 0
	}
	return groups, nextOffset, nil
}

// ListDuesByUsers groups dues by user with their latest dues.
func (s *Service) ListDuesByUsers(limit, offset int) ([]UserDuesGroup, int, error) {
	users, err := s.dues.ListUsersWithLatestDues(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]UserDuesGroup, 0, len(users))
	for _, user := range users {
		// Show 3 latest dues per user
		dues, err := s.dues.ListLatestByUser(user.ID, 3)
		if err != nil {
			return nil, 0, err
		}
		groups = append(groups, UserDuesGroup{User: mapUser(user), Dues: mapDues(dues)})
	}

	nextOffset := offset + limit
	if len(users) < limit {
		nextOffset = 0
	}
	return groups, nextOffset, nil
}

// FilterOptions returns all plans and users for the filter dropdowns.
func (s *Service) FilterOptions() (FilterOptions, error) {
	plans, err := s.dues.ListPlans()
	if err != nil {
		return FilterOptions{}, err
	}
	users, err := s.dues.ListUsers()
	if err != nil {
		return FilterOptions{}, err
	}

	opts := FilterOptions{
		Plans: make([]PlanOption, 0, len(plans)),
		Users: make([]UserOption, 0, len(users)),
	}
	for _, p := range plans {
		opts.Plans = append(opts.Plans, mapPlan(p))
	}
	for _, u := range users {
		opts.Users = append(opts.Users, mapUser(u))
	}
	return opts, nil
}

// GetDueByUUID returns nil when no due matches the UUID.
func (s *Service) GetDueByUUID(uuid string) (*DueItem, error) {
	due, err := s.dues.FindByUUID(uuid)
	if err != nil || due == nil {
		return nil, err
	}
	item := mapDue(*due)
	return &item, nil
}

// GetDueModelByUUID returns the raw due (with Plan and User preloaded) for
// flows that need model-level access, e.g. initiating a payment.
func (s *Service) GetDueModelByUUID(uuid string) (*models.PaymentDue, error) {
	return s.dues.FindByUUID(uuid)
}

// GetDueModelByID returns the raw due (with Plan and User preloaded) for
// flows that need model-level access, e.g. gateway callbacks.
func (s *Service) GetDueModelByID(id uint) (*models.PaymentDue, error) {
	return s.dues.FindByID(id)
}

// GetDueForRender returns nil when the due is missing.
func (s *Service) GetDueForRender(id uint) (*DueItem, error) {
	due, err := s.dues.FindByID(id)
	if err != nil || due == nil {
		return nil, err
	}
	item := mapDue(*due)
	return &item, nil
}

// MarkDueComplete marks a due paid manually; authorization is the caller's
// responsibility. It is a no-op when the due is already paid.
func (s *Service) MarkDueComplete(id uint) error {
	due, err := s.dues.FindByID(id)
	if err != nil {
		return err
	}
	if due == nil {
		return ErrNotFound
	}
	if due.PaymentStatus == models.PaymentStatusPaid {
		return nil
	}

	s.MarkAsPaid(due, map[string]interface{}{
		"payment_type":    "manual",
		"gross_amount":    due.CalculatedPayAmount,
		"payment_gateway": string(models.PaymentGatewayManual), // Pass as string, helper converts back
	})
	return nil
}

// MemberDuesSummary returns the member's dues plus pending/paid totals.
func (s *Service) MemberDuesSummary(userID uint, statusFilter string) ([]DueItem, float64, float64, error) {
	var statuses []string
	switch statusFilter {
	case "pending":
		statuses = []string{models.PaymentStatusPending, models.PaymentStatusOverdue}
	case "paid":
		statuses = []string{models.PaymentStatusPaid}
	}

	dues, err := s.dues.ListForUser(userID, statuses)
	if err != nil {
		return nil, 0, 0, err
	}

	totalPending, err := s.dues.SumForUserByStatus(userID, []string{models.PaymentStatusPending, models.PaymentStatusOverdue})
	if err != nil {
		return nil, 0, 0, err
	}
	totalPaid, err := s.dues.SumForUserByStatus(userID, []string{models.PaymentStatusPaid})
	if err != nil {
		return nil, 0, 0, err
	}

	return mapDues(dues), totalPending, totalPaid, nil
}

// CreateCallbackHistory persists a gateway callback payload for auditing.
func (s *Service) CreateCallbackHistory(h *models.PaymentCallbackHistory) error {
	return s.dues.CreateCallbackHistory(h)
}

// FindLatestByGatewayMetadata returns (nil, nil) when no session matches.
func (s *Service) FindLatestByGatewayMetadata(gateway models.PaymentGateway, metadataSubstring string) (*models.PaymentSession, error) {
	return s.sessions.FindLatestByGatewayMetadata(gateway, metadataSubstring)
}
