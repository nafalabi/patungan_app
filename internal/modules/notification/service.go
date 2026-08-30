package notification

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
)

// EmailSender abstracts the email delivery backend.
type EmailSender interface {
	SendEmail(to []string, subject, body string) error
}

// WAHAClient abstracts the WhatsApp delivery backend.
type WAHAClient interface {
	SendMessage(chatID, message string) error
}

// PrefRepo abstracts user notification preference lookup.
// FindByUserID returns (nil, nil) when no preference exists.
type PrefRepo interface {
	FindByUserID(userID uint) (*models.UserNotifPreference, error)
}

// TaskRepo abstracts scheduled-task persistence (used for retry rescheduling).
type TaskRepo interface {
	CreateScheduledTask(t *models.ScheduledTask) error
}

// Service sends notifications to users based on their preferences, using
// injected senders so delivery backends can be swapped or faked in tests.
type Service struct {
	email EmailSender
	waha  WAHAClient
	prefs PrefRepo
	tasks TaskRepo
}

// NewService builds a notification Service from its dependencies.
func NewService(email EmailSender, waha WAHAClient, prefs PrefRepo, tasks TaskRepo) *Service {
	return &Service{email: email, waha: waha, prefs: prefs, tasks: tasks}
}

// Send delivers notifications for the given args. Users without a preference
// (or with channel "none"/unknown) are skipped; failed users are counted and,
// while attempts remain, rescheduled as a retry task containing only the
// failed users with AttemptCount incremented. After max attempts it returns
// the result map plus an error.
// Port of SendNotificationTaskDef.HandleExecution.
func (s *Service) Send(args SendNotificationArgs) (map[string]interface{}, error) {
	total := len(args.Users)
	successCount := 0
	skippedCount := 0
	failureCount := 0
	var failures []string
	var failedUsers []NotificationUser

	for _, user := range args.Users {
		userID, ok := prefUserID(user.UserID)
		if !ok {
			log.Printf("Skipping notification for %s: invalid user ID %v", user.Username, user.UserID)
			skippedCount++
			continue
		}

		pref, err := s.prefs.FindByUserID(userID)
		if err != nil {
			log.Printf("Error fetching preference for %s: %v", user.Username, err)
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: db error", user.Username))
			failedUsers = append(failedUsers, user)
			continue
		}
		if pref == nil {
			// Skip if no preference found
			log.Printf("Skipping notification for %s: no preference found", user.Username)
			skippedCount++
			continue
		}

		var sendErr error
		if pref.Channel == models.NotificationChannelEmail {
			sendErr = s.sendEmailNotif(user, args)
		} else if pref.Channel == models.NotificationChannelWhatsapp {
			sendErr = s.sendWhatsappNotif(user, args, *pref)
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
			failedUsers = append(failedUsers, user)
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

	if failureCount > 0 {
		result["errors"] = failures

		attempt := args.AttemptCount
		maxRetries := args.MaxAttempts
		if maxRetries <= 0 {
			maxRetries = 3
		}

		if attempt < maxRetries {
			log.Printf("Partial failure: %d users failed. Rescheduling for Attempt %d", len(failedUsers), attempt+1)

			newArgs := args
			newArgs.Users = failedUsers
			newArgs.AttemptCount = attempt + 1

			// Re-schedule in 5 minutes
			nextRun := time.Now().Add(5 * time.Minute)

			newTask, err := models.BuildScheduledTask(sendNotificationTaskID, newArgs, nextRun, nil, models.ScheduledTaskTypeOneTime, maxRetries)
			if err == nil {
				if createErr := s.tasks.CreateScheduledTask(newTask); createErr != nil {
					log.Printf("Failed to create retry task: %v", createErr)
				}
			} else {
				log.Printf("Failed to create retry task: %v", err)
			}
		} else {
			log.Printf("Max attempts (%d) reached for %d failed users.", maxRetries, len(failedUsers))
			return result, fmt.Errorf("max attempts reached, failed to deliver to %d users", len(failedUsers))
		}
	}

	return result, nil
}

// sendWhatsappNotif handles sending WhatsApp notifications
func (s *Service) sendWhatsappNotif(user NotificationUser, args SendNotificationArgs, pref models.UserNotifPreference) error {
	notifTemplate := args.NotifTemplate
	if notifTemplate == "" {
		return fmt.Errorf("notiftemplate is missing")
	}

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

	return s.waha.SendMessage(chatId, msg)
}

// sendEmailNotif handles sending Email notifications
func (s *Service) sendEmailNotif(user NotificationUser, args SendNotificationArgs) error {
	notifTemplate := args.NotifTemplate
	if notifTemplate == "" {
		return fmt.Errorf("notiftemplate is missing")
	}

	// Simple subject extraction or default
	subject := "Notification"
	if args.Subject != "" {
		subject = args.Subject
	}

	msg := replacePlaceholders(notifTemplate, user, args)

	return s.email.SendEmail([]string{user.Email}, subject, msg)
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
	res = strings.ReplaceAll(res, "$paymentlink", user.PaymentLink)

	return res
}

// prefUserID normalizes the polymorphic NotificationUser.UserID (JSON numbers
// arrive as float64, DB values as uint/int) to uint. It reports false when the
// value cannot represent a user ID, which callers treat like a missing
// preference (skip), mirroring the previous behavior of querying with an
// unmatched value.
func prefUserID(v interface{}) (uint, bool) {
	switch x := v.(type) {
	case uint:
		return x, true
	case int:
		return uint(x), true
	case int64:
		return uint(x), true
	case float64:
		return uint(x), true
	case json.Number:
		id, err := x.Int64()
		return uint(id), err == nil
	case string:
		id, err := strconv.ParseUint(x, 10, 64)
		return uint(id), err == nil
	default:
		return 0, false
	}
}

type gormPrefRepo struct {
	db *gorm.DB
}

// NewGormPrefRepo returns a PrefRepo backed by the database.
func NewGormPrefRepo(db *gorm.DB) PrefRepo {
	return &gormPrefRepo{db: db}
}

func (r *gormPrefRepo) FindByUserID(userID uint) (*models.UserNotifPreference, error) {
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

type gormTaskRepo struct {
	db *gorm.DB
}

// NewGormTaskRepo returns a TaskRepo backed by the database.
func NewGormTaskRepo(db *gorm.DB) TaskRepo {
	return &gormTaskRepo{db: db}
}

func (r *gormTaskRepo) CreateScheduledTask(t *models.ScheduledTask) error {
	return r.db.Create(t).Error
}
