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
	// Get user info from context (set by auth middleware)
	userEmail, _ := c.Get("userEmail").(string)
	userUID, _ := c.Get("userUID").(string)

	data := map[string]interface{}{
		"UserEmail": userEmail,
		"UserUID":   userUID,
	}
	return c.Render(http.StatusOK, "dashboard.html", data)
}
