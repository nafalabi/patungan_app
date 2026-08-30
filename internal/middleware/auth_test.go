package middleware

import (
	"net/http"
	"net/http/httptest"
	"patungan_app_echo/internal/models"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequireAdmin(t *testing.T) {
	e := echo.New()

	handler := RequireAdmin()(func(c echo.Context) error {
		return c.String(http.StatusOK, "admin_ok")
	})

	t.Run("allows admin user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("userType", models.UserTypeAdmin)

		err := handler(c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
		if rec.Body.String() != "admin_ok" {
			t.Errorf("expected body 'admin_ok', got %q", rec.Body.String())
		}
	})

	t.Run("denies member user with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("userType", models.UserTypeMember)

		err := handler(c)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		httpErr, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("expected *echo.HTTPError, got %T", err)
		}
		if httpErr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", httpErr.Code)
		}
	})

	t.Run("denies missing userType with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		httpErr, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("expected *echo.HTTPError, got %T", err)
		}
		if httpErr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", httpErr.Code)
		}
	})
}
