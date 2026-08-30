package payment

import (
	"time"

	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// DuesCreatorAdapter implements plan.DueCreator using payment repos.
type DuesCreatorAdapter struct {
	db *gorm.DB
}

func NewDuesCreatorAdapter(db *gorm.DB) *DuesCreatorAdapter { return &DuesCreatorAdapter{db: db} }

func (a *DuesCreatorAdapter) EnsureBillingPeriod(planID uint, dueDate time.Time, name string) (uint, error) {
	var period models.PaymentBillingPeriod
	err := a.db.Where(models.PaymentBillingPeriod{PlanID: planID, DueDate: dueDate}).
		FirstOrCreate(&period, models.PaymentBillingPeriod{PlanID: planID, DueDate: dueDate, Name: name}).Error
	return period.ID, err
}

func (a *DuesCreatorAdapter) CreateDue(due *models.PaymentDue) error {
	return a.db.Create(due).Error
}
