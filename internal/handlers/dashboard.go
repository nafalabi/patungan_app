package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// DashboardHandler handles dashboard endpoints
type DashboardHandler struct{}

// NewDashboardHandler creates a new DashboardHandler
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// Dashboard renders the dashboard page
func (h *DashboardHandler) Dashboard(c echo.Context) error {
	// Breadcrumbs: Home > Dashboard
	breadcrumbs := []Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Dashboard", URL: ""}, // Current page
	}

	data := map[string]interface{}{
		"Title":       "Dashboard",
		"ActiveNav":   "dashboard",
		"Breadcrumbs": breadcrumbs,
	}
	return c.Render(http.StatusOK, "dashboard.html", data)
}
