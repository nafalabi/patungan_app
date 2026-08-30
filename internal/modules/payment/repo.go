package payment

import (
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// DueRepo persists payment dues. Implementations must translate
// gorm.ErrRecordNotFound to (nil, nil) where documented.
type DueRepo interface {
	FindByID(id uint) (*models.PaymentDue, error)
	Save(due *models.PaymentDue) error
	Create(due *models.PaymentDue) error
	CreatePaymentRecord(p *models.UserPayment) error
}

// SessionRepo persists payment sessions. FindLatestActive and FindByOrderID
// return (nil, nil) when no row matches.
type SessionRepo interface {
	FindLatestActive(dueID uint) (*models.PaymentSession, error)
	FindByOrderID(orderID string) (*models.PaymentSession, error)
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
