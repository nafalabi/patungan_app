package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"patungan_app_echo/internal/services"
)

// NotificationUser represents the user in the notification payload
type NotificationUser struct {
	UserID      interface{} `json:"userId"` // Can be string or int
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phonenumber"`
}

// SendWhatsappNotifHandler handles sending WhatsApp notifications
func SendWhatsappNotifHandler(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	usersBytes, err := json.Marshal(args["users"])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal users: %w", err)
	}

	var users []NotificationUser
	if err := json.Unmarshal(usersBytes, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	notifTemplate, ok := args["notiftemplate"].(string)
	if !ok {
		return nil, fmt.Errorf("notiftemplate is missing or not a string")
	}

	wahaService := services.NewWahaService()
	successCount := 0
	failureCount := 0
	var failures []string

	for _, user := range users {
		msg := replacePlaceholders(notifTemplate, user, args)

		// Ensure phone number has @c.us suffix if using Waha, or format as needed
		// Assuming Waha expects chatId format like 628123456789@c.us
		// If the input is just number, we might need to append it.
		// However, let's assume the user provides correct format or clean it up.
		chatId := user.PhoneNumber
		if !strings.Contains(chatId, "@") {
			chatId = chatId + "@c.us"
		}

		if err := wahaService.SendMessage(chatId, msg); err != nil {
			log.Printf("Failed to send WA to %s: %v", user.Username, err)
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: %v", user.Username, err))
		} else {
			successCount++
		}
	}

	result := map[string]interface{}{
		"total":   len(users),
		"success": successCount,
		"failure": failureCount,
	}

	if len(failures) > 0 {
		result["errors"] = failures
		// If all failed, return error? Or partial success is okay?
		// Usually for bulk tasks, we returning nil error but include failure details in result is better context,
		// unless we want to retry the whole batch.
		// For now let's return nil error so the task is marked as done, unless we want retry logic.
	}

	return result, nil
}

// SendEmailNotifHandler handles sending Email notifications
func SendEmailNotifHandler(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	usersBytes, err := json.Marshal(args["users"])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal users: %w", err)
	}

	var users []NotificationUser
	if err := json.Unmarshal(usersBytes, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	notifTemplate, ok := args["notiftemplate"].(string)
	if !ok {
		return nil, fmt.Errorf("notiftemplate is missing or not a string")
	}

	emailService := services.NewEmailService()
	successCount := 0
	failureCount := 0
	var failures []string

	// Simple subject extraction or default
	subject := "Notification"
	if sub, ok := args["subject"].(string); ok {
		subject = sub
	}

	for _, user := range users {
		msg := replacePlaceholders(notifTemplate, user, args)

		if err := emailService.SendEmail([]string{user.Email}, subject, msg); err != nil {
			log.Printf("Failed to send Email to %s: %v", user.Username, err)
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: %v", user.Username, err))
		} else {
			successCount++
		}
	}

	result := map[string]interface{}{
		"total":   len(users),
		"success": successCount,
		"failure": failureCount,
	}

	if len(failures) > 0 {
		result["errors"] = failures
	}

	return result, nil
}

func replacePlaceholders(template string, user NotificationUser, args map[string]interface{}) string {
	res := strings.ReplaceAll(template, "$name", user.Username)
	res = strings.ReplaceAll(res, "$username", user.Username)
	res = strings.ReplaceAll(res, "$email", user.Email)

	// Handle other args replacement if they exist in the top level args
	// The user request example showed: $planname, $paymentamount, $link
	// We can try to replace any key starting with $ from args

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
