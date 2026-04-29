package main

import (
	"net/http"
	"time"

	"github.com/bartek5186/procyon/controllers"
	"github.com/bartek5186/procyon/internal"
	mid "github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/internal/telemetry"
	"github.com/labstack/echo/v4"
)

func registerRoutes(
	e *echo.Echo,
	obsConfig internal.ObservabilityConfig,
	obs *telemetry.Manager,
	hello *controllers.HelloController,
	kratosAuth *mid.KratosAuth,
	rbac *mid.CasbinRBAC,
	adminAuth *mid.AdminKeyAuth,
) {
	e.Static("/", "static")

	e.GET(obsConfig.HealthPath, obs.HealthHandler)
	e.GET(obsConfig.ReadyPath, obs.ReadyHandler)
	e.GET(obsConfig.InfoPath, obs.InfoHandler)
	e.GET(obsConfig.MetricsPath, echo.WrapHandler(obs.MetricsHandler()))

	e.GET("/health", hello.Health)
	e.GET("/hello", hello.Hello)

	secured := e.Group("/v1", kratosAuth.RequireSession)
	secured.GET("/hello", hello.HelloAuthenticated, rbac.Require("hello", "read"))
	securedAdmin := secured.Group("/admin", rbac.Require("hello", "manage"))
	securedAdmin.GET("/hello", hello.HelloAdmin)

	admin := e.Group("/admin", adminAuth.RequireAdminKey)
	admin.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status": "ok",
			"auth":   "admin_key",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
}
