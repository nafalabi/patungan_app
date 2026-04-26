package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

	"patungan_app_echo/internal/handlers"
	authMiddleware "patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
	"patungan_app_echo/internal/services/payment_gateway"
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

	authClient, err := services.InitFirebase(credPath)
	if err != nil {
		log.Printf("Warning: Firebase initialization failed: %v", err)
		log.Println("Auth features will not work until valid credentials are provided")
	}

	// Initialize Database
	var db *gorm.DB
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		var err error
		db, err = services.InitDB(databaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		// Run auto-migration
		if err := services.AutoMigrate(db); err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}
		if err := db.AutoMigrate(&models.Settings{}); err != nil {
			log.Printf("Warning: Settings migration failed: %v", err)
		}

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
	var cache *services.RedisCache
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		var err error
		cache, err = services.NewRedisCache(redisURL)
		if err != nil {
			log.Printf("Warning: Redis initialization failed: %v", err)
			log.Println("Caching features will not work until Redis is available")
		}
	} else {
		log.Println("Warning: REDIS_URL not set, caching disabled")
	}

	// Initialize Payment Gateways (now handled dynamically within PaymentService)

	// Initialize Email
	emailService := services.NewEmailService()

	// Initialize WAHA
	wahaService := services.NewWahaService()

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
			c.Set("cache", cache)
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
	paymentService := services.NewPaymentService(db, gatewayManager)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authClient, db)
	dashboardHandler := handlers.NewDashboardHandler(db)
	planHandler := handlers.NewPlanHandler(db, cache)
	userHandler := handlers.NewUserHandler(db, cache)
	paymentDueHandler := handlers.NewPaymentDueHandler(db, cache, paymentService)
	userPrefHandler := handlers.NewUserPreferenceHandler(db)
	settingsHandler := handlers.NewSettingsHandler(db, gatewayManager)

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	publicHandler := handlers.NewPublicHandler(db, cache, paymentService)
	e.GET("/p/:uuid", publicHandler.ShowPaymentDue)
	e.POST("/p/:uuid/initiate", publicHandler.InitiatePayment)
	e.GET("/p/:uuid/active-session", publicHandler.CheckActiveSession)
	e.GET("/p/:uuid/status", publicHandler.CheckStatus)

	// Protected routes
	protected := e.Group("")
	protected.Use(authMiddleware.RequireAuth(authClient, db, cache))
	protected.GET("/dashboard", dashboardHandler.Dashboard)

	// Plan routes
	protected.GET("/plans", planHandler.ListPlans)
	protected.GET("/plans/create", planHandler.CreatePlanPage)
	protected.POST("/plans", planHandler.StorePlan)
	protected.GET("/plans/:id/edit", planHandler.EditPlanPage)
	protected.POST("/plans/:id/update", planHandler.UpdatePlan)
	protected.POST("/plans/:id/delete", planHandler.DeletePlan)
	protected.GET("/plans/:id/schedule-popup", planHandler.GetSchedulePopup)
	protected.POST("/plans/:id/schedule", planHandler.SchedulePlan)
	protected.POST("/plans/:id/disable-schedule", planHandler.DisableSchedulePlan)

	// User routes
	protected.GET("/users", userHandler.ListUsers)
	protected.GET("/users/create", userHandler.CreateUserPage)
	protected.POST("/users", userHandler.StoreUser)
	protected.GET("/users/:id/edit", userHandler.EditUserPage)
	protected.POST("/users/:id/update", userHandler.UpdateUser)
	protected.POST("/users/:id/delete", userHandler.DeleteUser)

	// User Preference (HTMX)
	protected.GET("/users/:id/preference", userPrefHandler.GetUserPreference)
	protected.PUT("/users/:id/preference", userPrefHandler.UpdateUserPreference)

	// Payment dues routes
	protected.GET("/payment-dues", paymentDueHandler.ListPaymentDues)
	protected.POST("/payments/initiate/:id", paymentDueHandler.InitiatePayment)
	protected.GET("/api/payments/:id/active-session", paymentDueHandler.CheckActiveSession)
	protected.GET("/payments/:id/status", paymentDueHandler.CheckPaymentStatus)
	protected.POST("/payments/:id/mark-complete", paymentDueHandler.HandleMarkAsComplete)

	// Settings routes (Admin only)
	protected.GET("/admin/settings", settingsHandler.GetSettings)
	protected.POST("/admin/settings", settingsHandler.UpdateSettings)

	// Webhook does not need auth protection, so it should be outside 'protected' group or explicitly allowed
	// However, we usually put it under public routes
	e.POST("/payments/callback/midtrans", paymentDueHandler.MidtransCallback)

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
