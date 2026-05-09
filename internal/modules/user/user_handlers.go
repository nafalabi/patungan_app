package user

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"patungan_app_echo/internal/models"
	user_pages "patungan_app_echo/internal/modules/user/pages"
	"patungan_app_echo/internal/services/cache"
	"patungan_app_echo/internal/shared/utils"
	shared "patungan_app_echo/web/templates/shared"
	"strconv"
)

type UserHandler struct {
	db    *gorm.DB
	cache *cache.RedisCache
}

func NewUserHandler(db *gorm.DB, cache *cache.RedisCache) *UserHandler {
	return &UserHandler{db: db, cache: cache}
}

// ListUsers renders the list of users
func (h *UserHandler) ListUsers(c echo.Context) error {
	var users []models.User
	if err := h.db.Find(&users).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch users")
	}

	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: ""},
	}

	props := user_pages.UsersListProps{
		Title:       "models.User Management",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   utils.GetStringFromContext(c, "userEmail"),
		UserUID:     utils.GetStringFromContext(c, "userUID"),
		Users:       users,
	}

	return user_pages.UsersList(props).Render(c.Request().Context(), c.Response())
}

// CreateUserPage renders the create user form
func (h *UserHandler) CreateUserPage(c echo.Context) error {
	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: "/users"},
		{Title: "Create models.User", URL: ""},
	}

	props := user_pages.UserFormProps{
		Title:       "Create New models.User",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   utils.GetStringFromContext(c, "userEmail"),
		UserUID:     utils.GetStringFromContext(c, "userUID"),
		IsEdit:      false,
	}

	return user_pages.UserForm(props).Render(c.Request().Context(), c.Response())
}

// StoreUser handles the creation of a new user
func (h *UserHandler) StoreUser(c echo.Context) error {
	user := models.User{
		Name:     c.FormValue("name"),
		Email:    c.FormValue("email"),
		Phone:    c.FormValue("phone"),
		UserType: models.UserType(c.FormValue("user_type")),
	}

	if user.UserType == "" {
		user.UserType = models.UserTypeMember
	}

	if err := h.db.Create(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
	}

	return c.Redirect(http.StatusSeeOther, "/users")
}

// EditUserPage renders the edit user form
func (h *UserHandler) EditUserPage(c echo.Context) error {
	id := c.Param("id")
	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "models.User not found")
	}

	breadcrumbs := []shared.Breadcrumb{
		{Title: "Home", URL: "/"},
		{Title: "Users", URL: "/users"},
		{Title: "Edit models.User", URL: ""},
	}

	props := user_pages.UserFormProps{
		Title:       "Edit models.User",
		ActiveNav:   "users",
		Breadcrumbs: breadcrumbs,
		UserEmail:   utils.GetStringFromContext(c, "userEmail"),
		UserUID:     utils.GetStringFromContext(c, "userUID"),
		IsEdit:      true,
		User:        user,
	}

	return user_pages.UserForm(props).Render(c.Request().Context(), c.Response())
}

// UpdateUser handles updating an existing user
func (h *UserHandler) UpdateUser(c echo.Context) error {
	id := c.Param("id")
	var user models.User
	if err := h.db.First(&user, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "models.User not found")
	}

	user.Name = c.FormValue("name")
	user.Email = c.FormValue("email")
	user.Phone = c.FormValue("phone")
	user.UserType = models.UserType(c.FormValue("user_type"))

	if err := h.db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update user")
	}

	return c.Redirect(http.StatusSeeOther, "/users")
}

// DeleteUser handles deleting a user
func (h *UserHandler) DeleteUser(c echo.Context) error {
	id := c.Param("id")
	idUint, _ := strconv.ParseUint(id, 10, 32)

	// Clear associations first
	h.db.Model(&models.User{ID: uint(idUint)}).Association("Plans").Clear()

	if err := h.db.Delete(&models.User{}, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete user")
	}
	return c.Redirect(http.StatusSeeOther, "/users")
}
