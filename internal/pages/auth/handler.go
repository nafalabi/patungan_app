package auth

import (
	"context"
	"errors"
	"net/http"
	"os"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"

	auth "patungan_app_echo/internal/modules/auth"
)

// firebaseVerifier adapts the Firebase auth client to the auth module's
// TokenVerifier port.
type firebaseVerifier struct{ c *fbauth.Client }

func (f firebaseVerifier) VerifyIDToken(ctx context.Context, token string) (*fbauth.Token, error) {
	return f.c.VerifyIDToken(ctx, token)
}

// Handler handles authentication endpoints
type Handler struct {
	svc    *auth.Service
	client *fbauth.Client
}

// NewHandler creates a new auth page handler. client may be nil when
// Firebase initialization failed; login endpoints then respond with 500.
func NewHandler(svc *auth.Service, client *fbauth.Client) *Handler {
	return &Handler{svc: svc, client: client}
}

// LoginPage renders the login page
func (h *Handler) LoginPage(c echo.Context) error {
	props := LoginProps{
		FirebaseAPIKey:     os.Getenv("FIREBASE_API_KEY"),
		FirebaseAuthDomain: os.Getenv("FIREBASE_AUTH_DOMAIN"),
		FirebaseProjectID:  os.Getenv("FIREBASE_PROJECT_ID"),
	}
	return Login(props).Render(c.Request().Context(), c.Response())
}

// HandleLogin verifies the Firebase ID token and creates a session cookie
func (h *Handler) HandleLogin(c echo.Context) error {
	if h.client == nil {
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

	tokenString := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	} else {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Invalid authorization format",
		})
	}

	// Resolve the registered user for the ID token
	if _, err := h.svc.ResolveUser(c.Request().Context(), tokenString, firebaseVerifier{h.client}); err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid token",
			})
		}
		// Unregistered users and lookup failures share the old 403 response
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "User not registered in the system",
		})
	}

	// Create Session Cookie (valid for 5 days)
	expiresIn := 5 * 24 * 60 * 60 * 1000                                                                  // 5 days in milliseconds for cookie
	cookieValue, err := h.client.SessionCookie(c.Request().Context(), tokenString, 5*24*60*60*1000000000) // 5 days in nanoseconds
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create session",
		})
	}

	// Set HTTP-Only Cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    cookieValue,
		MaxAge:   expiresIn / 1000.0, // convert ms to seconds
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
func (h *Handler) HandleLogout(c echo.Context) error {
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
