package settings

// View is the projection of the settings singleton rendered by the admin
// payment settings page.
type View struct {
	ActiveGateway        string
	MidtransMerchantID   string
	MidtransServerKey    string
	MidtransClientKey    string
	MidtransIsProduction bool
	MayarAPIKey          string
	MayarIsProduction    bool
}
