package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	coreauthz "github.com/bartek5186/procyon-core/authz"
	coreevents "github.com/bartek5186/procyon-core/events"
	coreplugins "github.com/bartek5186/procyon-core/plugins"
	coreruntime "github.com/bartek5186/procyon-core/runtime"
	"github.com/bartek5186/procyon/controllers"
	"github.com/bartek5186/procyon/internal"
	appauthz "github.com/bartek5186/procyon/internal/authz"
	"github.com/bartek5186/procyon/internal/i18n"
	"github.com/bartek5186/procyon/services"
	"github.com/bartek5186/procyon/store"
)

const appVersion = "1.0.0"

var (
	migrate    bool
	configPath string
)

type application struct {
	// procyon:module-controller-fields
	hello *controllers.HelloController
}

func init() {
	flag.BoolVar(&migrate, "migrate", false, "Run DB migrations on start")
	flag.StringVar(&configPath, "config", "", "Path to runtime config file")
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := i18n.LoadTranslations(); err != nil {
		return fmt.Errorf("load translations: %w", err)
	}

	return coreruntime.Run(context.Background(), coreruntime.Options{
		Version:         appVersion,
		ConfigPath:      configPath,
		StaticDir:       "static",
		DefaultLanguage: "pl",
		Migrate:         migrate,
		PluginRegistrations: append(
			append([]coreplugins.Registration(nil), localPluginFactories()...),
			installedPluginFactories()...,
		),
		MigrateApplication: func(ctx context.Context, dependencies coreruntime.Dependencies) error {
			return internal.MigrateRun(ctx, dependencies.DB, dependencies.Config)
		},
		NewApplication: newApplication,
	})
}

func newApplication(_ context.Context, dependencies coreruntime.Dependencies) (coreruntime.Application, error) {
	appStore := store.NewAppStore(dependencies.DB, &dependencies.Config)
	appService := services.NewAppService(appStore, dependencies.Logger, dependencies.Metrics)
	app := &application{
		// procyon:module-controller-init
		hello: controllers.NewHelloController(appService, dependencies.Logger),
	}

	return coreruntime.Application{
		RegisterEvents: func(eventBus *coreevents.Bus) error {
			return registerApplicationEventHandlers(eventBus, appService)
		},
		Policies: func() []coreauthz.Policy {
			return appauthz.Policies()
		},
		RegisterRoutes: app.registerRoutes,
	}, nil
}
