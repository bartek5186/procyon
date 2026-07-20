package main

import (
	"testing"

	coreruntime "github.com/bartek5186/procyon-core/runtime"
	"github.com/bartek5186/procyon/controllers"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func TestRegisterPublicRoutesUsesRuntimeGroups(t *testing.T) {
	server := echo.New()
	api := server.Group("/v1")
	authenticated := server.Group("/v1")
	routes := coreruntime.Routes{
		Public:        server,
		API:           api,
		Authenticated: authenticated,
		Require: func(_, _, _ string) echo.MiddlewareFunc {
			return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
		},
	}
	app := &application{hello: controllers.NewHelloController(nil, zap.NewNop())}

	if err := registerPublicRoutes(routes, app); err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"GET /health":         false,
		"GET /hello":          false,
		"GET /v1/hello":       false,
		"GET /v1/admin/hello": false,
	}
	for _, route := range server.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, registered := range want {
		if !registered {
			t.Errorf("route %s was not registered", route)
		}
	}
}
