package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"patungan_app_echo/internal/handlers"
	authMiddleware "patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/services"
)

// TemplateRenderer is a custom html/template renderer for Echo
type TemplateRenderer struct {
	templates *template.Template
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
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

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Template renderer
	renderer := &TemplateRenderer{
		templates: template.Must(template.ParseGlob("web/templates/*.html")),
	}
	e.Renderer = renderer

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authClient)
	dashboardHandler := handlers.NewDashboardHandler()

	// Public routes
	e.GET("/login", authHandler.LoginPage)
	e.POST("/auth/login", authHandler.HandleLogin)
	e.POST("/auth/logout", authHandler.HandleLogout)

	// Protected routes
	protected := e.Group("")
	protected.Use(authMiddleware.RequireAuth(authClient))
	protected.GET("/dashboard", dashboardHandler.Dashboard)

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
