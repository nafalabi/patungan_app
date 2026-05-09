package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		printGeneralUsage()
		os.Exit(1)
	}

	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system environment")
	}

	command := os.Args[1]
	switch command {
	case "schedule":
		handleScheduleTask()
	case "waha":
		handleTestWaha()
	case "register-admin":
		handleRegisterAdmin()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printGeneralUsage()
		os.Exit(1)
	}
}

func printGeneralUsage() {
	fmt.Println("Usage: debug_cli <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  schedule         Create a manual scheduled task in the database")
	fmt.Println("  waha             Test WhatsApp (WAHA) notification service")
	fmt.Println("  register-admin   Register or update a user as an Admin")
}
