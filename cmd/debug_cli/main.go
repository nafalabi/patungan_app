package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/database"
	"patungan_app_echo/internal/services/waha"
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
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printGeneralUsage()
		os.Exit(1)
	}
}

func printGeneralUsage() {
	fmt.Println("Usage: debug_cli <command> [arguments]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  schedule   Create a manual scheduled task in the database")
	fmt.Println("  waha       Test WhatsApp (WAHA) notification service")
}

func handleScheduleTask() {
	fs := flag.NewFlagSet("schedule", flag.ExitOnError)
	taskName := fs.String("name", "", "Name of the task (mandatory)")
	argsStr := fs.String("args", "", "JSON arguments for the task (mandatory)")
	dueStr := fs.String("due", "", "Due date (mandatory, format: 2006-01-02 15:04 or RFC3339)")
	taskType := fs.String("type", "onetime", "Task type (optional, default: onetime)")
	recurring := fs.String("recurring", "", "Recurring interval rule (optional)")
	maxAttempt := fs.Int("attempts", 3, "Max attempts (optional, default: 3)")

	fs.Parse(os.Args[2:])

	if *taskName == "" || *argsStr == "" || *dueStr == "" {
		fmt.Println("Usage: debug_cli schedule -name <name> -args <json_args> -due <date> [options]")
		fs.PrintDefaults()
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.InitDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect DB: %v", err)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(*argsStr), &args); err != nil {
		log.Fatalf("Invalid JSON arguments: %v", err)
	}

	due, err := time.Parse(time.RFC3339, *dueStr)
	if err != nil {
		due, err = time.ParseInLocation("2006-01-02 15:04", *dueStr, time.Local)
		if err != nil {
			log.Fatalf("Invalid due date format. Use '2006-01-02 15:04' (Local) or RFC3339: %v", err)
		}
	}

	var recurringPtr *string
	if *recurring != "" {
		recurringPtr = recurring
	}

	task := models.ScheduledTask{
		TaskName:          *taskName,
		Arguments:         args,
		Due:               due,
		TaskType:          models.ScheduledTaskType(*taskType),
		RecurringInterval: recurringPtr,
		MaxAttempt:        *maxAttempt,
		Status:            models.ScheduledTaskStatusActive,
	}

	if err := db.Create(&task).Error; err != nil {
		log.Fatalf("Failed to create task: %v", err)
	}

	fmt.Printf("Successfully created task ID: %d\n", task.ID)
}

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
