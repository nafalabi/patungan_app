package user_test

import (
	"errors"
	"testing"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/user"
)

type fakeUserRepo struct {
	users       map[uint]models.User
	preferences map[uint]models.UserNotifPreference
	nextID      uint

	created    []*models.User
	saved      []*models.User
	deleted    []uint
	savedPrefs []*models.UserNotifPreference
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:       map[uint]models.User{},
		preferences: map[uint]models.UserNotifPreference{},
	}
}

func (f *fakeUserRepo) List() ([]models.User, error) {
	users := make([]models.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *fakeUserRepo) FindByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return &u, nil
	}
	return nil, nil
}

func (f *fakeUserRepo) Create(u *models.User) error {
	f.nextID++
	u.ID = f.nextID
	f.users[u.ID] = *u
	f.created = append(f.created, u)
	return nil
}

func (f *fakeUserRepo) Save(u *models.User) error {
	f.users[u.ID] = *u
	f.saved = append(f.saved, u)
	return nil
}

func (f *fakeUserRepo) Delete(id uint) error {
	delete(f.users, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeUserRepo) FindPreferenceByUserID(userID uint) (*models.UserNotifPreference, error) {
	if p, ok := f.preferences[userID]; ok {
		return &p, nil
	}
	return nil, nil
}

func (f *fakeUserRepo) SavePreference(p *models.UserNotifPreference) error {
	if p.ID == 0 {
		f.nextID++
		p.ID = f.nextID
	}
	f.preferences[p.UserID] = *p
	f.savedPrefs = append(f.savedPrefs, p)
	return nil
}

func TestGetPreference_DefaultsWhenMissing(t *testing.T) {
	svc := user.NewService(newFakeUserRepo())

	pref, found, err := svc.GetPreference(42)
	if err != nil {
		t.Fatalf("GetPreference returned error: %v", err)
	}
	if found {
		t.Errorf("found = true, want false for missing preference")
	}
	if pref.Channel != models.NotificationChannelNone {
		t.Errorf("Channel = %q, want %q", pref.Channel, models.NotificationChannelNone)
	}
	if pref.WhatsappTargetType != models.WhatsappTargetTypePersonal {
		t.Errorf("WhatsappTargetType = %q, want %q", pref.WhatsappTargetType, models.WhatsappTargetTypePersonal)
	}
	if pref.UserID != 42 {
		t.Errorf("UserID = %d, want 42", pref.UserID)
	}
}

func TestGetPreference_ReturnsStoredWhenFound(t *testing.T) {
	repo := newFakeUserRepo()
	repo.preferences[7] = models.UserNotifPreference{
		ID:                 3,
		UserID:             7,
		Channel:            models.NotificationChannelWhatsapp,
		WhatsappTargetType: models.WhatsappTargetTypeGroup,
		WhatsappGroupID:    "123@g.us",
	}
	svc := user.NewService(repo)

	pref, found, err := svc.GetPreference(7)
	if err != nil {
		t.Fatalf("GetPreference returned error: %v", err)
	}
	if !found {
		t.Errorf("found = false, want true")
	}
	if pref.Channel != models.NotificationChannelWhatsapp {
		t.Errorf("Channel = %q, want %q", pref.Channel, models.NotificationChannelWhatsapp)
	}
	if pref.WhatsappTargetType != models.WhatsappTargetTypeGroup {
		t.Errorf("WhatsappTargetType = %q, want %q", pref.WhatsappTargetType, models.WhatsappTargetTypeGroup)
	}
}

func TestSavePreference_UpsertsWithoutDuplicating(t *testing.T) {
	repo := newFakeUserRepo()
	repo.preferences[9] = models.UserNotifPreference{
		ID:                 5,
		UserID:             9,
		Channel:            models.NotificationChannelEmail,
		WhatsappTargetType: models.WhatsappTargetTypePersonal,
	}
	svc := user.NewService(repo)

	incoming := models.UserNotifPreference{
		UserID:             9,
		Channel:            models.NotificationChannelWhatsapp,
		WhatsappTargetType: models.WhatsappTargetTypeGroup,
		WhatsappGroupID:    "999@g.us",
	}
	if err := svc.SavePreference(incoming); err != nil {
		t.Fatalf("SavePreference returned error: %v", err)
	}

	if len(repo.preferences) != 1 {
		t.Fatalf("preferences map has %d entries, want 1 (no duplicate insert)", len(repo.preferences))
	}
	got := repo.preferences[9]
	if got.ID != 5 {
		t.Errorf("ID = %d, want 5 (existing record updated, not replaced)", got.ID)
	}
	if got.Channel != models.NotificationChannelWhatsapp {
		t.Errorf("Channel = %q, want %q", got.Channel, models.NotificationChannelWhatsapp)
	}
	if got.WhatsappTargetType != models.WhatsappTargetTypeGroup {
		t.Errorf("WhatsappTargetType = %q, want %q", got.WhatsappTargetType, models.WhatsappTargetTypeGroup)
	}
	if got.WhatsappGroupID != "999@g.us" {
		t.Errorf("WhatsappGroupID = %q, want %q", got.WhatsappGroupID, "999@g.us")
	}
}

func TestSavePreference_InsertsWhenMissing(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewService(repo)

	incoming := models.UserNotifPreference{
		UserID:             11,
		Channel:            models.NotificationChannelEmail,
		WhatsappTargetType: models.WhatsappTargetTypePersonal,
	}
	if err := svc.SavePreference(incoming); err != nil {
		t.Fatalf("SavePreference returned error: %v", err)
	}

	if _, ok := repo.preferences[11]; !ok {
		t.Fatalf("preference for user 11 not inserted")
	}
}

func TestCreateUser_DefaultsToMember(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewService(repo)

	if err := svc.CreateUser("Budi", "budi@example.com", "628123", ""); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created %d users, want 1", len(repo.created))
	}
	if repo.created[0].UserType != models.UserTypeMember {
		t.Errorf("UserType = %q, want %q", repo.created[0].UserType, models.UserTypeMember)
	}
}

