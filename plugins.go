package main

import (
	"context"
	"fmt"

	"github.com/bartek5186/procyon-core/authz"
	coreconfig "github.com/bartek5186/procyon-core/config"
	coreplugins "github.com/bartek5186/procyon-core/plugins"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func loadInstalledPlugins(ctx context.Context, db *gorm.DB, logger *zap.Logger, config coreconfig.Config) ([]coreplugins.Plugin, error) {
	registrations := installedPluginFactories()
	instances := make([]coreplugins.Plugin, 0, len(registrations))
	for _, registration := range registrations {
		if registration.Factory == nil {
			return nil, fmt.Errorf("plugin %q has no factory", registration.Name)
		}
		pluginConfig := config.PluginConfig(registration.Name)
		if len(pluginConfig) == 0 {
			pluginConfig = registration.DefaultConfig
		}
		instance, err := registration.Factory(ctx, coreplugins.Dependencies{DB: db, Logger: logger}, pluginConfig)
		if err != nil {
			return nil, fmt.Errorf("initialize plugin %s: %w", registration.Name, err)
		}
		if instance == nil {
			return nil, fmt.Errorf("plugin %s returned a nil instance", registration.Name)
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func installedPluginPolicies(instances []coreplugins.Plugin) []authz.Policy {
	var policies []authz.Policy
	for _, instance := range instances {
		policies = append(policies, instance.Policies()...)
	}
	return policies
}

func migrateInstalledPlugins(ctx context.Context, instances []coreplugins.Plugin) error {
	for _, instance := range instances {
		if err := instance.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate plugin %s: %w", instance.Name(), err)
		}
	}
	return nil
}

func shutdownInstalledPlugins(ctx context.Context, instances []coreplugins.Plugin, logger *zap.Logger) {
	for index := len(instances) - 1; index >= 0; index-- {
		if err := instances[index].Shutdown(ctx); err != nil {
			logger.Error("plugin shutdown failed", zap.String("plugin", instances[index].Name()), zap.Error(err))
		}
	}
}
