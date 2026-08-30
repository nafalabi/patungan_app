package settings

import (
	"patungan_app_echo/internal/models"
)

// UpdateInput carries the editable settings fields from the admin form.
type UpdateInput struct {
	ActiveGateway        string
	MidtransMerchantID   string
	MidtransServerKey    string
	MidtransClientKey    string
	MidtransIsProduction bool
	MayarAPIKey          string
	MayarIsProduction    bool
}

type Service struct {
	repo SettingsRepo
}

func NewService(repo SettingsRepo) *Service {
	return &Service{repo: repo}
}

// Get returns the current settings for the settings page.
func (s *Service) Get() (View, error) {
	settings, err := s.repo.Get()
	if err != nil {
		return View{}, err
	}
	return mapView(*settings), nil
}

// Update applies the given fields to the existing singleton settings row
// (keeping its identity) and persists it. Returns an error when the row
// is missing or the save fails.
func (s *Service) Update(input UpdateInput) error {
	settings, err := s.repo.Get()
	if err != nil {
		return err
	}

	settings.ActivePaymentGateway = models.PaymentGateway(input.ActiveGateway)
	settings.MidtransMerchantID = input.MidtransMerchantID
	settings.MidtransServerKey = input.MidtransServerKey
	settings.MidtransClientKey = input.MidtransClientKey
	settings.MidtransIsProduction = input.MidtransIsProduction
	settings.MayarAPIKey = input.MayarAPIKey
	settings.MayarIsProduction = input.MayarIsProduction

	return s.repo.Save(settings)
}

func mapView(s models.Settings) View {
	return View{
		ActiveGateway:        string(s.ActivePaymentGateway),
		MidtransMerchantID:   s.MidtransMerchantID,
		MidtransServerKey:    s.MidtransServerKey,
		MidtransClientKey:    s.MidtransClientKey,
		MidtransIsProduction: s.MidtransIsProduction,
		MayarAPIKey:          s.MayarAPIKey,
		MayarIsProduction:    s.MayarIsProduction,
	}
}
