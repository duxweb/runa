package ws

import (
	"context"

	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/host"
	"github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

// Provider registers websocket hubs as host units.
func Provider(hubs ...*Hub) provider.Provider { return hubProvider{hubs: hubs} }

type hubProvider struct {
	provider.Base
	hubs []*Hub
}

func (provider hubProvider) Name() string { return "ws" }

func (item hubProvider) Init(_ context.Context, ctx provider.Context) error {
	provider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return newRegistry(), nil })
	return nil
}

func (item hubProvider) Register(ctx provider.Context) error {
	registry, err := provider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	registry.Add(item.hubs...)
	for _, hub := range item.hubs {
		if hub == nil {
			continue
		}
		var config Config
		if err := runaconfig.BindProvider(ctx, "ws", "hubs."+hub.Name(), &config); err != nil {
			return err
		}
		hub.configure(config)
		if err := ctx.RegisterHost(hostUnit{hub: hub}); err != nil {
			return err
		}
		if err := ctx.RegisterCommand(listCommand{hub: hub}, channelsCommand{hub: hub}, statsCommand{hub: hub}, kickCommand{hub: hub}); err != nil {
			return err
		}
	}
	return nil
}

type hostUnit struct{ hub *Hub }

func (unit hostUnit) Name() string                    { return unit.hub.HostName() }
func (unit hostUnit) Start(ctx context.Context) error { return unit.hub.Start(ctx) }
func (unit hostUnit) Stop(ctx context.Context) error  { return unit.hub.Stop(ctx) }
func (unit hostUnit) Status() host.Status             { return unit.hub.Status() }
