package settings_test

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/modules/settings"
)

type fakeSettingsRepo struct {
	row   *models.Settings
	saved []*models.Settings
}

func newFakeSettingsRepo(row *models.Settings) *fakeSettingsRepo {
	return &fakeSettingsRepo{row: row}
}

func (f *fakeSettingsRepo) Get() (*models.Settings, error) {
	if f.row == nil {
		return nil, errors.New("settings row missing")
	}
	return f.row, nil
}

func (f *fakeSettingsRepo) Save(s *models.Settings) error {
	f.row = s
	f.saved = append(f.saved, s)
	return nil
}

func TestUpdate_MapsFormFields(t *testing.T) {
	repo := newFakeSettingsRepo(&models.Settings{
		Model:                gorm.Model{ID: 1},
		ActivePaymentGateway: models.PaymentGatewayMidtrans,
	})
	svc := settings.NewService(repo)

	input := settings.UpdateInput{
		ActiveGateway:        "mayar",
		MidtransMerchantID:   "M001",
		MidtransServerKey:    "srv-key",
		MidtransClientKey:    "cli-key",
		MidtransIsProduction: true,
		MayarAPIKey:          "mayar-key",
		MayarIsProduction:    true,
	}
	if err := svc.Update(input); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved %d settings, want 1", len(repo.saved))
	}
	got := repo.saved[0]
	if got.ActivePaymentGateway != models.PaymentGatewayMayar {
		t.Errorf("ActivePaymentGateway = %q, want %q", got.ActivePaymentGateway, models.PaymentGatewayMayar)
	}
	if got.MidtransMerchantID != "M001" {
		t.Errorf("MidtransMerchantID = %q, want %q", got.MidtransMerchantID, "M001")
	}
	if got.MidtransServerKey != "srv-key" {
		t.Errorf("MidtransServerKey = %q, want %q", got.MidtransServerKey, "srv-key")
	}
	if got.MidtransClientKey != "cli-key" {
		t.Errorf("MidtransClientKey = %q, want %q", got.MidtransClientKey, "cli-key")
	}
	if !got.MidtransIsProduction {
		t.Errorf("MidtransIsProduction = false, want true")
	}
	if got.MayarAPIKey != "mayar-key" {
		t.Errorf("MayarAPIKey = %q, want %q", got.MayarAPIKey, "mayar-key")
	}
	if !got.MayarIsProduction {
		t.Errorf("MayarIsProduction = false, want true")
	}
	if got.ID != 1 {
		t.Errorf("ID = %d, want 1 (existing singleton row updated, not replaced)", got.ID)
	}
}

func TestUpdate_FalseBoolsClearProductionFlags(t *testing.T) {
	repo := newFakeSettingsRepo(&models.Settings{
		Model:                gorm.Model{ID: 2},
		ActivePaymentGateway: models.PaymentGatewayMayar,
		MidtransIsProduction: true,
		MayarIsProduction:    true,
	})
	svc := settings.NewService(repo)

	input := settings.UpdateInput{ActiveGateway: "midtrans"}
	if err := svc.Update(input); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got := repo.saved[0]
	if got.MidtransIsProduction || got.MayarIsProduction {
		t.Errorf("production flags not cleared: midtrans=%v mayar=%v", got.MidtransIsProduction, got.MayarIsProduction)
	}
}

func TestUpdate_MissingRowReturnsError(t *testing.T) {
	svc := settings.NewService(newFakeSettingsRepo(nil))

	if err := svc.Update(settings.UpdateInput{ActiveGateway: "midtrans"}); err == nil {
		t.Fatalf("Update error = nil, want error when settings row is missing")
	}
}

func TestGet_MapsAllFields(t *testing.T) {
	repo := newFakeSettingsRepo(&models.Settings{
		Model:                gorm.Model{ID: 3},
		ActivePaymentGateway: models.PaymentGatewayMayar,
		MidtransMerchantID:   "M003",
		MidtransServerKey:    "srv",
		MidtransClientKey:    "cli",
		MidtransIsProduction: true,
		MayarAPIKey:          "key",
		MayarIsProduction:    true,
	})
	svc := settings.NewService(repo)

	view, err := svc.Get()
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := settings.View{
		ActiveGateway:        string(models.PaymentGatewayMayar),
		MidtransMerchantID:   "M003",
		MidtransServerKey:    "srv",
		MidtransClientKey:    "cli",
		MidtransIsProduction: true,
		MayarAPIKey:          "key",
		MayarIsProduction:    true,
	}
	if view != want {
		t.Errorf("view = %+v, want %+v", view, want)
	}
}

func TestGet_MissingRowReturnsError(t *testing.T) {
	svc := settings.NewService(newFakeSettingsRepo(nil))

	if _, err := svc.Get(); err == nil {
		t.Fatalf("Get error = nil, want error when settings row is missing")
	}
}
