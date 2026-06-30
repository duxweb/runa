package ws

import "github.com/duxweb/runa/route"

// Mount mounts a hub websocket endpoint on a route target.
func Mount(target route.Target, hub *Hub) {
	if target == nil || hub == nil {
		return
	}
	target.RouteGroup().Get("/", hub.Serve).SkipDoc()
}
