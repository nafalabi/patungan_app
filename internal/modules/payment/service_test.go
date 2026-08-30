package payment_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/payment"
	"patungan_app_echo/internal/services/payment_gateway"
)

type fakeDueRepo struct {
	due             models.PaymentDue
	paymentsCreated []models.UserPayment

	byUUID           map[string]*models.PaymentDue
	histories        []models.PaymentCallbackHistory
	listFlatParams   []payment.ListFlatParams
	listFlat         []models.PaymentDue
	listFlatTotal    int64
	plansWithDues    []models.Plan
	periods          []models.PaymentBillingPeriod
	usersWithDues    []models.User
	allPlans         []models.Plan
	allUsers         []models.User
	latestPeriod     map[uint]*models.PaymentBillingPeriod
	duesByPlanPeriod map[uint][]models.PaymentDue
	orphansByPlan    map[uint][]models.PaymentDue
	topPlansByPeriod map[uint][]models.Plan
	duesByPeriodPlan map[[2]uint][]models.PaymentDue
	latestByUser     []models.PaymentDue
	forUser          []models.PaymentDue
	listForUserArgs  [][]string
	sums             map[string]float64
	sumArgs          [][]string
	countByStatus    map[string]int64
	sumByStatus      map[string]float64
	countSumArgs     [][]string
	upcoming         []models.PaymentDue
	upcomingArgs     [][]string
	paidSince        float64
	paidSinceArgs    []time.Time
}

func (f *fakeDueRepo) FindByID(id uint) (*models.PaymentDue, error) {
	if f.due.ID == 0 {
		return nil, nil
	}
	d := f.due
	return &d, nil
}

func (f *fakeDueRepo) FindByUUID(uuid string) (*models.PaymentDue, error) {
	if d, ok := f.byUUID[uuid]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeDueRepo) Save(due *models.PaymentDue) error { f.due = *due; return nil }
func (f *fakeDueRepo) Create(due *models.PaymentDue) error {
	f.due = *due
	return nil
}
func (f *fakeDueRepo) CreatePaymentRecord(p *models.UserPayment) error {
	f.paymentsCreated = append(f.paymentsCreated, *p)
	return nil
}
func (f *fakeDueRepo) CreateCallbackHistory(h *models.PaymentCallbackHistory) error {
	f.histories = append(f.histories, *h)
	return nil
}

func (f *fakeDueRepo) ListFlat(params payment.ListFlatParams) ([]models.PaymentDue, int64, error) {
	f.listFlatParams = append(f.listFlatParams, params)
	return f.listFlat, f.listFlatTotal, nil
}

func (f *fakeDueRepo) ListPlansWithLatestDues(limit, offset int) ([]models.Plan, error) {
	return f.plansWithDues, nil
}

func (f *fakeDueRepo) ListPeriods(limit, offset int) ([]models.PaymentBillingPeriod, error) {
	return f.periods, nil
}

func (f *fakeDueRepo) ListUsersWithLatestDues(limit, offset int) ([]models.User, error) {
	return f.usersWithDues, nil
}

func (f *fakeDueRepo) ListByPlanLatestPeriod(planID uint) (*models.PaymentBillingPeriod, []models.PaymentDue, error) {
	if p, ok := f.latestPeriod[planID]; ok && p != nil {
		cp := *p
		return &cp, f.duesByPlanPeriod[planID], nil
	}
	return nil, nil, nil
}

func (f *fakeDueRepo) ListOrphanDuesByPlan(planID uint) ([]models.PaymentDue, error) {
	return f.orphansByPlan[planID], nil
}

func (f *fakeDueRepo) ListTopPlansInPeriod(periodID uint, limit int) ([]models.Plan, error) {
	return f.topPlansByPeriod[periodID], nil
}

func (f *fakeDueRepo) ListByPeriodAndPlan(periodID, planID uint) ([]models.PaymentDue, error) {
	return f.duesByPeriodPlan[[2]uint{periodID, planID}], nil
}

func (f *fakeDueRepo) ListLatestByUser(userID uint, limit int) ([]models.PaymentDue, error) {
	return f.latestByUser, nil
}

func (f *fakeDueRepo) ListPlans() ([]models.Plan, error) { return f.allPlans, nil }

func (f *fakeDueRepo) ListUsers() ([]models.User, error) { return f.allUsers, nil }

func (f *fakeDueRepo) ListForUser(userID uint, statuses []string) ([]models.PaymentDue, error) {
	f.listForUserArgs = append(f.listForUserArgs, append([]string(nil), statuses...))
	return f.forUser, nil
}

func (f *fakeDueRepo) SumForUserByStatus(userID uint, statuses []string) (float64, error) {
	f.sumArgs = append(f.sumArgs, append([]string(nil), statuses...))
	if len(statuses) > 0 {
		if v, ok := f.sums[statuses[0]]; ok {
			return v, nil
		}
	}
	return 0, nil
}

