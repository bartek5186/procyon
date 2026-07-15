package main

import (
	coreevents "github.com/bartek5186/procyon-core/events"
	"github.com/bartek5186/procyon/services"
)

// registerApplicationEventHandlers is the composition root for application
// business logic that reacts to events published by Procyon plugins.
func registerApplicationEventHandlers(eventBus *coreevents.Bus, appService *services.AppService) error {
	_ = eventBus
	_ = appService
	// procyon:event-handlers
	return nil
}
