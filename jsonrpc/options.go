package jsonrpc

import "time"

// WSConfig configures JSON-RPC over WebSocket.
type WSConfig struct {
	Origin         []string
	MaxMessageSize int64
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// WSOption configures JSON-RPC WebSocket transport.
type WSOption func(*WSConfig)

func defaultWSConfig() WSConfig {
	return WSConfig{
		Origin:         []string{"*"},
		MaxMessageSize: 1 << 20,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   10 * time.Second,
	}
}

// Origin sets accepted WebSocket origins.
func Origin(patterns ...string) WSOption {
	return func(config *WSConfig) {
		config.Origin = append([]string(nil), patterns...)
	}
}

// MaxMessageSize sets max WebSocket message size.
func MaxMessageSize(value int64) WSOption {
	return func(config *WSConfig) {
		if value > 0 {
			config.MaxMessageSize = value
		}
	}
}

// ReadTimeout sets WebSocket read timeout.
func ReadTimeout(value time.Duration) WSOption {
	return func(config *WSConfig) {
		config.ReadTimeout = value
	}
}

// WriteTimeout sets WebSocket write timeout.
func WriteTimeout(value time.Duration) WSOption {
	return func(config *WSConfig) {
		config.WriteTimeout = value
	}
}
