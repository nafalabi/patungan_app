package user

import (
	"errors"

	"patungan_app_echo/internal/models"
)

// ErrNotFound is returned when a user does not exist.
var ErrNotFound = errors.New("user not found")

type Service struct {
	repo UserRepo
}

func NewService(repo UserRepo) *Service {
	return &Service{repo: repo}
}

// ListUsers returns all users for the admin list page.
func (s *Service) ListUsers() ([]UserSummary, error) {
	users, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	summaries := make([]UserSummary, 0, len(users))
	for _, u := range users {
		summaries = append(summaries, mapSummary(u))
	}
	return summaries, nil
}

// Get returns a single user, ErrNotFound when missing.
func (s *Service) Get(id uint) (*UserSummary, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotFound
	}
	summary := mapSummary(*u)
	return &summary, nil
}

// CreateUser persists a new user, defaulting an empty type to Member.
func (s *Service) CreateUser(name, email, phone string, userType models.UserType) error {
	if userType == "" {
		userType = models.UserTypeMember
	}
	u := models.User{
		Name:     name,
		Email:    email,
		Phone:    phone,
		UserType: userType,
	}
	return s.repo.Create(&u)
}

// UpdateUser applies the given fields to an existing user, defaulting an
// empty type to Member. Returns ErrNotFound when the user is missing.
func (s *Service) UpdateUser(id uint, name, email, phone string, userType models.UserType) error {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrNotFound
	}

	u.Name = name
	u.Email = email
	u.Phone = phone
	u.UserType = userType
	if u.UserType == "" {
		u.UserType = models.UserTypeMember
	}

	return s.repo.Save(u)
}

// DeleteUser soft-deletes a user.
func (s *Service) DeleteUser(id uint) error {
	return s.repo.Delete(id)
}

// GetPreference returns the user's notification preference. When none is
// stored it returns the defaults (channel none, WhatsApp target personal)
// with found=false.
func (s *Service) GetPreference(userID uint) (models.UserNotifPreference, bool, error) {
	pref, err := s.repo.FindPreferenceByUserID(userID)
	if err != nil {
		return models.UserNotifPreference{}, false, err
	}
	if pref == nil {
		return defaultPreference(userID), false, nil
	}
	return *pref, true, nil
}

// SavePreference upserts the preference for the given user: an existing row
// is updated in place (keeping its identity), a missing one is inserted.
func (s *Service) SavePreference(p models.UserNotifPreference) error {
	existing, err := s.repo.FindPreferenceByUserID(p.UserID)
	if err != nil {
		return err
	}
	if existing == nil {
		return s.repo.SavePreference(&p)
	}

	existing.Channel = p.Channel
	existing.WhatsappTargetType = p.WhatsappTargetType
	existing.WhatsappGroupID = p.WhatsappGroupID
	return s.repo.SavePreference(existing)
}

func defaultPreference(userID uint) models.UserNotifPreference {
	return models.UserNotifPreference{
		UserID:             userID,
		Channel:            models.NotificationChannelNone,
		WhatsappTargetType: models.WhatsappTargetTypePersonal,
	}
}

func mapSummary(u models.User) UserSummary {
	return UserSummary{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Phone:    u.Phone,
		UserType: string(u.UserType),
	}
}
