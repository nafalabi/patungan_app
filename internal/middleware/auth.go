package middleware

import (
	"fmt"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"patungan_app_echo/internal/models"
	"patungan_app_echo/internal/services"
)

// RequireAuth returns a middleware that verifies Firebase session cookies
// and loads user data from the database (with caching)
func RequireAuth(authClient *auth.Client, db *gorm.DB, cache *services.RedisCache) echo.MiddlewareFunc {
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

			// Get email from token
			email, _ := decodedToken.Claims["email"].(string)
			name, _ := decodedToken.Claims["name"].(string)

			if db == nil || cache == nil {
				return c.String(http.StatusInternalServerError, "Internal server error")
			}

			// Lookup user in database by email (with caching)
			cacheKey := fmt.Sprintf("user:email:%s", email)

			// Use GetOrSet for cached lookup
			user, err := services.GetOrSet(cache, c.Request().Context(), cacheKey, 5*time.Minute, func() (models.User, error) {
				var user models.User
				err := db.Where("email = ?", email).First(&user).Error
				return user, err
			})

			if err != nil || user.ID == 0 {
				clearCookie := &http.Cookie{
					Name:     "session",
					Value:    "",
					MaxAge:   -1,
					HttpOnly: true,
					Path:     "/",
				}
				c.SetCookie(clearCookie)
				return c.Redirect(http.StatusTemporaryRedirect, "/login?error=user_not_recognized")
			}

			// Set basic user info from token
			c.Set("userUID", decodedToken.UID)
			c.Set("userEmail", email)
			c.Set("userName", name)
			c.Set("user", user)
			c.Set("userType", user.UserType)
			c.Set("userID", user.ID)

			return next(c)
		}
	}
}
