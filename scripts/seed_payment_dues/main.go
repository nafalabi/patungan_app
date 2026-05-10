package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/database"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.InitDB(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Seeding payment dues for deep testing...")

	// 1. Get existing users and plans
	var users []models.User
	db.Find(&users)
	if len(users) < 10 {
		fmt.Println("Creating more users...")
		for i := len(users); i < 15; i++ {
			user := models.User{
				Name:  fmt.Sprintf("Tester User %d", i+1),
				Email: fmt.Sprintf("tester%d@example.com", i+1),
				Phone: fmt.Sprintf("0812345678%d", i),
			}
			db.Create(&user)
			users = append(users, user)
		}
	}

	var plans []models.Plan
	db.Find(&plans)
	if len(plans) < 5 {
		fmt.Println("Creating more plans...")
		planNames := []string{"Netflix Family", "Spotify Premium", "YouTube Premium", "Disney+ Hotstar", "Canva Pro"}
		for i := len(plans); i < len(planNames); i++ {
			plan := models.Plan{
				Name:       planNames[i],
				TotalPrice: float64(150000 + rand.Intn(100000)),
				OwnerID:    users[0].ID,
				PlanStartDate: time.Now().AddDate(0, -6, 0),
				PaymentType: "recurring",
			}
			db.Create(&plan)
			plans = append(plans, plan)
		}
	}

	// 2. Create Billing Periods for the last 6 months
	fmt.Println("Creating billing periods...")
	var periods []models.PaymentBillingPeriod
	now := time.Now()
	for i := -5; i <= 1; i++ {
		month := now.AddDate(0, i, 0)
		periodName := month.Format("January 2006")
		
		var period models.PaymentBillingPeriod
		err := db.Where("name = ?", periodName).First(&period).Error
		if err != nil {
			period = models.PaymentBillingPeriod{
				Name:    periodName,
				DueDate: time.Date(month.Year(), month.Month(), 28, 0, 0, 0, 0, time.Local),
			}
			db.Create(&period)
		}
		periods = append(periods, period)
	}

	// 3. Create Payment Dues
	fmt.Println("Generating dozens of payment dues...")
	statuses := []string{
		models.PaymentStatusPaid,
		models.PaymentStatusPending,
		models.PaymentStatusOverdue,
	}

	count := 0
	for _, period := range periods {
		for _, plan := range plans {
			// Randomly decide if this plan has dues in this period
			if rand.Float32() < 0.2 {
				continue
			}

			// Random subset of users for this plan in this period
			numUsers := 2 + rand.Intn(4)
			// Shuffle users
			shuffledUsers := make([]models.User, len(users))
			copy(shuffledUsers, users)
			rand.Shuffle(len(shuffledUsers), func(i, j int) { 
				shuffledUsers[i], shuffledUsers[j] = shuffledUsers[j], shuffledUsers[i] 
			})
			
			for i := 0; i < numUsers; i++ {
				user := shuffledUsers[i]
				
				// Calculate amount
				amount := plan.TotalPrice / float64(numUsers)
				
				status := statuses[rand.Intn(len(statuses))]
				// Future periods should be pending
				if period.DueDate.After(now) {
					status = models.PaymentStatusPending
				}

				due := models.PaymentDue{
					PlanID:                 plan.ID,
					UserID:                 user.ID,
					PaymentBillingPeriodID: period.ID,
					DueDate:                period.DueDate,
					CalculatedPayAmount:    amount,
					PaymentStatus:          status,
					UUID:                   uuid.New().String(),
					Portion:                1,
				}
				
				// Check if already exists to avoid duplicates if re-running
				var existing models.PaymentDue
				err := db.Where("plan_id = ? AND user_id = ? AND payment_billing_period_id = ?", 
					due.PlanID, due.UserID, due.PaymentBillingPeriodID).First(&existing).Error
				
				if err != nil {
					db.Create(&due)
					count++
				}
			}
		}
	}

	fmt.Printf("Seeding completed! Created %d new payment dues.\n", count)
	fmt.Println("You can now test 'Load More' and different view modes.")
}
