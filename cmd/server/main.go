package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

	authMiddleware "patungan_app_echo/internal/middleware"

	"patungan_app_echo/internal/services/payment_gateway"

	auth_mod "patungan_app_echo/internal/modules/auth"
	"patungan_app_echo/internal/modules/dashboard"
	"patungan_app_echo/internal/modules/payment"
	"patungan_app_echo/internal/modules/plan"
	"patungan_app_echo/internal/modules/settings"
	"patungan_app_echo/internal/modules/user"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/services/database"
	"patungan_app_echo/internal/services/email"
	"patungan_app_echo/internal/services/firebase"
	"patungan_app_echo/internal/services/payment_service"
	"patungan_app_echo/internal/services/waha"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Initialize Firebase
	credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credPath == "" {
		credPath = "./firebase-service-account.json"
	}

	authClient, err := firebase.InitFirebase(credPath)
	if err != nil {
		log.Printf("Warning: Firebase initialization failed: %v", err)
		log.Println("Auth features will not work until valid credentials are provided")
	}

	// Initialize Database
	var db *gorm.DB
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		var err error
		db, err = database.InitDB(databaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		// Run auto-migration
		log.Println("Running database migrations...")
		err = db.AutoMigrate(
			&models.User{},
			&models.Plan{},
			&models.PaymentBillingPeriod{},
			&models.PaymentDue{},
			&models.UserPayment{},
			&models.Refund{},
			&models.PlanParticipant{},
			&models.ScheduledTask{},
			&models.ScheduledTaskHistory{},
			&models.PaymentCallbackHistory{},
			&models.PaymentSession{},
			&models.UserNotifPreference{},
			&models.Settings{},
		)
		if err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}
		log.Println("Database migrations completed")

		// Seed initial settings if none exist
		var count int64
		db.Model(&models.Settings{}).Count(&count)
		if count == 0 {
			db.Create(&models.Settings{
				ActivePaymentGateway: models.PaymentGatewayMidtrans,
				MidtransMerchantID:   os.Getenv("MIDTRANS_MERCHANT_ID"),
				MidtransServerKey:    os.Getenv("MIDTRANS_SERVER_KEY"),
				MidtransClientKey:    os.Getenv("MIDTRANS_CLIENT_KEY"),
				MidtransIsProduction: os.Getenv("MIDTRANS_IS_PRODUCTION") == "true",
				MayarAPIKey:          os.Getenv("MAYAR_API_KEY"),
				MayarIsProduction:    os.Getenv("MAYAR_IS_PRODUCTION") == "true",
			})
			log.Println("Initialized default settings from environment variables")
		}
	} else {
		log.Println("Warning: DATABASE_URL not set, database features disabled")
	}

	// Initialize Redis
	var redisCache *cache.RedisCache
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		var err error
		redisCache, err = cache.NewRedisCache(redisURL)
		if err != nil {
			log.Printf("Warning: Redis initialization failed: %v", err)
			log.Println("Caching features will not work until Redis is available")
		}
	} else {
		log.Println("Warning: REDIS_URL not set, caching disabled")
	}

	// Initialize Payment Gateways (now handled dynamically within PaymentService)

	// Initialize Email
	emailService := email.NewEmailService()

	// Initialize WAHA
	wahaService := waha.NewWahaService()

	// Create Echo instance
	e := echo.New()

	// Set custom error handler
	e.HTTPErrorHandler = authMiddleware.CustomErrorHandler

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Inject services into context
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("cache", redisCache)
			c.Set("db", db)
			c.Set("email", emailService)
			c.Set("waha", wahaService)
			return next(c)
		}
	})

	// Static file serving
	e.Static("/static", "web/static")

	// Initialize PaymentService
	gatewayManager := payment_gateway.NewGatewayManager(db)
	paymentService := payment_service.NewPaymentService(db, gatewayManager)

	// Initialize handlers
	authHandler := auth_mod.NewAuthHandler(authClient, db)
	dashboardHandler := dashboard.NewDashboardHandler(db)
	planHandler := plan.NewPlanHandler(db, redisCache)
	userHandler := user.NewUserHandler(db, redisCache)
	paymentDueHandler := payment.NewPaymentDueHandler(db, redisCache, paymentService)
	userPrefHandler := user.NewUserPreferenceHandler(db)
	settingsHandler := settings.NewSettingsHandler(db, gatewayManager)

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	publicHandler := payment.NewPublicHandler(db, redisCache, paymentService)
	e.GET("/p/:uuid", publicHandler.ShowPaymentDue)
	e.POST("/p/:uuid/initiate", publicHandler.InitiatePayment)
	e.GET("/p/:uuid/active-session", publicHandler.CheckActiveSession)
	e.GET("/p/:uuid/status", publicHandler.CheckStatus)

	// Protected routes
	protected := e.Group("")
	protected.Use(authMiddleware.RequireAuth(authClient, db, redisCache))
	protected.GET("/dashboard", dashboardHandler.Dashboard)

	// models.Plan routes
	protected.GET("/plans", planHandler.ListPlans)
	protected.GET("/plans/create", planHandler.CreatePlanPage)
	protected.POST("/plans", planHandler.StorePlan)
	protected.GET("/plans/:id/edit", planHandler.EditPlanPage)
	protected.POST("/plans/:id/update", planHandler.UpdatePlan)
	protected.POST("/plans/:id/delete", planHandler.DeletePlan)
	protected.GET("/plans/:id/schedule-popup", planHandler.GetSchedulePopup)
	protected.POST("/plans/:id/schedule", planHandler.SchedulePlan)
	protected.POST("/plans/:id/disable-schedule", planHandler.DisableSchedulePlan)

	// models.User routes
	protected.GET("/users", userHandler.ListUsers)
	protected.GET("/users/create", userHandler.CreateUserPage)
	protected.POST("/users", userHandler.StoreUser)
	protected.GET("/users/:id/edit", userHandler.EditUserPage)
	protected.POST("/users/:id/update", userHandler.UpdateUser)
	protected.POST("/users/:id/delete", userHandler.DeleteUser)

	// models.User Preference (HTMX)
	protected.GET("/users/:id/preference", userPrefHandler.GetUserPreference)
	protected.PUT("/users/:id/preference", userPrefHandler.UpdateUserPreference)

	// Payment dues routes
	protected.GET("/payment-dues", paymentDueHandler.ListPaymentDues)
	protected.GET("/payments/:id/status", paymentDueHandler.CheckPaymentStatus)
	protected.POST("/payments/:id/mark-complete", paymentDueHandler.HandleMarkAsComplete)

	// models.Settings routes (Admin only)
	protected.GET("/admin/settings", settingsHandler.GetSettings)
	protected.POST("/admin/settings", settingsHandler.UpdateSettings)

	// Webhook does not need auth protection, so it should be outside 'protected' group or explicitly allowed
	// However, we usually put it under public routes
	e.POST("/payments/callback/midtrans", paymentDueHandler.MidtransCallback)
	e.POST("/payments/callback/mayar", paymentDueHandler.MayarCallback)

	// Redirect root to dashboard (or login if not authenticated)
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/dashboard")
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
