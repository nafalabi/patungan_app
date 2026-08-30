package plan_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/plan"
)

type fakeDueCreator struct {
	periods map[string]uint
	dues    []models.PaymentDue
}

func (f *fakeDueCreator) EnsureBillingPeriod(planID uint, due time.Time, name string) (uint, error) {
	key := name
	if id, ok := f.periods[key]; ok {
		return id, nil
	}
	id := uint(len(f.periods) + 1)
	f.periods[key] = id
	return id, nil
}

func (f *fakeDueCreator) CreateDue(due *models.PaymentDue) error {
	f.dues = append(f.dues, *due)
	return nil
}

type fakePlanRepo struct {
	plan  models.Plan
	plans []models.Plan
	total int64
	users map[uint]models.User

	savedPlans   []*models.Plan
	createdTasks []*models.ScheduledTask
	savedTasks   []*models.ScheduledTask
	replaced     map[uint][]models.PlanParticipant
	deleted      []uint
}

func (f *fakePlanRepo) FindByID(id uint) (*models.Plan, error) {
	if f.plan.ID == id {
		p := f.plan
		return &p, nil
	}
	return nil, nil
}

func (f *fakePlanRepo) FindByIDWithParticipants(id uint) (*models.Plan, error) {
	return f.FindByID(id)
}

func (f *fakePlanRepo) FindByIDWithTask(id uint) (*models.Plan, error) {
	return f.FindByID(id)
}

func (f *fakePlanRepo) List(p plan.ListParams) ([]models.Plan, int64, error) {
	return f.plans, f.total, nil
}

func (f *fakePlanRepo) ListEnrolled(userID uint) ([]models.Plan, error) {
	return f.plans, nil
}

func (f *fakePlanRepo) ListBillingPeriods(planID uint) ([]models.PaymentBillingPeriod, error) {
	return nil, nil
}

func (f *fakePlanRepo) Create(p *models.Plan) error {
	if p.ID == 0 {
		p.ID = uint(len(f.savedPlans) + len(f.plans) + 1)
	}
	f.plan = *p
	return nil
}

func (f *fakePlanRepo) Save(p *models.Plan) error {
	f.savedPlans = append(f.savedPlans, p)
	f.plan = *p
	return nil
}

func (f *fakePlanRepo) DeleteWithCascade(planID uint) error {
	f.deleted = append(f.deleted, planID)
	return nil
}

func (f *fakePlanRepo) ReplaceParticipants(planID uint, participants []models.PlanParticipant) error {
	if f.replaced == nil {
		f.replaced = map[uint][]models.PlanParticipant{}
	}
	f.replaced[planID] = participants
	return nil
}

func (f *fakePlanRepo) SaveTask(task *models.ScheduledTask) error {
	f.savedTasks = append(f.savedTasks, task)
	return nil
}

func (f *fakePlanRepo) CreateTask(task *models.ScheduledTask) error {
	if task.ID == 0 {
		task.ID = uint(len(f.createdTasks) + 1)
	}
	f.createdTasks = append(f.createdTasks, task)
	return nil
}

func (f *fakePlanRepo) FindUser(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return &u, nil
	}
	return nil, nil
}

