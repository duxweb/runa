package jsonrpc

// Registry stores app-scoped JSON-RPC servers.
type Registry struct {
	servers []*Server
}

func newRegistry() *Registry { return &Registry{} }

func (registry *Registry) Add(servers ...*Server) {
	if registry == nil {
		return
	}
	for _, server := range servers {
		if server != nil {
			registry.servers = append(registry.servers, server)
		}
	}
}

func (registry *Registry) Servers() []*Server {
	if registry == nil {
		return nil
	}
	return append([]*Server(nil), registry.servers...)
}
