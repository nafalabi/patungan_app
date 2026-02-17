package models

import (
	"time"

	"gorm.io/gorm"
)

type NotificationChannel string

const (
	NotificationChannelEmail    NotificationChannel = "email"
	NotificationChannelWhatsapp NotificationChannel = "whatsapp"
)

type WorkspaceType string

const (
	WhatsappTargetTypePersonal = "personal"
	WhatsappTargetTypeGroup    = "group"
)

type UserNotifPreference struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	UserID uint `gorm:"uniqueIndex" json:"user_id"`

	Channel NotificationChannel `gorm:"type:varchar(20);default:'email'" json:"channel"`

	// WhatsApp specific options
	WhatsappTargetType string `gorm:"type:varchar(20);default:'personal'" json:"whatsapp_target_type"` // 'personal' or 'group'
	WhatsappGroupID    string `gorm:"type:varchar(100)" json:"whatsapp_group_id"`                      // Group ID if target type is group
}