func (f *fakePlanRepo) ListUsers() ([]models.User, error) {
	users := make([]models.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *fakePlanRepo) ParticipantExists(planID, userID uint) (bool, error) {
	if f.plan.ID != planID {
		return false, nil
	}
	for _, p := range f.plan.Participants {
		if p.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakePlanRepo) CountActiveForUser(userID uint) (int64, error) {
	return f.total, nil
}

func (f *fakePlanRepo) CountAll() (int64, error) { return f.total, nil }

func TestProcessSchedule_CreatesDuesPerPortion(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix", TotalPrice: 90000,
			Participants: []models.PlanParticipant{
				{UserID: 10, Portion: 2, User: models.User{ID: 10, Name: "A", Email: "a@x.com", Phone: "1"}},
				{UserID: 11, Portion: 1, User: models.User{ID: 11, Name: "B", Email: "b@x.com", Phone: "2"}},
			},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	res, err := svc.ProcessSchedule(1, due)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["created_count"] != 2 {
		t.Fatalf("want 2 dues, got %v", res)
	}
	if dues.dues[0].CalculatedPayAmount != 60000 {
		t.Fatalf("want 60000 for portion 2, got %v", dues.dues[0].CalculatedPayAmount)
	}
	if dues.dues[1].CalculatedPayAmount != 30000 {
		t.Fatalf("want 30000 for portion 1, got %v", dues.dues[1].CalculatedPayAmount)
	}
	if dues.dues[0].UUID == "" {
		t.Fatalf("want UUID set on created due")
	}
	if dues.dues[0].PaymentStatus != models.PaymentStatusPending {
		t.Fatalf("want pending status, got %s", dues.dues[0].PaymentStatus)
	}
}

func TestProcessSchedule_SkipsWhenNoParticipants(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{ID: 1, Name: "Netflix", TotalPrice: 90000},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	res, err := svc.ProcessSchedule(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["status"] != "skipped" {
		t.Fatalf("want skipped status, got %v", res)
	}
	if len(dues.dues) != 0 {
		t.Fatalf("want no dues created, got %d", len(dues.dues))
	}
}

func TestProcessSchedule_ErrNoPortions(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix", TotalPrice: 90000,
			Participants: []models.PlanParticipant{
				{UserID: 10, Portion: 0, User: models.User{ID: 10}},
			},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	_, err := svc.ProcessSchedule(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, plan.ErrNoPortions) {
		t.Fatalf("want ErrNoPortions, got %v", err)
	}
}

func TestProcessSchedule_CreatesNotificationTask(t *testing.T) {
	t.Setenv("APP_URL", "http://test.example")
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix", TotalPrice: 90000,
			Participants: []models.PlanParticipant{
				{UserID: 10, Portion: 1, User: models.User{ID: 10, Name: "A", Email: "a@x.com", Phone: "1"}},
			},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	res, err := svc.ProcessSchedule(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.createdTasks) != 1 {
		t.Fatalf("want 1 notification task created, got %d", len(plans.createdTasks))
	}
	if plans.createdTasks[0].TaskName != "send_notification" {
		t.Fatalf("want send_notification task, got %s", plans.createdTasks[0].TaskName)
	}
	args, _ := res["notification_args"].(string)
	if !strings.Contains(args, "http://test.example/p/") {
		t.Fatalf("want APP_URL payment link in notification args, got %s", args)
	}
}

func TestProcessSchedule_NotFound(t *testing.T) {
	plans := &fakePlanRepo{}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	_, err := svc.ProcessSchedule(99, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatalf("want error for missing plan, got nil")
	}
}

func TestSchedule_CreatesTaskAndLinksPlan(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{ID: 1, Name: "Netflix", TotalPrice: 90000, PaymentType: "onetime"},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	if err := svc.Schedule(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.createdTasks) != 1 {
		t.Fatalf("want 1 scheduled task, got %d", len(plans.createdTasks))
	}
	created := plans.createdTasks[0]
	if created.TaskName != "process_plan_schedule" {
		t.Fatalf("want process_plan_schedule task, got %s", created.TaskName)
	}
	if created.Status != models.ScheduledTaskStatusActive {
		t.Fatalf("want active task, got %s", created.Status)
	}
	if len(plans.savedPlans) != 1 {
		t.Fatalf("want plan saved once, got %d saves", len(plans.savedPlans))
	}
	if plans.savedPlans[0].ScheduledTaskID == nil || *plans.savedPlans[0].ScheduledTaskID != created.ID {
		t.Fatalf("want plan linked to created task, got %+v", plans.savedPlans[0])
	}
}

func TestSchedule_UpdatesExistingTask(t *testing.T) {
	lastRun := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	taskID := uint(7)
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix", TotalPrice: 90000, PaymentType: "onetime",
			ScheduledTaskID: &taskID,
			ScheduledTask: &models.ScheduledTask{
				ID: taskID, TaskName: "process_plan_schedule", Status: models.ScheduledTaskStatusDone, LastRun: &lastRun,
			},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	if err := svc.Schedule(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.savedTasks) != 1 {
		t.Fatalf("want 1 saved task, got %d", len(plans.savedTasks))
	}
	saved := plans.savedTasks[0]
	if saved.ID != taskID {
		t.Fatalf("want task %d updated in place, got %d", taskID, saved.ID)
	}
	if saved.Status != models.ScheduledTaskStatusActive {
		t.Fatalf("want active status, got %s", saved.Status)
	}
	if saved.LastRun != nil {
		t.Fatalf("want last run reset, got %v", saved.LastRun)
	}
}

func TestDisableSchedule(t *testing.T) {
	taskID := uint(7)
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix",
			ScheduledTaskID: &taskID,
			ScheduledTask:   &models.ScheduledTask{ID: taskID, Status: models.ScheduledTaskStatusActive},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	if err := svc.DisableSchedule(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.savedTasks) != 1 || plans.savedTasks[0].Status != models.ScheduledTaskStatusDisabled {
		t.Fatalf("want disabled task, got %+v", plans.savedTasks)
	}
}

func TestUpdate_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{ID: 1, Name: "Netflix", OwnerID: 5},
		users: map[uint]models.User{
			6: {ID: 6, UserType: models.UserTypeMember},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	err := svc.Update(1, 6, plan.UpdateInput{Name: "New"})
	if !errors.Is(err, plan.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestUpdate_HealsMissingOwner(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{ID: 1, Name: "Netflix", OwnerID: 0},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	if err := svc.Update(1, 6, plan.UpdateInput{Name: "New"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.savedPlans) != 1 || plans.savedPlans[0].OwnerID != 6 {
		t.Fatalf("want owner healed to actor 6, got %+v", plans.savedPlans)
	}
}

func TestUpdate_ReplacesParticipants(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{ID: 1, Name: "Netflix", OwnerID: 5},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	input := plan.UpdateInput{
		Name:         "New",
		Participants: []models.PlanParticipant{{UserID: 10, Portion: 2}},
	}
	if err := svc.Update(1, 5, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := plans.replaced[1]
	if len(got) != 1 || got[0].UserID != 10 || got[0].PlanID != 1 {
		t.Fatalf("want participant stamped with PlanID 1, got %+v", got)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	plans := &fakePlanRepo{}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	err := svc.Update(99, 1, plan.UpdateInput{Name: "X"})
	if !errors.Is(err, plan.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDetailForUser(t *testing.T) {
	plans := &fakePlanRepo{
		plan: models.Plan{
			ID: 1, Name: "Netflix", OwnerID: 5, TotalPrice: 90000, PaymentType: "recurring",
			Participants: []models.PlanParticipant{
				{UserID: 10, Portion: 1, User: models.User{ID: 10, Name: "A", Email: "a@x.com"}},
				{UserID: 11, Portion: 2, User: models.User{ID: 11, Name: "B", Email: "b@x.com"}},
			},
		},
	}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	detail, _, err := svc.DetailForUser(1, 10, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Plan.PaymentType != "recurring" {
		t.Fatalf("want payment type recurring, got %q", detail.Plan.PaymentType)
	}
	if detail.Plan.ParticipantCount != 2 {
		t.Fatalf("want participant count 2, got %d", detail.Plan.ParticipantCount)
	}
	if len(detail.Participants) != 2 || detail.Participants[0].Name != "A" {
		t.Fatalf("want 2 participant views, got %+v", detail.Participants)
	}

	// Non-admin requesting a missing plan: enrollment is checked before plan
	// existence (old handler returned 403 in this case).
	if _, _, err := svc.DetailForUser(99, 10, false); !errors.Is(err, plan.ErrForbidden) {
		t.Fatalf("want ErrForbidden for non-admin on missing plan, got %v", err)
	}

	// Admin requesting a missing plan still gets ErrNotFound.
	if _, _, err := svc.DetailForUser(99, 1, true); !errors.Is(err, plan.ErrNotFound) {
		t.Fatalf("want ErrNotFound for admin on missing plan, got %v", err)
	}

	// Non-admin not enrolled in an existing plan gets ErrForbidden.
	if _, _, err := svc.DetailForUser(1, 99, false); !errors.Is(err, plan.ErrForbidden) {
		t.Fatalf("want ErrForbidden for non-enrolled user, got %v", err)
	}
}

func TestDelete_DelegatesToCascade(t *testing.T) {
	plans := &fakePlanRepo{}
	dues := &fakeDueCreator{periods: map[string]uint{}}
	svc := plan.NewService(plans, dues)

	if err := svc.Delete(3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans.deleted) != 1 || plans.deleted[0] != 3 {
		t.Fatalf("want cascade delete for plan 3, got %v", plans.deleted)
	}
}
