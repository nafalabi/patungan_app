package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/notification"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when a plan does not exist.
	ErrNotFound = errors.New("plan not found")
	// ErrForbidden is returned when the actor may not access the plan.
	ErrForbidden = errors.New("not allowed")
	// ErrNoPortions is returned when a plan's participants hold 0 portions in total.
	ErrNoPortions = errors.New("total portions is 0")
)

// DueCreator is the cross-module seam to payment (satisfied by a thin adapter
// over payment repos, wired in main.go/worker).
type DueCreator interface {
	EnsureBillingPeriod(planID uint, dueDate time.Time, name string) (periodID uint, err error)
	CreateDue(due *models.PaymentDue) error
}

type Service struct {
	plans        PlanRepo
	dues         DueCreator
	scheduleTask *ProcessPlanScheduleTaskDef
}

func NewService(plans PlanRepo, dues DueCreator) *Service {
	svc := &Service{plans: plans, dues: dues}
	svc.scheduleTask = &ProcessPlanScheduleTaskDef{svc: svc}
	return svc
}

// List returns the paginated admin plans list.
func (s *Service) List(p ListParams) ([]PlanSummary, int64, error) {
	plans, total, err := s.plans.List(p)
	if err != nil {
		return nil, 0, err
	}
	return mapSummaries(plans), total, nil
}

// EnrolledPlans returns the plans a user participates in.
func (s *Service) EnrolledPlans(userID uint) ([]PlanSummary, error) {
	plans, err := s.plans.ListEnrolled(userID)
	if err != nil {
		return nil, err
	}
	return mapSummaries(plans), nil
}

// ListUsers returns user options for form dropdowns.
func (s *Service) ListUsers() ([]UserOption, error) {
	users, err := s.plans.ListUsers()
	if err != nil {
		return nil, err
	}
	options := make([]UserOption, 0, len(users))
	for _, u := range users {
		options = append(options, UserOption{ID: u.ID, Name: u.Name, Email: u.Email})
	}
	return options, nil
}

// ScheduleView returns the schedule-popup projection for a plan.
func (s *Service) ScheduleView(id uint) (ScheduleView, error) {
	p, err := s.plans.FindByIDWithTask(id)
	if err != nil {
		return ScheduleView{}, err
	}
	if p == nil {
		return ScheduleView{}, ErrNotFound
	}

	view := ScheduleView{
		ID:          p.ID,
		Name:        p.Name,
		NextDue:     p.NextDue(),
		PaymentType: p.PaymentType,
	}
	if p.ScheduledTask != nil {
		due := p.ScheduledTask.Due
		view.CurrentDue = &due
		view.TaskStatus = string(p.ScheduledTask.Status)
		view.TaskID = p.ScheduledTaskID
	}
	return view, nil
}

// GetForEdit returns the raw plan plus its participant portions for the edit form.
func (s *Service) GetForEdit(id uint) (*models.Plan, map[uint]int, error) {
	p, err := s.plans.FindByIDWithParticipants(id)
	if err != nil {
		return nil, nil, err
	}
	if p == nil {
		return nil, nil, ErrNotFound
	}

	portions := make(map[uint]int, len(p.Participants))
	for _, participant := range p.Participants {
		portions[participant.UserID] = participant.Portion
	}
	return p, portions, nil
}

// Create persists a new plan and its participants. Input is assumed validated.
func (s *Service) Create(name string, ownerID uint, totalPrice float64, paymentType string, recurring *string, start time.Time, allowInvitation bool, participants []models.PlanParticipant) error {
	p := models.Plan{
		Name:                    name,
		OwnerID:                 ownerID,
		TotalPrice:              totalPrice,
		PaymentType:             paymentType,
		RecurringInterval:       recurring,
		PlanStartDate:           start,
		AllowInvitationAfterPay: allowInvitation,
	}
	if err := s.plans.Create(&p); err != nil {
		return err
	}
	if len(participants) > 0 {
		p.Participants = participants
		return s.plans.Save(&p)
	}
	return nil
}

