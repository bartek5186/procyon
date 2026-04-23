package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type AdminKeyAuth struct {
	secretKey string
}

func NewAdminKeyAuth(secretKey string) *AdminKeyAuth {
	return &AdminKeyAuth{
		secretKey: strings.TrimSpace(secretKey),
	}
}

func (m *AdminKeyAuth) RequireAdminKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		adminKey := strings.TrimSpace(c.Request().Header.Get("X-Admin-Key"))
		if adminKey == "" || adminKey != m.secretKey {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		}

		return next(c)
	}
}
