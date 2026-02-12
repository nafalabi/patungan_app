package handlers

import (
	"github.com/labstack/echo/v4"

	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"
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
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Dashboard", URL: ""}, // Current page
	}

	props := pages.DashboardProps{
		Title:       "Dashboard",
		ActiveNav:   "dashboard",
		Breadcrumbs: breadcrumbs,
		UserEmail:   getStringFromContext(c, "userEmail"),
		UserUID:     getStringFromContext(c, "userUID"),
	}

	return pages.Dashboard(props).Render(c.Request().Context(), c.Response())
}

// Helper to safely get string from context
func getStringFromContext(c echo.Context, key string) string {
	val := c.Get(key)
	if val == nil {
		return ""
	}
	strVal, ok := val.(string)
	if !ok {
		return ""
	}
	return strVal
}

func getUintFromContext(c echo.Context, key string) uint {
	val := c.Get(key)
	if val == nil {
		return 0
	}
	uintVal, ok := val.(uint)
	if !ok {
		return 0
	}
	return uintVal
}
