package main

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func registerPublicRoutes(e *echo.Echo, app *application) {
	e.Static("/", "static")

	e.GET(app.obsConfig.HealthPath, app.obs.HealthHandler)
	e.GET(app.obsConfig.ReadyPath, app.obs.ReadyHandler)
	e.GET(app.obsConfig.InfoPath, app.obs.InfoHandler)
	e.GET(app.obsConfig.MetricsPath, echo.WrapHandler(app.obs.MetricsHandler()))

	e.GET("/health", app.hello.Health)
	e.GET("/hello", app.hello.Hello)

	secured := e.Group("/v1", app.kratosAuth.RequireSession)
	secured.GET("/hello", app.hello.HelloAuthenticated, app.rbac.Require("hello", "read"))
	securedAdmin := secured.Group("/admin", app.rbac.Require("hello", "manage"))
	securedAdmin.GET("/hello", app.hello.HelloAdmin)
}

func registerAdminRoutes(e *echo.Echo, app *application) {
	admin := e.Group("", app.adminAuth.RequireAdminKey)
	admin.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status": "ok",
			"auth":   "admin_key",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
}

func registerUploadRoutes(e *echo.Echo, app *application) {
	e.Group("/upload", app.kratosAuth.RequireSession)
}
