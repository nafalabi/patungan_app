package auth

import "github.com/labstack/echo/v4"

// RegisterRoutes registers the public auth routes on the root router.
func RegisterRoutes(e *echo.Echo, h *Handler) {
	e.GET("/login", h.LoginPage)
	e.POST("/auth/login", h.HandleLogin)
	e.POST("/auth/logout", h.HandleLogout)
}
