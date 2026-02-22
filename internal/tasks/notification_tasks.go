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
	"time"
)

// NotificationUser represents the user in the notification payload
type NotificationUser struct {
	UserID      interface{} `json:"userId"` // Can be string or int
	Username    string      `json:"username"`
	Email       string      `json:"email"`
	PhoneNumber string      `json:"phonenumber"`
}

// SendNotificationArgs defines the arguments for a notification task
type SendNotificationArgs struct {
	Users         []NotificationUser `json:"users"`
	NotifTemplate string             `json:"notiftemplate"`
	Subject       string             `json:"subject"`
	PlanName      string             `json:"plan_name"`
	Amount        float64            `json:"amount"`
	DueDate       string             `json:"due_date"`
}

// SendNotificationTaskDef encapsulates the notification task logic
type SendNotificationTaskDef struct{}

// TaskID returns the unique identifier for this task
func (t *SendNotificationTaskDef) TaskID() string {
	return "send_notification"
}

// CreateTask builds a ScheduledTask record for this task
func (t *SendNotificationTaskDef) CreateTask(args SendNotificationArgs) (*models.ScheduledTask, error) {
	return BuildScheduledTask(t.TaskID(), args, time.Now(), nil, models.ScheduledTaskTypeOneTime, 3)
}

// HandleExecution handles sending notifications based on user preference
func (t *SendNotificationTaskDef) HandleExecution(ctx context.Context, db *gorm.DB, args map[string]interface{}) (map[string]interface{}, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	var parsedArgs SendNotificationArgs
	if err := json.Unmarshal(argsBytes, &parsedArgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal args: %w", err)
	}

	total := len(parsedArgs.Users)
	successCount := 0
	skippedCount := 0
	failureCount := 0
	var failures []string

	for _, user := range parsedArgs.Users {
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
			sendErr = sendEmailNotif(user, parsedArgs)
		} else if pref.Channel == models.NotificationChannelWhatsapp {
			sendErr = sendWhatsappNotif(user, parsedArgs, pref)
		} else if pref.Channel == models.NotificationChannelNone {
			// Explicitly disabled, skip
			log.Printf("Notification disabled (none) for %s", user.Username)
			skippedCount++
			continue
		} else {
			// Unknown channel, skip
			log.Printf("Unsupported notification channel %s for %s", pref.Channel, user.Username)
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

// SendNotificationTask is the singleton instance of SendNotificationTaskDef
var SendNotificationTask = &SendNotificationTaskDef{}

// sendWhatsappNotif handles sending WhatsApp notifications
func sendWhatsappNotif(user NotificationUser, args SendNotificationArgs, pref models.UserNotifPreference) error {
	notifTemplate := args.NotifTemplate
	if notifTemplate == "" {
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
		if !strings.HasSuffix(chatId, "@g.us") {
			chatId = chatId + "@g.us"
		}
	} else {
		// Personal
		chatId = user.PhoneNumber
	}

	return wahaService.SendMessage(chatId, msg)
}

// sendEmailNotif handles sending Email notifications
func sendEmailNotif(user NotificationUser, args SendNotificationArgs) error {
	notifTemplate := args.NotifTemplate
	if notifTemplate == "" {
		return fmt.Errorf("notiftemplate is missing")
	}

	emailService := services.NewEmailService()

	// Simple subject extraction or default
	subject := "Notification"
	if args.Subject != "" {
		subject = args.Subject
	}

	msg := replacePlaceholders(notifTemplate, user, args)

	return emailService.SendEmail([]string{user.Email}, subject, msg)
}

func replacePlaceholders(template string, user NotificationUser, args SendNotificationArgs) string {
	res := strings.ReplaceAll(template, "$name", user.Username)
	res = strings.ReplaceAll(res, "$username", user.Username)
	res = strings.ReplaceAll(res, "$email", user.Email)

	res = strings.ReplaceAll(res, "$notiftemplate", args.NotifTemplate)
	res = strings.ReplaceAll(res, "$subject", args.Subject)
	res = strings.ReplaceAll(res, "$plan_name", args.PlanName)
	res = strings.ReplaceAll(res, "$amount", fmt.Sprintf("%v", args.Amount))
	res = strings.ReplaceAll(res, "$due_date", args.DueDate)

	return res
}
