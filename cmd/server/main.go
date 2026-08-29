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
	admin_dashboard "patungan_app_echo/internal/modules/admin/dashboard"
	admin_payment "patungan_app_echo/internal/modules/admin/payment"
	admin_plan "patungan_app_echo/internal/modules/admin/plan"
	admin_settings "patungan_app_echo/internal/modules/admin/settings"
	admin_user "patungan_app_echo/internal/modules/admin/user"
	public_payment "patungan_app_echo/internal/modules/payment"

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
	adminDashboardHandler := admin_dashboard.NewDashboardHandler(db)
	adminPlanHandler := admin_plan.NewPlanHandler(db, redisCache)
	adminUserHandler := admin_user.NewUserHandler(db, redisCache)
	adminPaymentDueHandler := admin_payment.NewPaymentDueHandler(db, redisCache, paymentService)
	adminUserPrefHandler := admin_user.NewUserPreferenceHandler(db)
	adminSettingsHandler := admin_settings.NewSettingsHandler(db, gatewayManager)

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	publicHandler := public_payment.NewPublicHandler(db, redisCache, paymentService)
	e.GET("/p/:uuid", publicHandler.ShowPaymentDue)
	e.POST("/p/:uuid/initiate", publicHandler.InitiatePayment)
	e.GET("/p/:uuid/active-session", publicHandler.CheckActiveSession)
	e.GET("/p/:uuid/status", publicHandler.CheckStatus)

	// Admin routes
	adminGroup := e.Group("/admin")
	adminGroup.Use(authMiddleware.RequireAuth(authClient, db, redisCache))
	adminGroup.Use(authMiddleware.RequireAdmin())

	adminGroup.GET("/dashboard", adminDashboardHandler.Dashboard)

	// Admin Plan routes
	adminGroup.GET("/plans", adminPlanHandler.ListPlans)
	adminGroup.GET("/plans/create", adminPlanHandler.CreatePlanPage)
	adminGroup.POST("/plans", adminPlanHandler.StorePlan)
	adminGroup.GET("/plans/:id/edit", adminPlanHandler.EditPlanPage)
	adminGroup.POST("/plans/:id/update", adminPlanHandler.UpdatePlan)
	adminGroup.POST("/plans/:id/delete", adminPlanHandler.DeletePlan)
	adminGroup.GET("/plans/:id/schedule-popup", adminPlanHandler.GetSchedulePopup)
	adminGroup.POST("/plans/:id/schedule", adminPlanHandler.SchedulePlan)
	adminGroup.POST("/plans/:id/disable-schedule", adminPlanHandler.DisableSchedulePlan)

	// Admin User routes
	adminGroup.GET("/users", adminUserHandler.ListUsers)
	adminGroup.GET("/users/create", adminUserHandler.CreateUserPage)
	adminGroup.POST("/users", adminUserHandler.StoreUser)
	adminGroup.GET("/users/:id/edit", adminUserHandler.EditUserPage)
	adminGroup.POST("/users/:id/update", adminUserHandler.UpdateUser)
	adminGroup.POST("/users/:id/delete", adminUserHandler.DeleteUser)

	// Admin User Preference (HTMX)
	adminGroup.GET("/users/:id/preference", adminUserPrefHandler.GetUserPreference)
	adminGroup.PUT("/users/:id/preference", adminUserPrefHandler.UpdateUserPreference)

	// Admin Payment dues routes
	adminGroup.GET("/payment-dues", adminPaymentDueHandler.ListPaymentDues)
	adminGroup.GET("/payments/:id/status", adminPaymentDueHandler.CheckPaymentStatus)
	adminGroup.POST("/payments/:id/mark-complete", adminPaymentDueHandler.HandleMarkAsComplete)

	// Admin Settings routes
	adminGroup.GET("/settings", adminSettingsHandler.GetSettings)
	adminGroup.POST("/settings", adminSettingsHandler.UpdateSettings)

	// Webhooks
	e.POST("/payments/callback/midtrans", adminPaymentDueHandler.MidtransCallback)
	e.POST("/payments/callback/mayar", adminPaymentDueHandler.MayarCallback)

	// Redirect root to role-based dashboard
	e.GET("/", func(c echo.Context) error {
		userType, ok := c.Get("userType").(models.UserType)
		if ok && userType == models.UserTypeAdmin {
			return c.Redirect(http.StatusTemporaryRedirect, "/admin/dashboard")
		}
		return c.Redirect(http.StatusTemporaryRedirect, "/member/dashboard")
	}, authMiddleware.RequireAuth(authClient, db, redisCache))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
