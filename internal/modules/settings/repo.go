package settings

import (
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// SettingsRepo persists the singleton settings row. Get returns the first
// (only) settings record; a missing row surfaces as an error.
type SettingsRepo interface {
	Get() (*models.Settings, error)
	Save(*models.Settings) error
}

type gormSettingsRepo struct{ db *gorm.DB }

func NewGormSettingsRepo(db *gorm.DB) SettingsRepo { return &gormSettingsRepo{db: db} }

func (r *gormSettingsRepo) Get() (*models.Settings, error) {
	var settings models.Settings
	if err := r.db.First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *gormSettingsRepo) Save(settings *models.Settings) error { return r.db.Save(settings).Error }
