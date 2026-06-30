package ws

import "time"

// Config configures a websocket hub.
type Config struct {
	Auth Authenticator `toml:"-"`

	Origin []string `toml:"origin"`

	MaxMessageSize int64         `toml:"max_message_size"`
	ReadTimeout    time.Duration `toml:"read_timeout"`
	WriteTimeout   time.Duration `toml:"write_timeout"`
	PingInterval   time.Duration `toml:"ping_interval"`
	PongTimeout    time.Duration `toml:"pong_timeout"`

	SendBuffer int      `toml:"send_buffer"`
	Broker     Broker   `toml:"-"`
	Presence   Presence `toml:"-"`
	Node       string   `toml:"node"`
}

func normalize(config Config) Config {
	if config.MaxMessageSize <= 0 {
		config.MaxMessageSize = 64 << 10
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 5 * time.Second
	}
	if config.PingInterval <= 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.PongTimeout <= 0 {
		config.PongTimeout = 10 * time.Second
	}
	if config.SendBuffer <= 0 {
		config.SendBuffer = 32
	}
	if config.Node == "" {
		config.Node = "local"
	}
	if config.Broker == nil {
		config.Broker = NewMemoryBroker()
	}
	if config.Presence == nil {
		config.Presence = NewMemoryPresence()
	}
	return config
}
