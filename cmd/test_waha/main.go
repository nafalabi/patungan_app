package main

import (
	"flag"
	"log"
	"patungan_app_echo/internal/services"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	phone := flag.String("phone", "", "Phone number (e.g. 628123456789)")
	msg := flag.String("msg", "Test message from WahaService", "Message body")
	flag.Parse()

	if *phone == "" {
		log.Fatal("Please provide a phone number using -phone flag")
	}

	// Load envs
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found")
	}

	service := services.NewWahaService()

	// Format chat ID
	chatId := *phone
	// Remove non-numeric characters for safety/cleanup if needed,
	// but user might provide raw number.
	// Ensure it ends with @c.us if not present
	if !strings.HasSuffix(chatId, "@c.us") {
		chatId += "@c.us"
	}

	log.Printf("Sending message to %s: %s", chatId, *msg)

	err := service.SendMessage(chatId, *msg)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	log.Println("Message sent successfully!")
}
