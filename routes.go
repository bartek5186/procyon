package main

import (
	"net/http"
	"time"

	coreplugins "github.com/bartek5186/procyon-core/plugins"
	"github.com/labstack/echo/v4"
)

func registerPublicRoutes(e *echo.Echo, app *application) error {
	e.Static("/", "static")

	e.GET("/healthz", app.obs.HealthHandler)
	e.GET("/readyz", app.obs.ReadyHandler)
	e.GET("/info", app.obs.InfoHandler)

	e.GET("/health", app.hello.Health)
	e.GET("/hello", app.hello.Hello)

	api := e.Group("/v1")
	if app.kratosAuth != nil {
		api.Use(app.kratosAuth.RequireSession)
		api.GET("/hello", app.hello.HelloAuthenticated, app.requirePermission("*", "hello", "read"))
		securedAdmin := api.Group("/admin", app.requirePermission("*", "hello", "manage"))
		securedAdmin.GET("/hello", app.hello.HelloAdmin)
	}
	_ = api
	// procyon:api-routes

	pluginPublic := e.Group("/v1")
	var pluginAuthenticated *echo.Group
	var pluginAdmin *echo.Group
	if app.kratosAuth != nil {
		pluginAuthenticated = e.Group("/v1", app.kratosAuth.RequireSession)
		if app.rbac != nil {
			pluginAdmin = pluginAuthenticated.Group("/admin", app.requirePermission("*", "plugin_admin", "manage"))
		}
	}
	return app.plugins.RegisterRoutes(coreplugins.Routes{
		Public: pluginPublic, Authenticated: pluginAuthenticated, Admin: pluginAdmin, Require: app.requirePermission,
		Servers: []*echo.Echo{e},
	})
}

func registerAdminRoutes(e *echo.Echo, app *application) {
	e.GET("/metrics", echo.WrapHandler(app.obs.MetricsHandler()))

	if app.adminAuth != nil {
		admin := e.Group("", app.adminAuth.RequireAdminKey)
		admin.GET("/ping", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]any{
				"status": "ok",
				"auth":   "admin_key",
				"time":   time.Now().UTC().Format(time.RFC3339),
			})
		})
	}
}

func registerUploadRoutes(e *echo.Echo, app *application) {
	if app.kratosAuth != nil {
		e.Group("/upload", app.kratosAuth.RequireSession)
	}
}

func (app *application) requirePermission(dom, obj, act string) echo.MiddlewareFunc {
	if app.rbac == nil {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	return app.rbac.Require(dom, obj, act)
}
