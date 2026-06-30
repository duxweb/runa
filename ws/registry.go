package ws

// Registry stores app-scoped websocket hubs.
type Registry struct {
	hubs []*Hub
}

func newRegistry() *Registry { return &Registry{} }

func (registry *Registry) Add(hubs ...*Hub) {
	if registry == nil {
		return
	}
	for _, hub := range hubs {
		if hub != nil {
			registry.hubs = append(registry.hubs, hub)
		}
	}
}

func (registry *Registry) Hubs() []*Hub {
	if registry == nil {
		return nil
	}
	return append([]*Hub(nil), registry.hubs...)
}
