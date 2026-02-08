package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/internal/tasks"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Initialize Database
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := services.InitDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Task Registry
	tasks.Initialize()
	tasks.DefineTasks()

	log.Println("Worker started. Waiting for next tick...")

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down worker...")
		cancel()
	}()

	// Ticker for 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start? User said "in every 5 minutes it will check",
	// usually implies ticker. But useful to run once on start for debugging.
	// We'll stick to ticker for strictly complying with "in every 5 minutes".
	// But commonly we trigger one run at startup or wait for first tick.
	// Let's run once immediately for convenience/testing visibility, then tick.
	processScheduledTasks(ctx, db)

	for {
		select {
		case <-ticker.C:
			processScheduledTasks(ctx, db)
		case <-ctx.Done():
			return
		}
	}
}

func processScheduledTasks(ctx context.Context, db *gorm.DB) {
	log.Println("Checking for pending tasks...")

	var pendingTasks []models.ScheduledTask
	// status=active & due<=now
	// Note: Gorm query
	now := time.Now()
	if err := db.Where("status = ? AND due <= ?", models.ScheduledTaskStatusActive, now).Find(&pendingTasks).Error; err != nil {
		log.Printf("Error fetching pending tasks: %v", err)
		return
	}

	if len(pendingTasks) == 0 {
		log.Println("No pending tasks found.")
		return
	}

	log.Printf("Found %d pending tasks.", len(pendingTasks))

	for _, task := range pendingTasks {
		// Check context cancellation
		if ctx.Err() != nil {
			return
		}

		executeTask(ctx, db, task, 1)
	}
}

func executeTask(ctx context.Context, db *gorm.DB, task models.ScheduledTask, curAttempt int) {
	log.Printf("Processing task: %s (ID: %d)", task.TaskName, task.ID)

	// Retrieve data
	// task.TaskName, task.Arguments, task.MaxAttempt available in `task` variable

	// Inject MaxAttempt into arguments if not present
	if task.Arguments == nil {
		task.Arguments = make(map[string]interface{})
	}
	task.Arguments["max_attempt"] = task.MaxAttempt

	// Find task handle
	handler, found := tasks.GetHandler(task.TaskName)
	if !found {
		log.Printf("Task handler not found for: %s. Marking as failure.", task.TaskName)

		// Mark as failed
		updates := map[string]interface{}{
			"status": models.ScheduledTaskStatusFailure,
		}

		// Update LastRun
		now := time.Now()
		updates["last_run"] = &now

		db.Model(&task).Updates(updates)

		// Log history
		history := models.ScheduledTaskHistory{
			ScheduledTaskID: task.ID,
			TaskName:        task.TaskName,
			RunAt:           now,
			Status:          "handler_not_found",
			AttemptNumber:   1, // Start at 1?
			Arguments:       task.Arguments,
			Result:          map[string]interface{}{"error": "Handler not found"},
		}
		db.Create(&history)
		return
	}

	// Execute task
	startTime := time.Now()
	result, err := handler(ctx, db, task.Arguments)
	duration := time.Since(startTime)
	runtimeMs := int(duration.Milliseconds())

	status := "success"
	var resultData map[string]interface{}
	if err != nil {
		status = "failure"
		resultData = map[string]interface{}{"error": err.Error()}
		log.Printf("Task %s failed: %v", task.TaskName, err)
	} else {
		resultData = result
		log.Printf("Task %s completed successfully.", task.TaskName)
	}

	// Create History
	history := models.ScheduledTaskHistory{
		ScheduledTaskID: task.ID,
		TaskName:        task.TaskName,
		RunAt:           startTime,
		Runtime:         runtimeMs,
		Status:          status,
		AttemptNumber:   curAttempt,
		Arguments:       task.Arguments,
		Result:          resultData,
	}
	db.Create(&history)

	// Update ScheduledTask
	taskUpdates := map[string]interface{}{
		"last_run": &startTime,
	}

	if status != "success" {
		if curAttempt >= task.MaxAttempt {
			executeTask(ctx, db, task, curAttempt+1)
			return
		}
		taskUpdates["status"] = models.ScheduledTaskStatusFailure
	} else {
		switch task.TaskType {
		case models.ScheduledTaskTypeOneTime:
			taskUpdates["status"] = models.ScheduledTaskStatusDone
		case models.ScheduledTaskTypeRecurring:
			nextDue := task.NextDue()
			// check if the next due is a future date, to avoid the task from being executed repeatedly
			isNextDueFuture := nextDue.After(task.Due)
			if isNextDueFuture {
				taskUpdates["status"] = models.ScheduledTaskStatusActive
				taskUpdates["due"] = nextDue
			} else {
				taskUpdates["status"] = models.ScheduledTaskStatusDone
			}
		}
	}

	db.Model(&task).Updates(taskUpdates)
}
