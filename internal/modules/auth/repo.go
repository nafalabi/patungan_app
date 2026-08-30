package auth

import (
	"patungan_app_echo/internal/models"

	"gorm.io/gorm"
)

// UserRepo looks up registered users for login. FindByEmail returns
// (nil, nil) when no user has the given email.
type UserRepo interface {
	FindByEmail(email string) (*models.User, error)
}

type gormUserRepo struct{ db *gorm.DB }

func NewGormUserRepo(db *gorm.DB) UserRepo { return &gormUserRepo{db: db} }

func (r *gormUserRepo) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
