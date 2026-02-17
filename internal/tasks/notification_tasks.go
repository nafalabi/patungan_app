package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
)

// NotificationUser represents the user in the notification payload
type NotificationUser struct {
	UserID      interface{} `json:"userId"` // Can be string or int
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phonenumber"`
}

// SendNotificationHandler handles sending notifications based on user preference
func SendNotificationHandler(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	usersBytes, err := json.Marshal(args["users"])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal users: %w", err)
	}

	var users []NotificationUser
	if err := json.Unmarshal(usersBytes, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	total := len(users)
	successCount := 0
	skippedCount := 0
	failureCount := 0
	var failures []string

	for _, user := range users {
		// Fetch preference
		var pref models.UserNotifPreference
		err := db.Where("user_id = ?", user.UserID).First(&pref).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Skip if no preference found
				log.Printf("Skipping notification for %s: no preference found", user.Username)
				skippedCount++
				continue
			}
			// Log error but continue? or mark as fail
			log.Printf("Error fetching preference for %s: %v", user.Username, err)
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: db error", user.Username))
			continue
		}

		var sendErr error
		if pref.Channel == models.NotificationChannelEmail {
			sendErr = sendEmailNotif(user, args)
		} else if pref.Channel == models.NotificationChannelWhatsapp {
			sendErr = sendWhatsappNotif(user, args, pref)
		} else {
			// Unknown channel, skip
			log.Printf("Unknown channel %s for %s", pref.Channel, user.Username)
			skippedCount++
			continue
		}

		if sendErr != nil {
			log.Printf("Failed to send notification to %s via %s: %v", user.Username, pref.Channel, sendErr)
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: %v", user.Username, sendErr))
		} else {
			successCount++
		}
	}

	result := map[string]interface{}{
		"total":   total,
		"success": successCount,
		"skipped": skippedCount,
		"failure": failureCount,
	}

	if len(failures) > 0 {
		result["errors"] = failures
	}

	return result, nil
}

// sendWhatsappNotif handles sending WhatsApp notifications
func sendWhatsappNotif(user NotificationUser, args map[string]interface{}, pref models.UserNotifPreference) error {
	notifTemplate, ok := args["notiftemplate"].(string)
	if !ok {
		return fmt.Errorf("notiftemplate is missing")
	}

	wahaService := services.NewWahaService()

	msg := replacePlaceholders(notifTemplate, user, args)

	var chatId string
	if pref.WhatsappTargetType == models.WhatsappTargetTypeGroup {
		chatId = pref.WhatsappGroupID
		if chatId == "" {
			return fmt.Errorf("group ID is empty")
		}
	} else {
		// Personal
		chatId = user.PhoneNumber
		if !strings.Contains(chatId, "@") {
			chatId = chatId + "@c.us"
		}
	}

	return wahaService.SendMessage(chatId, msg)
}

// sendEmailNotif handles sending Email notifications
func sendEmailNotif(user NotificationUser, args map[string]interface{}) error {
	notifTemplate, ok := args["notiftemplate"].(string)
	if !ok {
		return fmt.Errorf("notiftemplate is missing")
	}

	emailService := services.NewEmailService()

	// Simple subject extraction or default
	subject := "Notification"
	if sub, ok := args["subject"].(string); ok {
		subject = sub
	}

	msg := replacePlaceholders(notifTemplate, user, args)

	return emailService.SendEmail([]string{user.Email}, subject, msg)
}

func replacePlaceholders(template string, user NotificationUser, args map[string]interface{}) string {
	res := strings.ReplaceAll(template, "$name", user.Username)
	res = strings.ReplaceAll(res, "$username", user.Username)
	res = strings.ReplaceAll(res, "$email", user.Email)

	// Handle other args replacement if they exist in the top level args
	// Common replacements from args
	for k, v := range args {
		if k == "users" || k == "notiftemplate" {
			continue
		}
		strVal := fmt.Sprintf("%v", v)
		res = strings.ReplaceAll(res, "$"+k, strVal)
	}

	return res
}
