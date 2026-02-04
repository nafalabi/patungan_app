package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authClient *auth.Client
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(authClient *auth.Client) *AuthHandler {
	return &AuthHandler{authClient: authClient}
}

// LoginPage renders the login page
func (h *AuthHandler) LoginPage(c echo.Context) error {
	data := map[string]interface{}{
		"FirebaseAPIKey":     os.Getenv("FIREBASE_API_KEY"),
		"FirebaseAuthDomain": os.Getenv("FIREBASE_AUTH_DOMAIN"),
		"FirebaseProjectID":  os.Getenv("FIREBASE_PROJECT_ID"),
	}
	return c.Render(http.StatusOK, "login.html", data)
}

// HandleLogin verifies the Firebase ID token and creates a session cookie
func (h *AuthHandler) HandleLogin(c echo.Context) error {
	if h.authClient == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Firebase not initialized",
		})
	}

	// Get ID Token from Authorization Header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Missing authorization header",
		})
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Invalid authorization format",
		})
	}

	// Verify ID Token
	_, err := h.authClient.VerifyIDToken(c.Request().Context(), tokenString)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Invalid token",
		})
	}

	// Create Session Cookie (valid for 5 days)
	expiresIn := time.Hour * 24 * 5
	cookieValue, err := h.authClient.SessionCookie(c.Request().Context(), tokenString, expiresIn)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create session",
		})
	}

	// Set HTTP-Only Cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    cookieValue,
		MaxAge:   int(expiresIn.Seconds()),
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "production",
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]string{
		"status": "success",
	})
}

// HandleLogout clears the session cookie
func (h *AuthHandler) HandleLogout(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Path:     "/",
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]string{
		"status": "logged out",
	})
}