func (f *fakeDueRepo) CountDuesByStatus(status string, userID *uint) (int64, error) {
	arg := []string{status}
	if userID != nil {
		arg = append(arg, fmt.Sprintf("%d", *userID))
	}
	f.countSumArgs = append(f.countSumArgs, arg)
	return f.countByStatus[status], nil
}

func (f *fakeDueRepo) SumDuesByStatus(status string, userID *uint) (float64, error) {
	arg := []string{status}
	if userID != nil {
		arg = append(arg, fmt.Sprintf("%d", *userID))
	}
	f.countSumArgs = append(f.countSumArgs, arg)
	return f.sumByStatus[status], nil
}

func (f *fakeDueRepo) ListUpcoming(status string, userID *uint, limit int) ([]models.PaymentDue, error) {
	arg := []string{status, fmt.Sprintf("limit=%d", limit)}
	if userID != nil {
		arg = append(arg, fmt.Sprintf("%d", *userID))
	}
	f.upcomingArgs = append(f.upcomingArgs, arg)
	return f.upcoming, nil
}

func (f *fakeDueRepo) CountPaidSince(userID uint, since time.Time) (float64, error) {
	f.paidSinceArgs = append(f.paidSinceArgs, since)
	return f.paidSince, nil
}

type fakeSessionRepo struct {
	active  *models.PaymentSession
	byOrder map[string]*models.PaymentSession
	byMeta  *models.PaymentSession
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
func (f *fakeSessionRepo) FindLatestByGatewayMetadata(gateway models.PaymentGateway, metadataSubstring string) (*models.PaymentSession, error) {
	return f.byMeta, nil
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

func TestListDuesFlat_MapsAndPaginates(t *testing.T) {
	repo := &fakeDueRepo{
		listFlat: []models.PaymentDue{
			{ID: 1, UUID: "u1", CalculatedPayAmount: 1000, PaymentStatus: models.PaymentStatusPending,
				Plan: models.Plan{ID: 2, Name: "Netflix"}, User: models.User{ID: 3, Name: "Budi", Email: "budi@example.com"}},
		},
		listFlatTotal: 25,
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	res, err := svc.ListDuesFlat(payment.ListFlatParams{FilterPlan: 2, SortBy: "due_date", SortOrder: "desc", Page: 2, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalPages != 2 || res.TotalCount != 25 || res.CurrentPage != 2 || res.PageSize != 20 {
		t.Fatalf("unexpected pagination: %+v", res)
	}
	if len(res.Dues) != 1 || res.Dues[0].Amount != 1000 || res.Dues[0].Status != models.PaymentStatusPending ||
		res.Dues[0].PlanName != "Netflix" || res.Dues[0].UserName != "Budi" || res.Dues[0].UserEmail != "budi@example.com" {
		t.Fatalf("unexpected mapping: %+v", res.Dues)
	}
	if len(repo.listFlatParams) != 1 || repo.listFlatParams[0].Page != 2 ||
		repo.listFlatParams[0].PageSize != 20 || repo.listFlatParams[0].FilterPlan != 2 {
		t.Fatalf("unexpected params: %+v", repo.listFlatParams)
	}
}

func TestListDuesByPlans_LatestPeriodAndOrphans(t *testing.T) {
	plan1 := models.Plan{ID: 1, Name: "Netflix", TotalPrice: 100000}
	plan2 := models.Plan{ID: 2, Name: "Spotify", TotalPrice: 50000}
	period := models.PaymentBillingPeriod{ID: 10, Name: "May 2026"}
	repo := &fakeDueRepo{
		plansWithDues: []models.Plan{plan1, plan2},
		latestPeriod:  map[uint]*models.PaymentBillingPeriod{1: &period},
		duesByPlanPeriod: map[uint][]models.PaymentDue{
			1: {{ID: 100, PlanID: 1, UserID: 3, CalculatedPayAmount: 25000, PaymentStatus: models.PaymentStatusPending,
				Plan: plan1, User: models.User{ID: 3, Name: "Budi", Email: "budi@example.com"}}},
		},
		orphansByPlan: map[uint][]models.PaymentDue{
			2: {{ID: 200, PlanID: 2, UserID: 3, CalculatedPayAmount: 10000, PaymentStatus: models.PaymentStatusPending, Plan: plan2}},
		},
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	groups, nextOffset, err := svc.ListDuesByPlans(5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextOffset != 0 {
		t.Fatalf("want nextOffset 0 when fewer plans than limit, got %d", nextOffset)
	}
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	if groups[0].Plan.Name != "Netflix" || groups[0].Plan.TotalPrice != 100000 {
		t.Fatalf("unexpected plan: %+v", groups[0].Plan)
	}
	if groups[0].Periods[0].Period.ID != 10 || groups[0].Periods[0].Period.Name != "May 2026" {
		t.Fatalf("unexpected period: %+v", groups[0].Periods[0].Period)
	}
	if len(groups[0].Periods[0].Dues) != 1 || groups[0].Periods[0].Dues[0].UserName != "Budi" {
		t.Fatalf("unexpected dues: %+v", groups[0].Periods[0].Dues)
	}
	if groups[1].Periods[0].Period.ID != 0 || len(groups[1].Periods[0].Dues) != 1 {
		t.Fatalf("unexpected orphan group: %+v", groups[1])
	}
}

func TestListDuesByPeriods_GroupsTopPlans(t *testing.T) {
	period := models.PaymentBillingPeriod{ID: 10, Name: "May 2026"}
	plan := models.Plan{ID: 1, Name: "Netflix"}
	repo := &fakeDueRepo{
		periods:          []models.PaymentBillingPeriod{period},
		topPlansByPeriod: map[uint][]models.Plan{10: {plan}},
		duesByPeriodPlan: map[[2]uint][]models.PaymentDue{
			{10, 1}: {{ID: 100, PlanID: 1, UserID: 3, CalculatedPayAmount: 25000, PaymentStatus: models.PaymentStatusPending, Plan: plan}},
		},
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	groups, nextOffset, err := svc.ListDuesByPeriods(3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextOffset != 0 {
		t.Fatalf("want nextOffset 0, got %d", nextOffset)
	}
	if len(groups) != 1 || groups[0].Period.ID != 10 || groups[0].Period.Name != "May 2026" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	if len(groups[0].Plans) != 1 || groups[0].Plans[0].Plan.Name != "Netflix" || len(groups[0].Plans[0].Dues) != 1 {
		t.Fatalf("unexpected plan group: %+v", groups[0].Plans)
	}
}

func TestListDuesByUsers_LatestDues(t *testing.T) {
	repo := &fakeDueRepo{
		usersWithDues: []models.User{{ID: 3, Name: "Budi", Email: "budi@example.com"}},
		latestByUser: []models.PaymentDue{
			{ID: 100, UserID: 3, CalculatedPayAmount: 25000, PaymentStatus: models.PaymentStatusPending, Plan: models.Plan{ID: 1, Name: "Netflix"}},
		},
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	groups, nextOffset, err := svc.ListDuesByUsers(5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextOffset != 0 {
		t.Fatalf("want nextOffset 0, got %d", nextOffset)
	}
	if len(groups) != 1 || groups[0].User.Name != "Budi" || groups[0].User.Email != "budi@example.com" {
		t.Fatalf("unexpected group: %+v", groups)
	}
	if len(groups[0].Dues) != 1 || groups[0].Dues[0].PlanName != "Netflix" {
		t.Fatalf("unexpected dues: %+v", groups[0].Dues)
	}
}

func TestFilterOptions_MapsOptions(t *testing.T) {
	repo := &fakeDueRepo{
		allPlans: []models.Plan{{ID: 1, Name: "Netflix", TotalPrice: 100000}},
		allUsers: []models.User{{ID: 3, Name: "Budi", Email: "budi@example.com"}},
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	opts, err := svc.FilterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Plans) != 1 || opts.Plans[0].Name != "Netflix" || opts.Plans[0].TotalPrice != 100000 {
		t.Fatalf("unexpected plans: %+v", opts.Plans)
	}
	if len(opts.Users) != 1 || opts.Users[0].Email != "budi@example.com" {
		t.Fatalf("unexpected users: %+v", opts.Users)
	}
}

func TestGetDueByUUID_FoundAndMissing(t *testing.T) {
	repo := &fakeDueRepo{byUUID: map[string]*models.PaymentDue{
		"u-1": {ID: 1, UUID: "u-1", CalculatedPayAmount: 1000, PaymentStatus: models.PaymentStatusPending, Plan: models.Plan{ID: 2, Name: "Netflix"}},
	}}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	item, err := svc.GetDueByUUID("u-1")
	if err != nil || item == nil || item.PlanName != "Netflix" {
		t.Fatalf("unexpected result: %+v, %v", item, err)
	}

	missing, err := svc.GetDueByUUID("nope")
	if err != nil || missing != nil {
		t.Fatalf("want nil,nil for missing uuid, got %+v, %v", missing, err)
	}
}

func TestMarkDueComplete_MarksPendingPaid(t *testing.T) {
	repo := &fakeDueRepo{due: models.PaymentDue{ID: 1, CalculatedPayAmount: 5000, PaymentStatus: models.PaymentStatusPending}}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	if err := svc.MarkDueComplete(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.due.PaymentStatus != models.PaymentStatusPaid {
		t.Fatalf("want paid, got %s", repo.due.PaymentStatus)
	}
	if len(repo.paymentsCreated) != 1 || repo.paymentsCreated[0].ChannelPayment != "manual" {
		t.Fatalf("want manual payment record, got %+v", repo.paymentsCreated)
	}
}

func TestMarkDueComplete_SkipsAlreadyPaid(t *testing.T) {
	repo := &fakeDueRepo{due: models.PaymentDue{ID: 1, CalculatedPayAmount: 5000, PaymentStatus: models.PaymentStatusPaid}}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	if err := svc.MarkDueComplete(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.paymentsCreated) != 0 {
		t.Fatalf("should not create payment record, got %d", len(repo.paymentsCreated))
	}
}

func TestMarkDueComplete_MissingDue(t *testing.T) {
	svc := payment.NewService(&fakeDueRepo{}, &fakeSessionRepo{}, &fakeGateway{})
	if err := svc.MarkDueComplete(99); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMemberDuesSummary_FilterAndTotals(t *testing.T) {
	repo := &fakeDueRepo{
		forUser: []models.PaymentDue{
			{ID: 1, CalculatedPayAmount: 1000, PaymentStatus: models.PaymentStatusPending, Plan: models.Plan{ID: 2, Name: "Netflix"}},
		},
		sums: map[string]float64{
			models.PaymentStatusPending: 3000,
			models.PaymentStatusPaid:    7000,
		},
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	dues, totalPending, totalPaid, err := svc.MemberDuesSummary(3, "pending")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.listForUserArgs) != 1 || len(repo.listForUserArgs[0]) != 2 ||
		repo.listForUserArgs[0][0] != models.PaymentStatusPending || repo.listForUserArgs[0][1] != models.PaymentStatusOverdue {
		t.Fatalf("want pending+overdue statuses, got %v", repo.listForUserArgs)
	}
	if totalPending != 3000 || totalPaid != 7000 {
		t.Fatalf("unexpected totals: %f, %f", totalPending, totalPaid)
	}
	if len(dues) != 1 || dues[0].PlanName != "Netflix" {
		t.Fatalf("unexpected dues: %+v", dues)
	}

	if _, _, _, err := svc.MemberDuesSummary(3, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.listForUserArgs) != 2 || len(repo.listForUserArgs[1]) != 0 {
		t.Fatalf("want no status filter, got %v", repo.listForUserArgs[1])
	}
}

func TestUserDashboardStats_SumsPendingAndPaidThisMonth(t *testing.T) {
	repo := &fakeDueRepo{
		forUser: []models.PaymentDue{
			{ID: 2, DueDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), CalculatedPayAmount: 500, PaymentStatus: models.PaymentStatusOverdue, Plan: models.Plan{ID: 2, Name: "Netflix"}},
			{ID: 1, DueDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), CalculatedPayAmount: 1000, PaymentStatus: models.PaymentStatusPending, Plan: models.Plan{ID: 2, Name: "Netflix"}},
		},
		paidSince: 15000,
	}
	svc := payment.NewService(repo, &fakeSessionRepo{}, &fakeGateway{})

	stats, err := svc.UserDashboardStats(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.PendingAmount != 1500 {
		t.Fatalf("want pending sum incl. overdue 1500, got %f", stats.PendingAmount)
	}
	if stats.PendingCount != 2 {
		t.Fatalf("want 2 pending dues, got %d", stats.PendingCount)
	}
	if stats.PaidThisMonth != 15000 {
		t.Fatalf("want paid this month 15000, got %f", stats.PaidThisMonth)
	}
	if len(stats.PendingDues) != 2 || stats.PendingDues[0].ID != 1 || stats.PendingDues[1].ID != 2 {
		t.Fatalf("want dues ordered by due date asc, got %+v", stats.PendingDues)
	}
	if stats.PendingDues[0].PlanName != "Netflix" {
		t.Fatalf("want PlanName mapped, got %+v", stats.PendingDues[0])
	}
	if len(repo.listForUserArgs) != 1 || len(repo.listForUserArgs[0]) != 2 ||
		repo.listForUserArgs[0][0] != models.PaymentStatusPending || repo.listForUserArgs[0][1] != models.PaymentStatusOverdue {
		t.Fatalf("want pending+overdue statuses, got %v", repo.listForUserArgs)
	}
	if len(repo.paidSinceArgs) != 1 {
		t.Fatalf("want one paid-since query, got %d", len(repo.paidSinceArgs))
	}
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if !repo.paidSinceArgs[0].Equal(startOfMonth) {
		t.Fatalf("want start of month %v, got %v", startOfMonth, repo.paidSinceArgs[0])
	}
	if stats.ActivePlansCount != 0 {
		t.Fatalf("want ActivePlansCount left to caller, got %d", stats.ActivePlansCount)
	}
}
