package payment_gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MayarGateway struct {
	APIKey  string
	BaseURL string
}

type MayarConfig struct {
	APIKey       string
	IsProduction bool
}

func NewMayarGateway(config MayarConfig) *MayarGateway {
	baseURL := "https://api.mayar.club/hl/v1" // Sandbox
	if config.IsProduction {
		baseURL = "https://api.mayar.id/hl/v1" // Production
	}

	return &MayarGateway{
		APIKey:  config.APIKey,
		BaseURL: baseURL,
	}
}

type mayarCreateResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID     string `json:"id"`
		Link   string `json:"link"`
		Status string `json:"status"`
	} `json:"data"`
}

func (g *MayarGateway) CreateTransaction(req *PaymentRequest) (*PaymentResponse, error) {
	url := fmt.Sprintf("%s/payment/create", g.BaseURL)

	payload := map[string]interface{}{
		"name":         req.Customer.Name,
		"email":        req.Customer.Email,
		"amount":       req.Amount,
		"description":  fmt.Sprintf("Payment for %s", req.OrderID),
		"mobile":       "08123456789", // Placeholder, Mayar often requires mobile
		"redirect_url": req.CallbackURL,
	}

	jsonPayload, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mayar api error: %s", string(body))
	}

	var mayarResp mayarCreateResponse
	if err := json.Unmarshal(body, &mayarResp); err != nil {
		return nil, err
	}

	return &PaymentResponse{
		Token:       mayarResp.Data.ID,
		RedirectURL: mayarResp.Data.Link,
		RawResponse: body,
	}, nil
}

func (g *MayarGateway) CheckTransaction(paymentID string) (*TransactionStatusResponse, error) {
	// Mayar typically uses 'payment/{id}'
	url := fmt.Sprintf("%s/payment/%s", g.BaseURL, paymentID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Map Mayar status to generic status
	var mayarStatus struct {
		Data struct {
			Status string `json:"status"`
			Amount int64  `json:"amount"`
		} `json:"data"`
	}
	json.Unmarshal(body, &mayarStatus)

	status := StatusUnknown
	switch strings.ToLower(mayarStatus.Data.Status) {
	case "unpaid":
		status = StatusPending
	case "paid":
		status = StatusSettlement
	case "pending":
		status = StatusPending
	case "success":
		status = StatusSettlement
	case "failed":
		status = StatusFailure
	case "expired":
		status = StatusExpire
	}

	return &TransactionStatusResponse{
		TransactionStatus: status,
		OrderID:           paymentID,
		GrossAmount:       fmt.Sprintf("%d", mayarStatus.Data.Amount),
		RawResponse:       body,
	}, nil
}

func (g *MayarGateway) CancelTransaction(orderID string) error {
	return nil
}

func (g *MayarGateway) VerifyNotification(payload []byte, headers map[string]string) (bool, error) {
	return true, nil
}
