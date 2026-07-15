package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bartek5186/procyon-core/authz"
	coreconfig "github.com/bartek5186/procyon-core/config"
	coreevents "github.com/bartek5186/procyon-core/events"
	coreplugins "github.com/bartek5186/procyon-core/plugins"
)

type pluginTestPayload struct{ Value string }

const pluginTestTopic coreevents.Topic[pluginTestPayload] = "test.plugin.shared.v1"

type lifecycleTestPlugin struct{ name string }

func (p *lifecycleTestPlugin) Name() string                    { return p.name }
func (*lifecycleTestPlugin) Migrate(context.Context) error     { return nil }
func (*lifecycleTestPlugin) Policies() []authz.Policy          { return nil }
func (*lifecycleTestPlugin) RegisterRoutes(coreplugins.Routes) {}
func (*lifecycleTestPlugin) Shutdown(context.Context) error    { return nil }

func TestInstalledPluginsShareOneApplicationEventBus(t *testing.T) {
	eventBus := coreevents.New()
	received := ""
	var publisherBus *coreevents.Bus
	registrations := []coreplugins.Registration{
		{Name: "consumer", Factory: func(_ context.Context, dependencies coreplugins.Dependencies, _ json.RawMessage) (coreplugins.Plugin, error) {
			if err := coreevents.Subscribe(dependencies.Events, pluginTestTopic, "consumer.handler", func(_ context.Context, message coreevents.Message[pluginTestPayload]) error {
				received = message.Payload.Value
				return nil
			}); err != nil {
				return nil, err
			}
			return &lifecycleTestPlugin{name: "consumer"}, nil
		}},
		{Name: "publisher", Factory: func(_ context.Context, dependencies coreplugins.Dependencies, _ json.RawMessage) (coreplugins.Plugin, error) {
			publisherBus = dependencies.Events
			return &lifecycleTestPlugin{name: "publisher"}, nil
		}},
	}
	instances, err := instantiatePlugins(context.Background(), registrations, nil, nil, eventBus, coreconfig.Config{})
	if err != nil {
		t.Fatalf("instantiate plugins: %v", err)
	}
	if len(instances) != 2 || publisherBus != eventBus {
		t.Fatalf("shared bus was not passed to every plugin")
	}
	eventBus.Seal()
	if err := coreevents.Publish(context.Background(), publisherBus, pluginTestTopic, coreevents.Message[pluginTestPayload]{
		ID: "event-1", OccurredAt: time.Now().UTC(), Source: "publisher", Payload: pluginTestPayload{Value: "delivered"},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if received != "delivered" {
		t.Fatalf("received %q", received)
	}
}
