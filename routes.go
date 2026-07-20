package main

import coreruntime "github.com/bartek5186/procyon-core/runtime"

func (app *application) registerRoutes(routes coreruntime.Routes) error {
	return registerPublicRoutes(routes, app)
}

// registerPublicRoutes is the project-owned HTTP composition root. Standard
// health, metrics, static, authentication and plugin routes are owned by core.
func registerPublicRoutes(routes coreruntime.Routes, app *application) error {
	e := routes.Public
	e.GET("/health", app.hello.Health)
	e.GET("/hello", app.hello.Hello)

	api := routes.API
	authenticated := routes.Authenticated
	if authenticated != nil {
		authenticated.GET("/hello", app.hello.HelloAuthenticated, routes.Require("*", "hello", "read"))
		securedAdmin := authenticated.Group("/admin", routes.Require("*", "hello", "manage"))
		securedAdmin.GET("/hello", app.hello.HelloAdmin)
	}
	_ = api
	// procyon:api-routes

	return nil
}
