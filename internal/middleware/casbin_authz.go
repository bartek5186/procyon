package middleware

import (
	"strings"

	"github.com/bartek5186/procyon/internal/apierr"
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
				return apierr.ReplyInternal(c, "authorization not configured", nil)
			}

			session, ok := SessionFromContext(c)
			if !ok || session == nil || session.Identity == nil {
				return apierr.ReplyUnauthorized(c, "unauthorized")
			}

			userID := strings.TrimSpace(session.Identity.Id)
			if userID == "" {
				return apierr.ReplyUnauthorized(c, "unauthorized")
			}

			role, err := m.authorizer.EnsureDefaultRole(c.Request().Context(), userID)
			if err != nil {
				return apierr.ReplyInternalCode(c, "authorization_sync_failed", "authorization sync failed", err)
			}

			allowed, err := m.authorizer.Can(c.Request().Context(), userID, obj, act)
			if err != nil {
				return apierr.ReplyInternalCode(c, "authorization_check_failed", "authorization check failed", err)
			}
			if !allowed {
				return apierr.ReplyForbidden(c, "forbidden")
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
