package user

import (
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// UserRepo persists users and their notification preferences. Find-style
// methods return (nil, nil) when the row is missing.
type UserRepo interface {
	List() ([]models.User, error)
	FindByID(id uint) (*models.User, error) // (nil, nil) when missing
	Create(user *models.User) error
	Save(user *models.User) error
	Delete(id uint) error // soft delete

	FindPreferenceByUserID(userID uint) (*models.UserNotifPreference, error) // (nil, nil) when missing
	SavePreference(pref *models.UserNotifPreference) error                   // insert when ID is unset, update otherwise
}

type gormUserRepo struct{ db *gorm.DB }

func NewGormUserRepo(db *gorm.DB) UserRepo { return &gormUserRepo{db: db} }

func (r *gormUserRepo) List() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *gormUserRepo) FindByID(id uint) (*models.User, error) {
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

func (r *gormUserRepo) Create(user *models.User) error { return r.db.Create(user).Error }
func (r *gormUserRepo) Save(user *models.User) error   { return r.db.Save(user).Error }

func (r *gormUserRepo) Delete(id uint) error { return r.db.Delete(&models.User{}, id).Error }

func (r *gormUserRepo) FindPreferenceByUserID(userID uint) (*models.UserNotifPreference, error) {
	var pref models.UserNotifPreference
	err := r.db.Where("user_id = ?", userID).First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &pref, nil
}

func (r *gormUserRepo) SavePreference(pref *models.UserNotifPreference) error {
	return r.db.Save(pref).Error
}
