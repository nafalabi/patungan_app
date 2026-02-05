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
	"patungan_app_echo/internal/services"
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
	} else {
		log.Println("Warning: DATABASE_URL not set, database features disabled")
	}

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Static file serving
	e.Static("/static", "web/static")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authClient)
	dashboardHandler := handlers.NewDashboardHandler()
	planHandler := handlers.NewPlanHandler(db)

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	// Protected routes
	protected := e.Group("")
	protected.Use(authMiddleware.RequireAuth(authClient))
	protected.GET("/dashboard", dashboardHandler.Dashboard)

	// Plan routes
	protected.GET("/plans", planHandler.ListPlans)
	protected.GET("/plans/create", planHandler.CreatePlanPage)
	protected.POST("/plans", planHandler.StorePlan)
	protected.GET("/plans/:id/edit", planHandler.EditPlanPage)
	protected.POST("/plans/:id/update", planHandler.UpdatePlan)
	protected.POST("/plans/:id/delete", planHandler.DeletePlan)

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
