package main

import (
	coreplugins "github.com/bartek5186/procyon-core/plugins"
	// procyon:local-plugin-imports
)

// localPluginFactories contains project-owned plugins from the top-level
// plugins directory. This file is never generated or overwritten by Procyon CLI.
func localPluginFactories() []coreplugins.Registration {
	return []coreplugins.Registration{
		// procyon:local-plugin-registrations
	}
}
