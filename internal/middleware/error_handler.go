package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/web/templates/pages"
	"patungan_app_echo/web/templates/shared"
)

// CustomErrorHandler creates a custom error handler for Echo
func CustomErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	errorTitle := "Internal Server Error"
	errorMessage := ""

	// Check if it's an Echo HTTPError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code

		// Try to extract message from HTTPError
		if msg, ok := he.Message.(string); ok && msg != "" {
			errorMessage = msg
		}

		// Set title and default message if no custom message provided
		switch code {
		case http.StatusNotFound:
			errorTitle = "Page Not Found"
			if errorMessage == "" {
				errorMessage = "The page you're looking for doesn't exist."
			}
		case http.StatusForbidden:
			errorTitle = "Access Denied"
			if errorMessage == "" {
				errorMessage = "You don't have permission to access this resource."
			}
		case http.StatusUnauthorized:
			errorTitle = "Unauthorized"
			if errorMessage == "" {
				errorMessage = "Please log in to continue."
			}
		case http.StatusBadRequest:
			errorTitle = "Bad Request"
			if errorMessage == "" {
				errorMessage = "The request could not be processed."
			}
		default:
			if errorMessage == "" {
				errorMessage = "Something went wrong. Please try again later."
			}
		}
	} else {
		// Non-HTTPError, use default
		errorMessage = "Something went wrong. Please try again later."
	}

	// Log the error
	c.Logger().Error(err)

	// Try to get user context (may not be available for all errors)
	userEmail := ""
	userUID := ""
	if val := c.Get("userEmail"); val != nil {
		if email, ok := val.(string); ok {
			userEmail = email
		}
	}
	if val := c.Get("userUID"); val != nil {
		if uid, ok := val.(string); ok {
			userUID = uid
		}
	}

	// Build breadcrumbs
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Error", URL: ""},
	}

	// Prepare error page props
	props := pages.ErrorPageProps{
		Title:        errorTitle,
		ActiveNav:    "", // No active nav for error pages
		Breadcrumbs:  breadcrumbs,
		UserEmail:    userEmail,
		UserUID:      userUID,
		ErrorTitle:   errorTitle,
		ErrorMessage: errorMessage,
		BackLink:     "",
		BackText:     "",
	}

	// Set status code
	c.Response().Status = code

	// Check if this is a public route or authentication route
	path := c.Request().URL.Path
	isPublic := false
	if len(path) >= 3 && path[:3] == "/p/" { // Public payment pages
		isPublic = true
	} else if len(path) >= 6 && path[:6] == "/login" {
		isPublic = true
	} else if len(path) >= 5 && path[:5] == "/auth" {
		isPublic = true
	} else if len(path) >= 7 && path[:7] == "/static" {
		isPublic = true
	}

	// Try to render the error page template
	var renderErr error
	if isPublic {
		renderErr = pages.PublicErrorPage(props).Render(c.Request().Context(), c.Response())
	} else {
		renderErr = pages.ErrorPage(props).Render(c.Request().Context(), c.Response())
	}

	if renderErr != nil {
		// Fallback to plain text if template fails
		c.Logger().Error(fmt.Errorf("failed to render error page: %w", renderErr))
		c.String(code, errorMessage)
	}
}
