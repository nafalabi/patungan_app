package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/web/templates/pages"
)

type UserPreferenceHandler struct {
	DB *gorm.DB
}

func NewUserPreferenceHandler(db *gorm.DB) *UserPreferenceHandler {
	return &UserPreferenceHandler{DB: db}
}

// GetUserPreference returns the preference modal content for HTMX
func (h *UserPreferenceHandler) GetUserPreference(c echo.Context) error {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid user ID")
	}

	var pref models.UserNotifPreference
	err = h.DB.Where("user_id = ?", userID).First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Default values
			pref = models.UserNotifPreference{
				UserID:             uint(userID),
				Channel:            models.NotificationChannelEmail,
				WhatsappTargetType: models.WhatsappTargetTypePersonal,
			}
		} else {
			fmt.Printf("DB Error fetching preference for user %d: %v\n", userID, err)
			return c.String(http.StatusInternalServerError, "Error fetching preference")
		}
	}

	// Retrieve user name for display purpose
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	// Render the templ component
	return pages.UserPreferencePopup(user, pref).Render(c.Request().Context(), c.Response())
}

// UpdateUserPreference handles the form submission
func (h *UserPreferenceHandler) UpdateUserPreference(c echo.Context) error {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid user ID")
	}

	channel := c.FormValue("channel")               // "email" or "whatsapp"
	waTarget := c.FormValue("whatsapp_target_type") // "personal" or "group"
	waGroup := c.FormValue("whatsapp_group_id")

	// Upsert preference
	var pref models.UserNotifPreference
	err = h.DB.Where("user_id = ?", userID).First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			pref = models.UserNotifPreference{UserID: uint(userID)}
		} else {
			return c.String(http.StatusInternalServerError, "Database error")
		}
	}

	pref.Channel = models.NotificationChannel(channel)
	pref.WhatsappTargetType = waTarget
	pref.WhatsappGroupID = waGroup

	if err := h.DB.Save(&pref).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save preference")
	}

	// Return Success Component
	return pages.UserPreferenceSuccess().Render(c.Request().Context(), c.Response())
}
