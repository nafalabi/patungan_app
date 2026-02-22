package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/internal/tasks"
)

const MaxConcurrentTasks = 10

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

	// Ticker for 1 minutes
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
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

	log.Printf("Found %d pending tasks. Processing with concurrency limit of %d", len(pendingTasks), MaxConcurrentTasks)

	var wg sync.WaitGroup
	sem := make(chan struct{}, MaxConcurrentTasks)

	for _, task := range pendingTasks {
		// Check context cancellation
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(t models.ScheduledTask) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore
			executeTask(ctx, db, t, 1)
		}(task)
	}

	wg.Wait()
	log.Println("Finished processing batch.")
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
	result, err := handler(ctx, db, task)
	duration := time.Since(startTime)
	runtimeMs := int(duration.Milliseconds())

	status := ""
	var resultData map[string]interface{}
	if err != nil {
		status = "failure"
		resultData = map[string]interface{}{"error": err.Error()}
		log.Printf("Task %s failed: %v", task.TaskName, err)
	} else {
		status = "success"
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
		if curAttempt < task.MaxAttempt {
			log.Printf("Task %s failed (Attempt %d/%d). Retrying...", task.TaskName, curAttempt, task.MaxAttempt)
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
