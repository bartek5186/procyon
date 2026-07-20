package main

import (
	"context"
	"fmt"

	"github.com/bartek5186/procyon-core/authz"
	coreevents "github.com/bartek5186/procyon-core/events"
	coreplugins "github.com/bartek5186/procyon-core/plugins"
	coreruntime "github.com/bartek5186/procyon-core/runtime"
	"github.com/bartek5186/procyon/controllers"
	"github.com/bartek5186/procyon/internal"
	"github.com/bartek5186/procyon/internal/i18n"
	"github.com/bartek5186/procyon/services"
	"github.com/bartek5186/procyon/store"
)

type application struct {
	// procyon:module-controller-fields
	hello *controllers.HelloController
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
		Policies: func() []authz.Policy {
			return applicationPolicies
		},
		RegisterRoutes: app.registerRoutes,
	}, nil
}
