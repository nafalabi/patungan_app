package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"

	"github.com/joho/godotenv"
)

func main() {
	// defined flags
	taskName := flag.String("task_name", "", "Name of the task (mandatory)")
	argsStr := flag.String("arguments", "", "JSON arguments for the task (mandatory)")
	dueStr := flag.String("due", "", "Due date (mandatory, format: 2006-01-02 15:04)")
	taskType := flag.String("tasktype", "onetime", "Task type (optional, default: onetime)")
	recurring := flag.String("recurring", "", "Recurring interval rule (optional)")
	maxAttempt := flag.Int("max_attempt", 3, "Max attempts (optional, default: 3)")

	flag.Parse()

	// Validation
	if *taskName == "" || *argsStr == "" || *dueStr == "" {
		fmt.Println("Usage: schedule_task -task_name <name> -arguments <json_args> -due <YYYY-MM-DD HH:MM> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Init DB
	db, err := services.InitDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect DB: %v", err)
	}

	// Parse arguments JSON
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(*argsStr), &args); err != nil {
		log.Fatalf("Invalid JSON arguments: %v", err)
	}

	// Parse due date
	// Try simplified format first (Local time assumed if no timezone info, but time.Parse uses UTC for year/month/day layouts usually unless InLocation is used)
	// Actually time.Parse treats it as UTC if no tz offset.
	// Let's assume user inputs something generic, or RFC3339.
	due, err := time.Parse(time.RFC3339, *dueStr)
	if err != nil {
		// Try simple format "2006-01-02 15:04"
		// We'll parse it in Local time context for convenience? Or UTC?
		// Let's sticking to time.Parse which returns UTC for this layout is safer for server consistency unless we know user's timezone.
		// Use ParseInLocation with Local?
		due, err = time.ParseInLocation("2006-01-02 15:04", *dueStr, time.Local)
		if err != nil {
			log.Fatalf("Invalid due date format. Use '2006-01-02 15:04' (Local) or RFC3339: %v", err)
		}
	}

	// Recurring ptr
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
	fmt.Printf("Task: %s\nDue: %s\nType: %s\n", task.TaskName, task.Due, task.TaskType)
}
