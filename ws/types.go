package ws

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

const (
	EventSubscribe   = "subscribe"
	EventUnsubscribe = "unsubscribe"
	EventPing        = "ping"
	EventPong        = "pong"
)

// Message is a client-to-server websocket message.
type Message struct {
	ID      string       `json:"id,omitempty"`
	Event   string       `json:"event"`
	Channel string       `json:"channel,omitempty"`
	Data    core.JSONRaw `json:"data,omitempty"`
	Meta    core.Map     `json:"meta,omitempty"`
}

// Response is a server-to-client websocket response.
type Response struct {
	ID      string   `json:"id,omitempty"`
	Event   string   `json:"event,omitempty"`
	OK      bool     `json:"ok"`
	Code    string   `json:"code,omitempty"`
	Message string   `json:"message,omitempty"`
	Data    any      `json:"data,omitempty"`
	Meta    core.Map `json:"meta,omitempty"`
}

// Packet is a server push packet.
type Packet struct {
	Channel string   `json:"channel,omitempty"`
	Event   string   `json:"event"`
	Data    any      `json:"data,omitempty"`
	Meta    core.Map `json:"meta,omitempty"`
	Origin  string   `json:"origin,omitempty"`
}

// Identity is the authenticated websocket client snapshot.
type Identity struct {
	ID   string
	Name string
	Meta core.Map
}

// Authenticator authenticates a websocket handshake.
type Authenticator func(ctx *route.Context) (*Identity, error)

// Handler handles one websocket message.
type Handler func(ctx *Context) error

// Middleware wraps websocket event handling.
type Middleware func(Handler) Handler

// SubscribeHook checks a subscription request.
type SubscribeHook func(ctx *Context, channel string) error

// PublishHook runs before publishing a packet.
type PublishHook func(ctx context.Context, channel string, event string, data any) error

// Broker coordinates packets across nodes.
type Broker interface {
	Publish(ctx context.Context, channel string, packet Packet) error
	Subscribe(ctx context.Context, node string, fn func(Packet) error) error
	Close(ctx context.Context) error
}

// Presence stores online client state.
type Presence interface {
	Set(ctx context.Context, client ClientInfo) error
	Remove(ctx context.Context, clientID string) error
	Clients(ctx context.Context, filter ...Filter) ([]ClientInfo, error)
	Channels(ctx context.Context) ([]ChannelInfo, error)
	Close(ctx context.Context) error
}

// Filter filters client info.
type Filter func(ClientInfo) bool

// ClientInfo describes one online client.
type ClientInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Node      string    `json:"node,omitempty"`
	Channels  []string  `json:"channels,omitempty"`
	Connected time.Time `json:"connected"`
	LastSeen  time.Time `json:"last_seen"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Meta      core.Map  `json:"meta,omitempty"`
}

// ChannelInfo describes one channel.
type ChannelInfo struct {
	Name    string `json:"name"`
	Clients int    `json:"clients"`
}

// Stats describes hub runtime counters.
type Stats struct {
	Clients     int    `json:"clients"`
	Channels    int    `json:"channels"`
	MessagesIn  uint64 `json:"messages_in"`
	MessagesOut uint64 `json:"messages_out"`
	BytesIn     uint64 `json:"bytes_in"`
	BytesOut    uint64 `json:"bytes_out"`
}

// CloseReason is a human-readable close reason.
type CloseReason string
