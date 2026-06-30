package jsonrpc

import (
	"context"
	"strings"

	runaconfig "github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
	"github.com/samber/do/v2"
)

// Provider mounts JSON-RPC HTTP and optional WebSocket endpoints.
func Provider(server *Server, options ...Option) runaprovider.Provider {
	config := defaultConfig()
	return provider{server: server, config: config, options: append([]Option(nil), options...)}
}

type provider struct {
	runaprovider.Base
	server  *Server
	config  Config
	options []Option
}

func (provider provider) Name() string { return "jsonrpc" }

func (item provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return newRegistry(), nil })
	return nil
}

func (item provider) Register(ctx runaprovider.Context) error {
	server := item.server
	if server == nil {
		server = New()
	}
	config := item.config
	if err := runaconfig.BindProvider(ctx, "jsonrpc", "", &config); err != nil {
		return err
	}
	for _, option := range item.options {
		if option != nil {
			option(&config)
		}
	}
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	registry.Add(server)
	if config.Path != "" {
		routes, err := runaprovider.Invoke[*route.Registry](ctx)
		if err != nil {
			return err
		}
		options := []MountOption{}
		if config.WSPath != "" {
			options = append(options, WebSocket(relativePath(config.Path, config.WSPath), config.WSOptions...))
		}
		Mount(routes.Group(config.Path), server, options...)
	}
	return nil
}

// Config configures JSON-RPC route mounting.
type Config struct {
	Path      string     `toml:"path"`
	WSPath    string     `toml:"ws_path"`
	WSOptions []WSOption `toml:"-"`
}

// Option configures the JSON-RPC provider.
type Option func(*Config)

func defaultConfig() Config {
	return Config{Path: "/jsonrpc"}
}

// Path sets the HTTP JSON-RPC endpoint.
func Path(value string) Option {
	return func(config *Config) {
		config.Path = value
	}
}

// WSPath sets the WebSocket JSON-RPC endpoint.
func WSPath(value string, options ...WSOption) Option {
	return func(config *Config) {
		config.WSPath = value
		config.WSOptions = append([]WSOption(nil), options...)
	}
}

func relativePath(base string, path string) string {
	base = "/" + strings.Trim(strings.TrimSpace(base), "/")
	path = "/" + strings.Trim(strings.TrimSpace(path), "/")
	if strings.HasPrefix(path, base+"/") {
		return strings.TrimPrefix(path, base)
	}
	if path == base {
		return "/"
	}
	return path
}
