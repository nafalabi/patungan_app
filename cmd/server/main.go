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

	payment "patungan_app_echo/internal/modules/payment"
	plan "patungan_app_echo/internal/modules/plan"
	settings "patungan_app_echo/internal/modules/settings"
	user "patungan_app_echo/internal/modules/user"
	admin_pages "patungan_app_echo/internal/pages/admin"
	member_pages "patungan_app_echo/internal/pages/member"
	public_payment "patungan_app_echo/internal/pages/public/payment"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/services/database"
	"patungan_app_echo/internal/services/email"
	"patungan_app_echo/internal/services/firebase"
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
	paymentSvc := payment.NewService(
		payment.NewGormDueRepo(db),
		payment.NewGormSessionRepo(db),
		payment.NewGatewayClient(gatewayManager),
	)

	// Initialize PlanService
	planSvc := plan.NewService(plan.NewGormPlanRepo(db), payment.NewDuesCreatorAdapter(db))

	// Initialize UserService
	userSvc := user.NewService(user.NewGormUserRepo(db))

	// Initialize SettingsService
	settingsSvc := settings.NewService(settings.NewGormSettingsRepo(db))

	// Initialize handlers
	authHandler := auth_mod.NewAuthHandler(authClient, db)

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	publicHandler := public_payment.NewPublicHandler(paymentSvc)
	e.GET("/p/:uuid", publicHandler.ShowPaymentDue)
	e.POST("/p/:uuid/initiate", publicHandler.InitiatePayment)
	e.GET("/p/:uuid/active-session", publicHandler.CheckActiveSession)
	e.GET("/p/:uuid/status", publicHandler.CheckStatus)

	// Admin routes
	adminGroup := e.Group("/admin")
	adminGroup.Use(authMiddleware.RequireAuth(authClient, db, redisCache))
	adminGroup.Use(authMiddleware.RequireAdmin())

	// Admin Dashboard + Payment + Plan + User + Settings routes (also registers the two gateway webhook callbacks on the root router)
	admin_pages.RegisterRoutes(e, adminGroup, admin_pages.Deps{Payments: paymentSvc, Plans: planSvc, Users: userSvc, Settings: settingsSvc})

	// Member routes
	memberGroup := e.Group("/member")
	memberGroup.Use(authMiddleware.RequireAuth(authClient, db, redisCache))

	// Member Dashboard + Payment + Plan routes
	member_pages.RegisterRoutes(memberGroup, member_pages.Deps{Payments: paymentSvc, Plans: planSvc})

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
