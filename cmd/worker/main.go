package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"patungan_app_echo/internal/modules/notification"
	"patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	"patungan_app_echo/internal/modules/scheduler"
	"patungan_app_echo/internal/services/database"
	"patungan_app_echo/internal/services/email"
	"patungan_app_echo/internal/services/waha"
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

	db, err := database.InitDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Task Registry
	scheduler.Initialize()
	scheduler.RegisterHandler(scheduler.LogInfoTask.TaskID(), scheduler.LogInfoTask.HandleExecution)

	// Composition root: bind the plan schedule task to the plan service
	planSvc := plan.NewService(plan.NewGormPlanRepo(db), payment.NewDuesCreatorAdapter(db))
	processTask := plan.NewProcessScheduleTask(planSvc)
	scheduler.RegisterHandler(processTask.TaskID(), processTask.HandleExecution)

	// Composition root: bind the notification task to the notification service
	notifSvc := notification.NewService(email.NewEmailService(), waha.NewWahaService(), notification.NewGormPrefRepo(db), notification.NewGormTaskRepo(db))
	notifTask := notification.NewSendNotificationTask(notifSvc)
	scheduler.RegisterHandler(notifTask.TaskID(), notifTask.HandleExecution)

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

	// Poll and execute scheduled tasks (loop lives in the scheduler module)
	scheduler.RunLoop(ctx, db)
}
