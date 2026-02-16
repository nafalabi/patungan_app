package services

import (
	"fmt"
	"net/smtp"
	"os"
)

type EmailService struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

func NewEmailService() *EmailService {
	return &EmailService{
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		user:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASS"),
		from:     os.Getenv("EMAIL_FROM"),
	}
}

func (s *EmailService) SendEmail(to []string, subject, body string) error {
	if s.host == "" || s.port == "" || s.user == "" || s.password == "" {
		return fmt.Errorf("SMTP credentials not fully configured")
	}

	auth := smtp.PlainAuth("", s.user, s.password, s.host)

	// Build the message
	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", to[0], subject, body))

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	err := smtp.SendMail(addr, auth, s.from, to, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