// Update applies validated input to a plan, enforcing the owner-healing /
// admin-check rule from UpdatePlan, then replaces its participants.
func (s *Service) Update(id uint, actorID uint, input UpdateInput) error {
	p, err := s.plans.FindByID(id)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrNotFound
	}

	// Authorization Check & Data Integrity Logic
	if p.OwnerID == 0 {
		// Data corruption healing: If plan has no owner (0), assign to current user
		p.OwnerID = actorID
	} else if p.OwnerID != actorID {
		// Strict ownership check: Only owner or Admin can edit
		user, err := s.plans.FindUser(actorID)
		if err == nil && user != nil && user.UserType != models.UserTypeAdmin {
			return ErrForbidden
		}
	}

	p.Name = input.Name
	p.TotalPrice = input.TotalPrice
	p.PaymentType = input.PaymentType
	if input.PaymentType == "recurring" && input.RecurringInterval != nil && *input.RecurringInterval != "" {
		p.RecurringInterval = input.RecurringInterval
	} else {
		p.RecurringInterval = nil
	}
	if !input.PlanStartDate.IsZero() {
		p.PlanStartDate = input.PlanStartDate
	}
	p.AllowInvitationAfterPay = input.AllowInvitationAfterPay

	if err := s.plans.Save(p); err != nil {
		return err
	}

	participants := make([]models.PlanParticipant, 0, len(input.Participants))
	for _, participant := range input.Participants {
		participant.PlanID = p.ID
		participants = append(participants, participant)
	}
	return s.plans.ReplaceParticipants(p.ID, participants)
}

// Delete removes a plan with the full cascade (refunds, canceled dues,
// disabled scheduled task).
func (s *Service) Delete(id uint) error {
	return s.plans.DeleteWithCascade(id)
}

// Schedule creates or refreshes the plan's scheduled processing task.
func (s *Service) Schedule(id uint) error {
	p, err := s.plans.FindByIDWithTask(id)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrNotFound
	}

	due := p.PlanStartDate
	if p.ScheduledTaskID != nil && p.ScheduledTask != nil {
		due = p.NextDue()
	}

	taskArgs := ProcessPlanScheduleArgs{
		PlanID:            p.ID,
		Due:               due,
		RecurringInterval: p.RecurringInterval,
	}

	createdTask, err := s.scheduleTask.CreateTask(taskArgs)
	if err != nil {
		return err
	}

	if p.ScheduledTaskID == nil {
		// Create new task
		if err := s.plans.CreateTask(createdTask); err != nil {
			return err
		}

		p.ScheduledTaskID = &createdTask.ID
		return s.plans.Save(p)
	}

	// Update existing task
	task := p.ScheduledTask
	task.TaskName = createdTask.TaskName
	task.Arguments = createdTask.Arguments
	task.Due = createdTask.Due
	task.RecurringInterval = createdTask.RecurringInterval
	task.Status = createdTask.Status
	task.TaskType = createdTask.TaskType
	task.MaxAttempt = createdTask.MaxAttempt
	task.LastRun = nil // Reset last run

	return s.plans.SaveTask(task)
}

// DisableSchedule disables the plan's scheduled processing task when present.
func (s *Service) DisableSchedule(id uint) error {
	p, err := s.plans.FindByIDWithTask(id)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrNotFound
	}

	if p.ScheduledTaskID != nil && p.ScheduledTask != nil {
		p.ScheduledTask.Status = models.ScheduledTaskStatusDisabled
		return s.plans.SaveTask(p.ScheduledTask)
	}
	return nil
}

// DetailForUser returns the plan detail and billing periods for a viewer.
// Non-admin viewers must be enrolled; otherwise ErrForbidden. Missing plans
// yield ErrNotFound.
func (s *Service) DetailForUser(planID, userID uint, isAdmin bool) (*PlanDetail, []PeriodView, error) {
	p, err := s.plans.FindByIDWithParticipants(planID)
	if err != nil {
		return nil, nil, err
	}
	if p == nil {
		return nil, nil, ErrNotFound
	}

	if !isAdmin {
		enrolled := false
		for _, participant := range p.Participants {
			if participant.UserID == userID {
				enrolled = true
				break
			}
		}
		if !enrolled {
			return nil, nil, ErrForbidden
		}
	}

	detail := &PlanDetail{
		Plan: PlanSummary{
			ID:         p.ID,
			Name:       p.Name,
			OwnerID:    p.OwnerID,
			OwnerName:  p.Owner.Name,
			TotalPrice: p.TotalPrice,
			StartDate:  p.PlanStartDate,
		},
		Participants: make([]ParticipantView, 0, len(p.Participants)),
	}
	for _, participant := range p.Participants {
		detail.Participants = append(detail.Participants, ParticipantView{
			UserID:  participant.UserID,
			Name:    participant.User.Name,
			Email:   participant.User.Email,
			Portion: participant.Portion,
		})
	}

	periods, err := s.plans.ListBillingPeriods(planID)
	if err != nil {
		return nil, nil, err
	}
	return detail, mapPeriods(periods), nil
}

