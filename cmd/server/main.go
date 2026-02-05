package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

	"patungan_app_echo/internal/handlers"
	authMiddleware "patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/services"
)

// TemplateRenderer is a custom html/template renderer for Echo
// Uses per-page template cloning to allow each page to define its own blocks
type TemplateRenderer struct {
	templates map[string]*template.Template
}

// NewTemplateRenderer creates a template renderer with per-page cloning
func NewTemplateRenderer() *TemplateRenderer {
	templates := make(map[string]*template.Template)

	// Parse base layout and partials as the foundation
	baseTemplate := template.Must(template.ParseGlob("web/templates/layouts/*.html"))
	template.Must(baseTemplate.ParseGlob("web/templates/partials/*.html"))

	// Find all page templates and clone base for each
	pages, err := filepath.Glob("web/templates/pages/*.html")
	if err != nil {
		log.Fatal(err)
	}

	for _, page := range pages {
		pageName := filepath.Base(page)
		// Clone the base template for this page
		pageTemplate := template.Must(baseTemplate.Clone())
		// Parse the page-specific template
		template.Must(pageTemplate.ParseFiles(page))
		templates[pageName] = pageTemplate
	}

	// Also parse standalone templates (like login) that don't use the base layout
	standalonePages, _ := filepath.Glob("web/templates/*.html")
	for _, page := range standalonePages {
		pageName := filepath.Base(page)
		if _, exists := templates[pageName]; !exists {
			templates[pageName] = template.Must(template.ParseFiles(page))
		}
	}

	return &TemplateRenderer{templates: templates}
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	tmpl, ok := t.templates[name]
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template not found: "+name)
	}
	// Check if this template has a "base" definition (page templates)
	// or should be rendered directly (standalone templates like login)
	if tmpl.Lookup("base") != nil {
		// Auto-inject user data from context if data is a map
		if dataMap, ok := data.(map[string]interface{}); ok {
			dataMap["UserEmail"] = c.Get("userEmail")
			dataMap["UserUID"] = c.Get("userUID")

			// Can also inject other common data here, e.g. current path for highlighting nav
			// dataMap["CurrentPath"] = c.Path()
		} else if data == nil {
			// If data is nil, initialize it with user data
			data = map[string]interface{}{
				"UserEmail": c.Get("userEmail"),
				"UserUID":   c.Get("userUID"),
			}
		}

		return tmpl.ExecuteTemplate(w, "base", data)
	}
	// Standalone template - execute directly
	return tmpl.Execute(w, data)
}

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

	// Template renderer with per-page cloning
	e.Renderer = NewTemplateRenderer()

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
