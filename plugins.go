package main

import (
	"context"
	"encoding/json"

	coreconfig "github.com/bartek5186/procyon-core/config"
	coreplugins "github.com/bartek5186/procyon-core/plugins"
)

func pluginRegistrations() []coreplugins.Registration {
	registrations := append([]coreplugins.Registration(nil), localPluginFactories()...)
	return append(registrations, installedPluginFactories()...)
}

func loadPlugins(ctx context.Context, dependencies coreplugins.Dependencies, config coreconfig.Config) (*coreplugins.Registry, error) {
	registry, err := coreplugins.NewRegistry(pluginRegistrations())
	if err != nil {
		return nil, err
	}
	if err := registry.Instantiate(ctx, dependencies, func(registration coreplugins.Registration) json.RawMessage {
		return config.PluginConfig(registration.Name)
	}); err != nil {
		return nil, err
	}
	return registry, nil
}
