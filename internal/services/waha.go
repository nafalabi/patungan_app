package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type WahaService struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewWahaService() *WahaService {
	url := os.Getenv("WAHA_BASE_URL")
	if url == "" {
		url = "http://waha:3000"
	}
	return &WahaService{
		baseURL: url,
		apiKey:  os.Getenv("WAHA_API_KEY"),
		client:  &http.Client{},
	}
}

func (s *WahaService) makeRequest(method, endpoint string, payload interface{}) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		bodyReader = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", s.baseURL, endpoint), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *WahaService) sendSeen(chatId string) error {
	return s.makeRequest("POST", "/api/sendSeen", map[string]string{
		"chatId":  chatId,
		"session": "default",
	})
}

func (s *WahaService) startTyping(chatId string) error {
	return s.makeRequest("POST", "/api/startTyping", map[string]string{
		"chatId":  chatId,
		"session": "default",
	})
}

func (s *WahaService) stopTyping(chatId string) error {
	return s.makeRequest("POST", "/api/stopTyping", map[string]string{
		"chatId":  chatId,
		"session": "default",
	})
}

func (s *WahaService) sendText(chatId, text string) error {
	return s.makeRequest("POST", "/api/sendText", map[string]string{
		"chatId":  chatId,
		"text":    text,
		"session": "default",
	})
}

// NormalizeChatID normalizes WhatsApp chat IDs by adding required suffixes and standardizing country codes
func NormalizeChatID(chatId string) string {
	chatId = strings.TrimSpace(chatId)

	// If it's already a group ID, it's correct
	if strings.HasSuffix(chatId, "@g.us") {
		return chatId
	}

	// Remove @c.us suffix temporarily if it exists for easier processing
	chatId = strings.TrimSuffix(chatId, "@c.us")

	// Standardize Indonesian numbers starting with '0' to '62'
	if strings.HasPrefix(chatId, "0") {
		chatId = "62" + strings.TrimPrefix(chatId, "0")
	}

	// Re-add required suffix
	return chatId + "@c.us"
}

// SendMessage sends a message with authentic behavior (seen -> typing -> stop typing -> send)
func (s *WahaService) SendMessage(chatId, text string) error {
	chatId = NormalizeChatID(chatId)

	// a. sendSeen request, wait for 100ms
	if err := s.sendSeen(chatId); err != nil {
		return fmt.Errorf("failed to send seen: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// b. send startTyping request, wait for 150ms
	if err := s.startTyping(chatId); err != nil {
		return fmt.Errorf("failed to start typing: %w", err)
	}
	time.Sleep(150 * time.Millisecond)

	// c. send stopTyping request, wait for 50ms
	if err := s.stopTyping(chatId); err != nil {
		return fmt.Errorf("failed to stop typing: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	// d. send sendText request
	if err := s.sendText(chatId, text); err != nil {
		return fmt.Errorf("failed to send text: %w", err)
	}

	return nil
}
