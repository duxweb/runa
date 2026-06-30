package console

import (
	"github.com/duxweb/runa/jsonrpc"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/ws"
)

func jsonrpcServers(app AppContext) []*jsonrpc.Server {
	registry, err := runaprovider.Invoke[*jsonrpc.Registry](app)
	if err != nil || registry == nil {
		return nil
	}
	return registry.Servers()
}

func websocketHubs(app AppContext) []*ws.Hub {
	registry, err := runaprovider.Invoke[*ws.Registry](app)
	if err != nil || registry == nil {
		return nil
	}
	return registry.Hubs()
}