// ProcessSchedule creates the billing period and per-participant dues for a
// plan run, then queues the notification task. Port of
// ProcessPlanScheduleTaskDef.HandleExecution.
func (s *Service) ProcessSchedule(planID uint, due time.Time) (map[string]interface{}, error) {
	p, err := s.plans.FindByIDWithParticipants(planID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plan: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("failed to fetch plan: %w", ErrNotFound)
	}

	if len(p.Participants) == 0 {
		return map[string]interface{}{"status": "skipped", "message": "No participants in plan"}, nil
	}

	totalPortions := 0
	for _, participant := range p.Participants {
		totalPortions += participant.Portion
	}

	if totalPortions == 0 {
		return nil, ErrNoPortions
	}

	pricePerPortion := p.TotalPrice / float64(totalPortions)
	periodName := due.Format("January 2006")

	periodID, err := s.dues.EnsureBillingPeriod(p.ID, due, periodName)
	if err != nil {
		return nil, fmt.Errorf("failed to create/fetch billing period: %w", err)
	}

	var createdDues []uint
	var notificationUsers []notification.NotificationUser

	appBaseURL := os.Getenv("APP_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:8080"
	}

	for _, participant := range p.Participants {
		amount := pricePerPortion * float64(participant.Portion)

		paymentDue := models.PaymentDue{
			PlanID:                 p.ID,
			UserID:                 participant.UserID,
			Portion:                participant.Portion,
			CalculatedPayAmount:    amount,
			PaymentStatus:          models.PaymentStatusPending,
			DueDate:                due,
			UUID:                   uuid.New().String(),
			PaymentBillingPeriodID: periodID,
		}
		if err := s.dues.CreateDue(&paymentDue); err != nil {
			log.Printf("Failed to create models.PaymentDue for user %d: %v", participant.UserID, err)
			continue
		}
		createdDues = append(createdDues, paymentDue.ID)

		paymentLink := fmt.Sprintf("%s/p/%s", appBaseURL, paymentDue.UUID)

		notificationUsers = append(notificationUsers, notification.NotificationUser{
			UserID:      participant.UserID,
			Username:    participant.User.Name,
			Email:       participant.User.Email,
			PhoneNumber: participant.User.Phone,
			PaymentLink: paymentLink,
		})
	}

	if len(notificationUsers) > 0 {
		notifArgs := notification.SendNotificationArgs{
			Users:         notificationUsers,
			NotifTemplate: "Halo $name, tagihan untuk plan $plan_name sudah jatuh tempo. Yuk segera dibayar di $paymentlink",
			Subject:       "Tagihan Plan " + p.Name,
			PlanName:      p.Name,
			Amount:        pricePerPortion,
			DueDate:       due.Format("02 Jan 2006"),
		}

		notifTask, err := notification.SendNotificationTask.CreateTask(notifArgs)
		if err != nil {
			log.Printf("Failed to create notification task args: %v", err)
		} else {
			if err := s.plans.CreateTask(notifTask); err != nil {
				log.Printf("Failed to create notification task: %v", err)
			}
		}

		serializedArgs, _ := json.Marshal(notifArgs)
		log.Printf("[Task ProcessPlanSchedule] Generated notification args: %s", string(serializedArgs))

		return map[string]interface{}{
			"status":            "success",
			"created_count":     len(createdDues),
			"total_portions":    totalPortions,
			"notification_args": string(serializedArgs),
		}, nil
	}

	return map[string]interface{}{
		"status":         "success",
		"created_count":  len(createdDues),
		"total_portions": totalPortions,
	}, nil
}

func mapSummaries(plans []models.Plan) []PlanSummary {
	summaries := make([]PlanSummary, 0, len(plans))
	for _, p := range plans {
		names := make([]string, 0, len(p.Participants))
		totalPortions := 0
		for _, participant := range p.Participants {
			names = append(names, participant.User.Name)
			totalPortions += participant.Portion
		}
		summary := PlanSummary{
			ID:         p.ID,
			Name:       p.Name,
			OwnerID:    p.OwnerID,
			OwnerName:  p.Owner.Name,
			TotalPrice: p.TotalPrice,
			StartDate:  p.PlanStartDate,

			PaymentType: p.PaymentType,
			NextDue:     p.NextDue(),

			ParticipantCount: len(p.Participants),
			ParticipantNames: names,
			TotalPortions:    totalPortions,
		}
		if p.ScheduledTask != nil {
			summary.ScheduledTaskStatus = string(p.ScheduledTask.Status)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func mapPeriods(periods []models.PaymentBillingPeriod) []PeriodView {
	views := make([]PeriodView, 0, len(periods))
	for _, period := range periods {
		view := PeriodView{
			ID:      period.ID,
			Name:    period.Name,
			DueDate: period.DueDate,
			Dues:    make([]DueView, 0, len(period.Dues)),
		}
		for _, due := range period.Dues {
			view.Dues = append(view.Dues, DueView{
				ID:       due.ID,
				UUID:     due.UUID,
				Amount:   due.CalculatedPayAmount,
				Status:   due.PaymentStatus,
				UserID:   due.UserID,
				UserName: due.User.Name,
				DueDate:  due.DueDate,
			})
		}
		views = append(views, view)
	}
	return views
}
