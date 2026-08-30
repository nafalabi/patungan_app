package plan

import (
	"time"

	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// PlanRepo persists plans and their related aggregates. Find-style methods
// return (nil, nil) when the row is missing.
type PlanRepo interface {
	FindByID(id uint) (*models.Plan, error)                                // (nil, nil) when missing
	FindByIDWithParticipants(id uint) (*models.Plan, error)                // Preload Owner + Participants.User
	FindByIDWithTask(id uint) (*models.Plan, error)                        // Preload ScheduledTask
	List(p ListParams) ([]models.Plan, int64, error)                       // Preload Owner, ScheduledTask, Participants.User
	ListEnrolled(userID uint) ([]models.Plan, error)                       // join plan_participants, Preload Owner + Participants.User
	ListBillingPeriods(planID uint) ([]models.PaymentBillingPeriod, error) // Preload Dues.User, order due_date DESC
	Create(plan *models.Plan) error
	Save(plan *models.Plan) error
	DeleteWithCascade(planID uint) error // refunds + cancel dues + task disable + delete
	ReplaceParticipants(planID uint, participants []models.PlanParticipant) error
	SaveTask(task *models.ScheduledTask) error
	CreateTask(task *models.ScheduledTask) error
	FindUser(id uint) (*models.User, error) // for Update admin check; (nil, nil) when missing
	CountActiveForUser(userID uint) (int64, error)
	CountAll() (int64, error)
}

type gormPlanRepo struct{ db *gorm.DB }

func NewGormPlanRepo(db *gorm.DB) PlanRepo { return &gormPlanRepo{db: db} }

func (r *gormPlanRepo) FindByID(id uint) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.First(&plan, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *gormPlanRepo) FindByIDWithParticipants(id uint) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.Preload("Owner").Preload("Participants.User").First(&plan, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *gormPlanRepo) FindByIDWithTask(id uint) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.Preload("ScheduledTask").First(&plan, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *gormPlanRepo) List(p ListParams) ([]models.Plan, int64, error) {
	query := r.db.Model(&models.Plan{}).Preload("Owner").Preload("ScheduledTask").Preload("Participants.User")

	if p.FilterOwner > 0 {
		query = query.Where("owner_id = ?", p.FilterOwner)
	}
	if p.FilterType != "" {
		query = query.Where("payment_type = ?", p.FilterType)
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	pageSize := p.PageSize
	if pageSize <= 0 {
		pageSize = 15
	}
	page := p.Page
	if page <= 0 {
		page = 1
	}
	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	sortOrder := p.SortOrder
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	switch p.SortBy {
	case "name":
		query = query.Order("name " + sortOrder)
	case "date":
		query = query.Order("plan_start_date " + sortOrder)
	case "price":
		query = query.Order("total_price " + sortOrder)
	default:
		query = query.Order("created_at " + sortOrder)
	}

	var plans []models.Plan
	err := query.Limit(pageSize).Offset(offset).Find(&plans).Error
	return plans, totalCount, err
}

func (r *gormPlanRepo) ListEnrolled(userID uint) ([]models.Plan, error) {
	var plans []models.Plan
	err := r.db.Joins("JOIN plan_participants ON plan_participants.plan_id = plans.id AND plan_participants.deleted_at IS NULL").
		Where("plan_participants.user_id = ?", userID).
		Preload("Owner").
		Preload("Participants.User").
		Order("plans.created_at DESC").
		Find(&plans).Error
	return plans, err
}

func (r *gormPlanRepo) ListBillingPeriods(planID uint) ([]models.PaymentBillingPeriod, error) {
	var periods []models.PaymentBillingPeriod
	err := r.db.Where("plan_id = ?", planID).
		Preload("Dues.User").
		Order("due_date DESC").
		Find(&periods).Error
	return periods, err
}

func (r *gormPlanRepo) Create(plan *models.Plan) error { return r.db.Create(plan).Error }
func (r *gormPlanRepo) Save(plan *models.Plan) error   { return r.db.Save(plan).Error }

func (r *gormPlanRepo) DeleteWithCascade(planID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var plan models.Plan
		if err := tx.Preload("ScheduledTask").First(&plan, planID).Error; err != nil {
			return err
		}

		var paymentDues []models.PaymentDue
		tx.Preload("UserPayment").Where("plan_id = ?", planID).Find(&paymentDues)

		for _, due := range paymentDues {
			if due.PaymentStatus == models.PaymentStatusPaid {
				if due.UserPayment != nil {
					refund := models.Refund{
						PlanID:         planID,
						PaymentDueID:   due.ID,
						UserPaymentID:  due.UserPayment.ID,
						UserID:         due.UserID,
						TotalRefund:    due.UserPayment.TotalPay,
						ChannelPayment: due.UserPayment.ChannelPayment,
						RefundDate:     time.Now(),
					}
					if err := tx.Create(&refund).Error; err != nil {
						return err
					}
				}
			}
			if err := tx.Model(&due).Update("payment_status", models.PaymentStatusCanceled).Error; err != nil {
				return err
			}
		}

		if plan.ScheduledTask != nil {
			if err := tx.Model(&plan.ScheduledTask).Update("status", models.ScheduledTaskStatusDisabled).Error; err != nil {
				return err
			}
		}

		return tx.Delete(&plan).Error
	})
}

func (r *gormPlanRepo) ReplaceParticipants(planID uint, participants []models.PlanParticipant) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&models.PlanParticipant{}).Error; err != nil {
			return err
		}
		if len(participants) > 0 {
			if err := tx.Create(&participants).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormPlanRepo) SaveTask(task *models.ScheduledTask) error   { return r.db.Save(task).Error }
func (r *gormPlanRepo) CreateTask(task *models.ScheduledTask) error { return r.db.Create(task).Error }

func (r *gormPlanRepo) FindUser(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *gormPlanRepo) CountActiveForUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Plan{}).
		Joins("LEFT JOIN plan_participants ON plan_participants.plan_id = plans.id").
		Where("plans.owner_id = ? OR plan_participants.user_id = ?", userID, userID).
		Distinct("plans.id").
		Count(&count).Error
	return count, err
}

func (r *gormPlanRepo) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&models.Plan{}).Count(&count).Error
	return count, err
}
