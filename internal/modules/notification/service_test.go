package notification_test

import (
	"encoding/json"
	"errors"
	"testing"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/notification"
)

type fakePrefRepo struct {
	prefs map[uint]*models.UserNotifPreference
}

func (f *fakePrefRepo) FindByUserID(userID uint) (*models.UserNotifPreference, error) {
	p, ok := f.prefs[userID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

type fakeWAHAClient struct {
	failFor map[string]error
	sent    []string
}

func (f *fakeWAHAClient) SendMessage(chatID, message string) error {
	if err, ok := f.failFor[chatID]; ok {
		return err
	}
	f.sent = append(f.sent, chatID)
	return nil
}

type fakeEmailSender struct {
	sent int
}

func (f *fakeEmailSender) SendEmail(to []string, subject, body string) error {
	f.sent++
	return nil
}

type fakeTaskRepo struct {
	created []*models.ScheduledTask
}

func (f *fakeTaskRepo) CreateScheduledTask(t *models.ScheduledTask) error {
	f.created = append(f.created, t)
	return nil
}

func newTestService(prefs *fakePrefRepo, waha *fakeWAHAClient, taskRepo *fakeTaskRepo) *notification.Service {
	return notification.NewService(&fakeEmailSender{}, waha, prefs, taskRepo)
}

func TestSend_RetriesFailedUsers(t *testing.T) {
	prefs := &fakePrefRepo{prefs: map[uint]*models.UserNotifPreference{
		1: {
			UserID:             1,
			Channel:            models.NotificationChannelWhatsapp,
			WhatsappTargetType: models.WhatsappTargetTypeGroup,
			WhatsappGroupID:    "123",
		},
	}}
	waha := &fakeWAHAClient{failFor: map[string]error{
		"123@g.us": errors.New("waha down"),
	}}
	taskRepo := &fakeTaskRepo{}

	svc := newTestService(prefs, waha, taskRepo)

	args := notification.SendNotificationArgs{
		Users: []notification.NotificationUser{
			{UserID: 1, Username: "alice", PhoneNumber: "+111"},
			{UserID: 2, Username: "bob", PhoneNumber: "+222"},
		},
		NotifTemplate: "Hi $name",
		AttemptCount:  0,
	}

	result, err := svc.Send(args)
	if err != nil {
		t.Fatalf("Send returned unexpected error: %v", err)
	}

	if result["total"] != 2 {
		t.Errorf("total = %v, want 2", result["total"])
	}
	if result["success"] != 0 {
		t.Errorf("success = %v, want 0", result["success"])
	}
	if result["skipped"] != 1 {
		t.Errorf("skipped = %v, want 1", result["skipped"])
	}
	if result["failure"] != 1 {
		t.Errorf("failure = %v, want 1", result["failure"])
	}

	if len(taskRepo.created) != 1 {
		t.Fatalf("expected 1 retry task to be created, got %d", len(taskRepo.created))
	}
	retry := taskRepo.created[0]
	if retry.TaskName != "send_notification" {
		t.Errorf("retry task name = %q, want %q", retry.TaskName, "send_notification")
	}
	if retry.TaskType != models.ScheduledTaskTypeOneTime {
		t.Errorf("retry task type = %v, want one-time", retry.TaskType)
	}

	var retryArgs notification.SendNotificationArgs
	argsBytes, err := json.Marshal(retry.Arguments)
	if err != nil {
		t.Fatalf("failed to marshal retry args: %v", err)
	}
	if err := json.Unmarshal(argsBytes, &retryArgs); err != nil {
		t.Fatalf("failed to unmarshal retry args: %v", err)
	}

	if len(retryArgs.Users) != 1 {
		t.Fatalf("expected retry task to contain only failed users, got %d users", len(retryArgs.Users))
	}
	if userID(retryArgs.Users[0].UserID) != 1 {
		t.Errorf("retry task contains wrong user ID: %v", retryArgs.Users[0].UserID)
	}
	if retryArgs.AttemptCount != 1 {
		t.Errorf("retry AttemptCount = %d, want 1", retryArgs.AttemptCount)
	}
}

func TestSend_ReturnsErrorAfterMaxAttempts(t *testing.T) {
	prefs := &fakePrefRepo{prefs: map[uint]*models.UserNotifPreference{
		1: {
			UserID:             1,
			Channel:            models.NotificationChannelWhatsapp,
			WhatsappTargetType: models.WhatsappTargetTypeGroup,
			WhatsappGroupID:    "123",
		},
	}}
	waha := &fakeWAHAClient{failFor: map[string]error{
		"123@g.us": errors.New("waha down"),
	}}
	taskRepo := &fakeTaskRepo{}

	svc := newTestService(prefs, waha, taskRepo)

	args := notification.SendNotificationArgs{
		Users: []notification.NotificationUser{
			{UserID: 1, Username: "alice", PhoneNumber: "+111"},
		},
		NotifTemplate: "Hi $name",
		AttemptCount:  1,
		MaxAttempts:   1,
	}

	result, err := svc.Send(args)
	if err == nil {
		t.Fatal("expected error after max attempts, got nil")
	}
	if result == nil {
		t.Fatal("expected result map to be returned alongside error")
	}
	if result["failure"] != 1 {
		t.Errorf("failure = %v, want 1", result["failure"])
	}
	if len(taskRepo.created) != 0 {
		t.Errorf("no retry task should be created after max attempts, got %d", len(taskRepo.created))
	}
}

func userID(v interface{}) uint {
	switch x := v.(type) {
	case uint:
		return x
	case int:
		return uint(x)
	case float64:
		return uint(x)
	default:
		return 0
	}
}
