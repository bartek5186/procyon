package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/bartek5186/procyon/internal/apierr"
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
		if adminKey == "" || m.secretKey == "" || subtle.ConstantTimeCompare([]byte(adminKey), []byte(m.secretKey)) != 1 {
			return apierr.ReplyUnauthorized(c, "unauthorized")
		}

		return next(c)
	}
}
