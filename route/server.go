package route

import (
	"time"

	"github.com/duxweb/runa/host"
)

// ServerConfig configures a route HTTP server.
type ServerConfig struct {
	Name            string
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Server creates an HTTP host unit backed by this route registry.
func (registry *Registry) Server(config ServerConfig) *host.HTTPServer {
	return host.NewHTTP(host.HTTPConfig{
		Name:            config.Name,
		Addr:            config.Addr,
		Handler:         registry.Handler(),
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		IdleTimeout:     config.IdleTimeout,
		ShutdownTimeout: config.ShutdownTimeout,
	})
}
