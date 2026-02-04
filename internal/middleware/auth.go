package middleware

import (
	"net/http"

	"firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"
)

// RequireAuth returns a middleware that verifies Firebase session cookies
func RequireAuth(authClient *auth.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check if Firebase is initialized
			if authClient == nil {
				return c.Redirect(http.StatusTemporaryRedirect, "/login?error=auth_not_configured")
			}

			// Get the session cookie
			cookie, err := c.Cookie("session")
			if err != nil || cookie.Value == "" {
				return c.Redirect(http.StatusTemporaryRedirect, "/login")
			}

			// Verify the session cookie
			decodedToken, err := authClient.VerifySessionCookie(c.Request().Context(), cookie.Value)
			if err != nil {
				// Invalid session, clear cookie and redirect
				clearCookie := &http.Cookie{
					Name:     "session",
					Value:    "",
					MaxAge:   -1,
					HttpOnly: true,
					Path:     "/",
				}
				c.SetCookie(clearCookie)
				return c.Redirect(http.StatusTemporaryRedirect, "/login")
			}

			// Set user info in context for downstream handlers
			c.Set("userUID", decodedToken.UID)
			if email, ok := decodedToken.Claims["email"].(string); ok {
				c.Set("userEmail", email)
			}
			if name, ok := decodedToken.Claims["name"].(string); ok {
				c.Set("userName", name)
			}

			return next(c)
		}
	}
}
