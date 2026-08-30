package payment

import (
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// DueRepo persists payment dues. Implementations must translate
// gorm.ErrRecordNotFound to (nil, nil) where documented.
type DueRepo interface {
	FindByID(id uint) (*models.PaymentDue, error)
	FindByUUID(uuid string) (*models.PaymentDue, error) // (nil, nil) when missing
	Save(due *models.PaymentDue) error
	Create(due *models.PaymentDue) error
	CreatePaymentRecord(p *models.UserPayment) error
	CreateCallbackHistory(h *models.PaymentCallbackHistory) error
	ListFlat(params ListFlatParams) ([]models.PaymentDue, int64, error)
	ListPlansWithLatestDues(limit, offset int) ([]models.Plan, error)
	ListPeriods(limit, offset int) ([]models.PaymentBillingPeriod, error)
	ListUsersWithLatestDues(limit, offset int) ([]models.User, error)
	ListByPlanLatestPeriod(planID uint) (*models.PaymentBillingPeriod, []models.PaymentDue, error) // (nil, nil, nil) when no dues
	ListOrphanDuesByPlan(planID uint) ([]models.PaymentDue, error)                                 // payment_billing_period_id = 0
	ListTopPlansInPeriod(periodID uint, limit int) ([]models.Plan, error)
	ListByPeriodAndPlan(periodID, planID uint) ([]models.PaymentDue, error)
	ListLatestByUser(userID uint, limit int) ([]models.PaymentDue, error)
	ListPlans() ([]models.Plan, error)
	ListUsers() ([]models.User, error)
	ListForUser(userID uint, statuses []string) ([]models.PaymentDue, error) // statuses empty = all
	SumForUserByStatus(userID uint, statuses []string) (float64, error)      // statuses empty = all
}

// SessionRepo persists payment sessions. FindLatestActive, FindByOrderID and
// FindLatestByGatewayMetadata return (nil, nil) when no row matches.
type SessionRepo interface {
	FindLatestActive(dueID uint) (*models.PaymentSession, error)
	FindByOrderID(orderID string) (*models.PaymentSession, error)
	FindLatestByGatewayMetadata(gateway models.PaymentGateway, metadataSubstring string) (*models.PaymentSession, error)
	Save(s *models.PaymentSession) error
	Create(s *models.PaymentSession) error
}

type gormDueRepo struct{ db *gorm.DB }

func NewGormDueRepo(db *gorm.DB) DueRepo { return &gormDueRepo{db: db} }

func (r *gormDueRepo) FindByID(id uint) (*models.PaymentDue, error) {
	var due models.PaymentDue
	err := r.db.Preload("Plan").Preload("User").First(&due, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &due, nil
}

func (r *gormDueRepo) Save(due *models.PaymentDue) error               { return r.db.Save(due).Error }
func (r *gormDueRepo) Create(due *models.PaymentDue) error             { return r.db.Create(due).Error }
func (r *gormDueRepo) CreatePaymentRecord(p *models.UserPayment) error { return r.db.Create(p).Error }

func (r *gormDueRepo) CreateCallbackHistory(h *models.PaymentCallbackHistory) error {
	return r.db.Create(h).Error
}

func (r *gormDueRepo) FindByUUID(uuid string) (*models.PaymentDue, error) {
	var due models.PaymentDue
	err := r.db.Preload("Plan").Preload("User").Where("uuid = ?", uuid).First(&due).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &due, nil
}

func (r *gormDueRepo) ListFlat(params ListFlatParams) ([]models.PaymentDue, int64, error) {
	query := r.db.Model(&models.PaymentDue{}).
		Preload("Plan").Preload("User").Preload("BillingPeriod").
		Where("payment_status != ?", models.PaymentStatusCanceled)

	if params.FilterPlan != 0 {
		query = query.Where("plan_id = ?", params.FilterPlan)
	}
	if params.FilterUser != 0 {
		query = query.Where("user_id = ?", params.FilterUser)
	}

	var totalCount int64
	query.Count(&totalCount)

	switch params.SortBy {
	case "plan":
		query = query.Joins("JOIN plans ON plans.id = payment_dues.plan_id").Order("plans.name " + params.SortOrder)
	case "user":
		query = query.Joins("JOIN users ON users.id = payment_dues.user_id").Order("users.name " + params.SortOrder)
	default:
		query = query.Order(params.SortBy + " " + params.SortOrder)
	}

	var dues []models.PaymentDue
	err := query.Limit(params.PageSize).Offset((params.Page - 1) * params.PageSize).Find(&dues).Error
	return dues, totalCount, err
}

func (r *gormDueRepo) ListPlansWithLatestDues(limit, offset int) ([]models.Plan, error) {
	var plans []models.Plan
	err := r.db.Table("plans").
		Select("plans.*").
		Joins("JOIN (SELECT plan_id, MAX(due_date) as latest FROM payment_dues GROUP BY plan_id) as ld ON ld.plan_id = plans.id").
		Order("ld.latest DESC, plans.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&plans).Error
	return plans, err
}

func (r *gormDueRepo) ListPeriods(limit, offset int) ([]models.PaymentBillingPeriod, error) {
	var periods []models.PaymentBillingPeriod
	err := r.db.Order("due_date DESC, id DESC").Limit(limit).Offset(offset).Find(&periods).Error
	return periods, err
}

func (r *gormDueRepo) ListUsersWithLatestDues(limit, offset int) ([]models.User, error) {
	var users []models.User
	err := r.db.Table("users").
		Select("users.*").
		Joins("JOIN (SELECT user_id, MAX(due_date) as latest FROM payment_dues GROUP BY user_id) as ld ON ld.user_id = users.id").
		Order("ld.latest DESC, users.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

func (r *gormDueRepo) ListByPlanLatestPeriod(planID uint) (*models.PaymentBillingPeriod, []models.PaymentDue, error) {
	var latestPeriod models.PaymentBillingPeriod
	err := r.db.Table("payment_billing_periods").
		Joins("JOIN payment_dues ON payment_dues.payment_billing_period_id = payment_billing_periods.id").
		Where("payment_dues.plan_id = ?", planID).
		Order("payment_billing_periods.due_date DESC").
		First(&latestPeriod).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	var dues []models.PaymentDue
	err = r.db.Preload("User").Preload("BillingPeriod").
		Where("plan_id = ? AND payment_billing_period_id = ?", planID, latestPeriod.ID).
		Find(&dues).Error
	if err != nil {
		return nil, nil, err
	}
	return &latestPeriod, dues, nil
}

func (r *gormDueRepo) ListOrphanDuesByPlan(planID uint) ([]models.PaymentDue, error) {
	var dues []models.PaymentDue
	err := r.db.Preload("User").
		Where("plan_id = ? AND payment_billing_period_id = 0", planID).
		Find(&dues).Error
	return dues, err
}

func (r *gormDueRepo) ListTopPlansInPeriod(periodID uint, limit int) ([]models.Plan, error) {
	var plans []models.Plan
	err := r.db.Table("plans").
		Select("plans.*").
		Joins("JOIN payment_dues ON payment_dues.plan_id = plans.id").
		Where("payment_dues.payment_billing_period_id = ?", periodID).
		Group("plans.id").
		Order("plans.id ASC").
		Limit(limit).
		Find(&plans).Error
	return plans, err
}

func (r *gormDueRepo) ListByPeriodAndPlan(periodID, planID uint) ([]models.PaymentDue, error) {
	var dues []models.PaymentDue
	err := r.db.Preload("User").Preload("BillingPeriod").
		Where("payment_billing_period_id = ? AND plan_id = ?", periodID, planID).
		Find(&dues).Error
	return dues, err
}

func (r *gormDueRepo) ListLatestByUser(userID uint, limit int) ([]models.PaymentDue, error) {
	var dues []models.PaymentDue
	err := r.db.Preload("Plan").Preload("BillingPeriod").
		Where("user_id = ?", userID).
		Order("due_date DESC").
		Limit(limit).
		Find(&dues).Error
	return dues, err
}

func (r *gormDueRepo) ListPlans() ([]models.Plan, error) {
	var plans []models.Plan
	err := r.db.Find(&plans).Error
	return plans, err
}

func (r *gormDueRepo) ListUsers() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *gormDueRepo) ListForUser(userID uint, statuses []string) ([]models.PaymentDue, error) {
	var dues []models.PaymentDue
	query := r.db.Where("user_id = ?", userID).Preload("Plan")
	if len(statuses) > 0 {
		query = query.Where("payment_status IN ?", statuses)
	}
	err := query.Order("due_date DESC").Find(&dues).Error
	return dues, err
}

func (r *gormDueRepo) SumForUserByStatus(userID uint, statuses []string) (float64, error) {
	var total float64
	query := r.db.Model(&models.PaymentDue{}).Where("user_id = ?", userID)
	if len(statuses) > 0 {
		query = query.Where("payment_status IN ?", statuses)
	}
	err := query.Select("COALESCE(SUM(calculated_pay_amount), 0)").Scan(&total).Error
	return total, err
}

type gormSessionRepo struct{ db *gorm.DB }

func NewGormSessionRepo(db *gorm.DB) SessionRepo { return &gormSessionRepo{db: db} }

func (r *gormSessionRepo) FindLatestActive(dueID uint) (*models.PaymentSession, error) {
	var s models.PaymentSession
	err := r.db.Where("payment_due_id = ? AND is_active = ?", dueID, true).Order("created_at desc").First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *gormSessionRepo) FindByOrderID(orderID string) (*models.PaymentSession, error) {
	var s models.PaymentSession
	err := r.db.Where("order_id = ?", orderID).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *gormSessionRepo) Save(s *models.PaymentSession) error   { return r.db.Save(s).Error }
func (r *gormSessionRepo) Create(s *models.PaymentSession) error { return r.db.Create(s).Error }

func (r *gormSessionRepo) FindLatestByGatewayMetadata(gateway models.PaymentGateway, metadataSubstring string) (*models.PaymentSession, error) {
	var s models.PaymentSession
	err := r.db.Where("payment_gateway = ? AND response_metadata LIKE ?", gateway, "%"+metadataSubstring+"%").Order("created_at desc").First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}