func TestCreateUser_KeepsExplicitType(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewService(repo)

	if err := svc.CreateUser("Admin", "admin@example.com", "", models.UserTypeAdmin); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if repo.created[0].UserType != models.UserTypeAdmin {
		t.Errorf("UserType = %q, want %q", repo.created[0].UserType, models.UserTypeAdmin)
	}
}

func TestUpdateUser_MissingReturnsErrNotFound(t *testing.T) {
	svc := user.NewService(newFakeUserRepo())

	err := svc.UpdateUser(99, "x", "x@example.com", "", models.UserTypeMember)
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("UpdateUser error = %v, want ErrNotFound", err)
	}
}

func TestUpdateUser_AppliesFields(t *testing.T) {
	repo := newFakeUserRepo()
	repo.users[4] = models.User{ID: 4, Name: "Old", Email: "old@example.com"}
	svc := user.NewService(repo)

	if err := svc.UpdateUser(4, "New", "new@example.com", "628999", ""); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	got := repo.users[4]
	if got.Name != "New" || got.Email != "new@example.com" || got.Phone != "628999" {
		t.Errorf("user fields not applied: %+v", got)
	}
	if got.UserType != models.UserTypeMember {
		t.Errorf("UserType = %q, want %q (empty type defaults to Member)", got.UserType, models.UserTypeMember)
	}
}

func TestGet_MissingReturnsErrNotFound(t *testing.T) {
	svc := user.NewService(newFakeUserRepo())

	if _, err := svc.Get(123); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestDeleteUser_SoftDeletes(t *testing.T) {
	repo := newFakeUserRepo()
	svc := user.NewService(repo)

	if err := svc.DeleteUser(2); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != 2 {
		t.Errorf("deleted = %v, want [2]", repo.deleted)
	}
}

func TestListUsers_MapsToSummaries(t *testing.T) {
	repo := newFakeUserRepo()
	repo.users[1] = models.User{ID: 1, Name: "A", Email: "a@example.com", Phone: "111", UserType: models.UserTypeAdmin}
	svc := user.NewService(repo)

	summaries, err := svc.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	if summaries[0].ID != 1 || summaries[0].Name != "A" || summaries[0].Email != "a@example.com" || summaries[0].Phone != "111" || summaries[0].UserType != string(models.UserTypeAdmin) {
		t.Errorf("unexpected summary: %+v", summaries[0])
	}
}
