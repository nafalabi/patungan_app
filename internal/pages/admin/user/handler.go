package user

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"patungan_app_echo/internal/middleware"
	"patungan_app_echo/internal/models"
	user "patungan_app_echo/internal/modules/user"
	types "patungan_app_echo/internal/template/types"
)

type UserHandler struct {
	users *user.Service
}

func NewUserHandler(users *user.Service) *UserHandler {
	return &UserHandler{users: users}
}

// ListUsers renders the list of users
func (h *UserHandler) ListUsers(c echo.Context) error {
	summaries, err := h.users.ListUsers()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch users")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: "/admin/users"},
	}

	props := UsersListProps{
		Title:       "User Management",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   middleware.GetString(c, "userEmail"),
		UserUID:     middleware.GetString(c, "userUID"),
		Users:       summaries,
	}

	return UsersList(props).Render(c.Request().Context(), c.Response())
}

// CreateUserPage renders the create user form
func (h *UserHandler) CreateUserPage(c echo.Context) error {
	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: "/admin/users"},
		{Title: "Create User", URL: ""},
	}

	props := UserFormProps{
		Title:       "Create New User",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   middleware.GetString(c, "userEmail"),
		UserUID:     middleware.GetString(c, "userUID"),
		IsEdit:      false,
		User:        &user.UserSummary{},
	}

	return UserForm(props).Render(c.Request().Context(), c.Response())
}

// StoreUser handles the creation of a new user
func (h *UserHandler) StoreUser(c echo.Context) error {
	userType := models.UserType(c.FormValue("user_type"))

	if err := h.users.CreateUser(c.FormValue("name"), c.FormValue("email"), c.FormValue("phone"), userType); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

// EditUserPage renders the edit user form
func (h *UserHandler) EditUserPage(c echo.Context) error {
	id, ok := parseUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	u, err := h.users.Get(id)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "User not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch user")
	}

	breadcrumbs := []types.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: "/admin/users"},
		{Title: "Edit User", URL: ""},
	}

	props := UserFormProps{
		Title:       "Edit User",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   middleware.GetString(c, "userEmail"),
		UserUID:     middleware.GetString(c, "userUID"),
		IsEdit:      true,
		User:        u,
	}

	return UserForm(props).Render(c.Request().Context(), c.Response())
}

// UpdateUser handles updating an existing user
func (h *UserHandler) UpdateUser(c echo.Context) error {
	id, ok := parseUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	userType := models.UserType(c.FormValue("user_type"))

	if err := h.users.UpdateUser(id, c.FormValue("name"), c.FormValue("email"), c.FormValue("phone"), userType); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "User not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update user")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

// DeleteUser handles deleting a user
func (h *UserHandler) DeleteUser(c echo.Context) error {
	id, ok := parseUserID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user ID")
	}

	if err := h.users.DeleteUser(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete user")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

// GetUserPreference returns the preference modal content for HTMX
func (h *UserHandler) GetUserPreference(c echo.Context) error {
	id, ok := parseUserID(c)
	if !ok {
		return c.String(http.StatusBadRequest, "Invalid user ID")
	}

	pref, _, err := h.users.GetPreference(id)
	if err != nil {
		fmt.Printf("DB Error fetching preference for user %d: %v\n", id, err)
		return c.String(http.StatusInternalServerError, "Error fetching preference")
	}

	u, err := h.users.Get(id)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return c.String(http.StatusNotFound, "User not found")
		}
		return c.String(http.StatusInternalServerError, "Error fetching preference")
	}

	// Render the templ component
	return UserPreferencePopup(*u, pref).Render(c.Request().Context(), c.Response())
}

// UpdateUserPreference handles the form submission
func (h *UserHandler) UpdateUserPreference(c echo.Context) error {
	id, ok := parseUserID(c)
	if !ok {
		return c.String(http.StatusBadRequest, "Invalid user ID")
	}

	pref := models.UserNotifPreference{
		UserID:             id,
		Channel:            models.NotificationChannel(c.FormValue("channel")), // "email" or "whatsapp"
		WhatsappTargetType: c.FormValue("whatsapp_target_type"),                // "personal" or "group"
		WhatsappGroupID:    c.FormValue("whatsapp_group_id"),
	}

	if err := h.users.SavePreference(pref); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save preference")
	}

	// Return Success Component
	return UserPreferenceSuccess().Render(c.Request().Context(), c.Response())
}

// parseUserID parses the :id route param; ok=false maps to the caller's
// invalid/not-found behavior.
func parseUserID(c echo.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}
