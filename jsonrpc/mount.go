package jsonrpc

import (
	"strings"

	"github.com/duxweb/runa/route"
)

// Mount mounts a JSON-RPC server on a route target.
func Mount(target route.Target, server *Server, options ...MountOption) {
	if target == nil {
		return
	}
	if server == nil {
		server = New()
	}
	config := defaultMountConfig()
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	group := target.RouteGroup()
	group.Post("/", server.HTTP()).Raw().SkipDoc()
	if config.WSPath != "" {
		group.Get(cleanMountPath(config.WSPath), server.WS(config.WSOptions...)).Raw().SkipDoc()
	}
}

// MountConfig configures JSON-RPC mounting on a route target.
type MountConfig struct {
	WSPath    string
	WSOptions []WSOption
}

// MountOption configures JSON-RPC route mounting.
type MountOption func(*MountConfig)

func defaultMountConfig() MountConfig { return MountConfig{} }

// WebSocket mounts a JSON-RPC WebSocket endpoint under target.
func WebSocket(path string, options ...WSOption) MountOption {
	return func(config *MountConfig) {
		config.WSPath = path
		config.WSOptions = append([]WSOption(nil), options...)
	}
}

func cleanMountPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
