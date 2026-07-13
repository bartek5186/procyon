package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectPluginRoutesPreservesAccessModeAndGroups(t *testing.T) {
	project := t.TempDir()
	plugin := filepath.Join(project, "plugins", "payments")
	writePostmanTestFile(t, filepath.Join(project, ".procyon.json"), `{
  "modules": {
    "payment-system": {
      "kind": "go-plugin",
      "go_module": "example.com/payment-system",
      "package": "example.com/payment-system",
      "local_source": "plugins/payments"
    }
  }
}`)
	writePostmanTestFile(t, filepath.Join(plugin, "plugin.go"), `package payments

type Plugin struct{}

func (*Plugin) RegisterRoutes(routes Routes) {
	if routes.Public != nil {
		routes.Public.GET("/payments/prices/:provider", priceList)
	}
	payments := routes.Authenticated.Group("/payments")
	payments.POST("/checkout", checkout)
	routes.Admin.DELETE("/payments/:id", removePayment)
}
`)

	generator := &generator{root: project}
	routes, err := generator.collectPluginRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes = %d, want 3: %+v", len(routes), routes)
	}
	assertPluginRoute(t, routes, "GET", "/v1/payments/prices/:provider", routeAuthPublic, false)
	assertPluginRoute(t, routes, "POST", "/v1/payments/checkout", routeAuthBearer, false)
	assertPluginRoute(t, routes, "DELETE", "/v1/admin/payments/:id", routeAuthAdmin, true)
	for _, item := range routes {
		if item.Folder != "Plugins/Payment System" {
			t.Fatalf("unexpected folder %q", item.Folder)
		}
	}
}

func TestCollectPluginRoutesWithoutMetadataReturnsNoRoutes(t *testing.T) {
	routes, err := (&generator{root: t.TempDir()}).collectPluginRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("unexpected routes: %+v", routes)
	}
}

func TestCollectPluginRoutesSkipsDisabledPlugin(t *testing.T) {
	project := t.TempDir()
	writePostmanTestFile(t, filepath.Join(project, ".procyon.json"), `{
  "modules": {
    "disabled": {
      "enabled": false,
      "kind": "go-plugin",
      "local_source": "missing-but-must-not-be-resolved"
    }
  }
}`)
	routes, err := (&generator{root: project}).collectPluginRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("disabled plugin routes were generated: %+v", routes)
	}
}

func assertPluginRoute(t *testing.T, routes []route, method, path, authMode string, admin bool) {
	t.Helper()
	for _, item := range routes {
		if item.Method == method && item.Path == path {
			if item.AuthMode != authMode || item.Admin != admin {
				t.Fatalf("unexpected route metadata: %+v", item)
			}
			return
		}
	}
	t.Fatalf("missing route %s %s in %+v", method, path, routes)
}

func writePostmanTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
