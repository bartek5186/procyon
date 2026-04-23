package middleware

import (
	"net/http"
	"strings"

	"github.com/bartek5186/procyon/internal/authz"
	"github.com/labstack/echo/v4"
)

const ContextKeyRole = "casbinRole"

type CasbinRBAC struct {
	authorizer *authz.CasbinAuthorizer
}

func NewCasbinRBAC(authorizer *authz.CasbinAuthorizer) *CasbinRBAC {
	return &CasbinRBAC{authorizer: authorizer}
}

func (m *CasbinRBAC) Require(obj, act string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if m == nil || m.authorizer == nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": "authorization not configured"})
			}

			session, ok := SessionFromContext(c)
			if !ok || session == nil || session.Identity == nil {
				return c.JSON(http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			}

			userID := strings.TrimSpace(session.Identity.Id)
			if userID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			}

			role := authz.RoleFromSession(session)
			if err := m.authorizer.EnsureUserRole(c.Request().Context(), userID, role); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": "authorization sync failed"})
			}

			allowed, err := m.authorizer.Can(c.Request().Context(), userID, obj, act)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": "authorization check failed"})
			}
			if !allowed {
				return c.JSON(http.StatusForbidden, map[string]any{"error": "forbidden"})
			}

			c.Set(ContextKeyRole, role)
			return next(c)
		}
	}
}

func RoleFromContext(c echo.Context) (string, bool) {
	role, ok := c.Get(ContextKeyRole).(string)
	return role, ok
}
