package models

import (
	"time"

	"gorm.io/gorm"
)

// UserType represents the type of user
type UserType string

const (
	UserTypeAdmin  UserType = "Admin"
	UserTypeMember UserType = "Member"
)

// User represents a user in the system
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name     string   `gorm:"type:varchar(255)" json:"name"`
	Phone    string   `gorm:"type:varchar(50)" json:"phone"`
	Email    string   `gorm:"type:varchar(255);uniqueIndex" json:"email"`
	UserType UserType `gorm:"type:varchar(20);default:'Member'" json:"user_type"`

	// Relationships
	Plans        []Plan        `gorm:"many2many:plan_user;" json:"plans,omitempty"`
	UserPayments []UserPayment `gorm:"foreignKey:UserID" json:"user_payments,omitempty"`
	Refunds      []Refund      `gorm:"foreignKey:UserID" json:"refunds,omitempty"`
}
