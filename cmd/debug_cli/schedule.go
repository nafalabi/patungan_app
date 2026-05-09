package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/database"
)

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
