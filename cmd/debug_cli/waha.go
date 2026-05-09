package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"patungan_app_echo/internal/services/waha"
)

func handleTestWaha() {
	fs := flag.NewFlagSet("waha", flag.ExitOnError)
	phone := fs.String("phone", "", "Phone number (e.g. 628123456789)")
	msg := fs.String("msg", "Test message from debug_cli", "Message body")

	fs.Parse(os.Args[2:])

	if *phone == "" {
		log.Fatal("Please provide a phone number using -phone flag")
	}

	service := waha.NewWahaService()
	chatId := *phone
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
