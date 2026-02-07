package models

import (
	"time"

	"gorm.io/gorm"
)

// PlanParticipant represents the link between a Plan and a User with specific settings
type PlanParticipant struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	PlanID uint `json:"plan_id"`
	UserID uint `json:"user_id"`

	// Portion represents how many "shares" this user pays for. Default is 1.
	Portion int `gorm:"default:1" json:"portion"`

	// Relationships
	Plan Plan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
