package main

import (
	"log"
	"os"
	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/database"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.InitDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Starting migration: Grouping existing PaymentDues into PaymentBillingPeriods...")

	var dues []models.PaymentDue
	// Find dues that don't have a billing period yet
	if err := db.Where("payment_billing_period_id = ? OR payment_billing_period_id IS NULL", 0).Find(&dues).Error; err != nil {
		log.Fatalf("Failed to fetch dues: %v", err)
	}

	log.Printf("Found %d dues to process", len(dues))

	// Map to keep track of created periods: planID -> dueDate -> periodID
	periodMap := make(map[uint]map[string]uint)

	count := 0
	for i := range dues {
		due := &dues[i]
		
		planID := due.PlanID
		dueDateStr := due.DueDate.Format("2006-01-02")

		if _, ok := periodMap[planID]; !ok {
			periodMap[planID] = make(map[string]uint)
		}

		periodID, ok := periodMap[planID][dueDateStr]
		if !ok {
			// Check if period already exists in DB (to be safe if script is run multiple times)
			var period models.PaymentBillingPeriod
			err := db.Where("plan_id = ? AND due_date = ?", planID, due.DueDate).First(&period).Error
			if err == nil {
				periodID = period.ID
			} else {
				// Create new period
				periodName := due.DueDate.Format("January 2006")
				newPeriod := models.PaymentBillingPeriod{
					PlanID:  planID,
					DueDate: due.DueDate,
					Name:    periodName,
				}
				if err := db.Create(&newPeriod).Error; err != nil {
					log.Printf("Failed to create period for Plan %d, Date %s: %v", planID, dueDateStr, err)
					continue
				}
				periodID = newPeriod.ID
				log.Printf("Created new period: %s for Plan %d", periodName, planID)
			}
			periodMap[planID][dueDateStr] = periodID
		}

		// Update due with period ID
		due.PaymentBillingPeriodID = periodID
		if err := db.Model(due).Update("payment_billing_period_id", periodID).Error; err != nil {
			log.Printf("Failed to update PaymentDue %d: %v", due.ID, err)
		} else {
			count++
		}
	}

	log.Printf("Migration completed. Successfully updated %d dues.", count)
}
