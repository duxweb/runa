package ws

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/errs"
)

// Client is one active websocket connection.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	id        string
	identity  Identity
	connected time.Time
	lastSeen  time.Time
	ip        string
	userAgent string
	send      chan any
	done      chan struct{}
	channels  map[string]struct{}
	mu        sync.RWMutex
	once      sync.Once
}

func newClient(hub *Hub, conn *websocket.Conn, identity Identity, ip string, userAgent string) *Client {
	if identity.ID == "" {
		identity.ID = hub.nextID()
	}
	now := core.Now()
	return &Client{
		hub:       hub,
		conn:      conn,
		id:        identity.ID,
		identity:  identity,
		connected: now,
		lastSeen:  now,
		ip:        ip,
		userAgent: userAgent,
		send:      make(chan any, hub.config.SendBuffer),
		done:      make(chan struct{}),
		channels:  make(map[string]struct{}),
	}
}

// ID returns client id.
func (client *Client) ID() string { return client.id }

// Identity returns client identity snapshot.
func (client *Client) Identity() Identity { return client.identity }

// Subscribe subscribes this client to a channel.
func (client *Client) Subscribe(channel string) error {
	return client.hub.subscribe(nil, client, channel)
}

// Unsubscribe unsubscribes this client from a channel.
func (client *Client) Unsubscribe(channel string) error {
	return client.hub.unsubscribe(client, channel)
}

// Send sends an event to this client.
func (client *Client) Send(event string, data any) error {
	return client.write(Packet{Event: event, Data: data})
}

// Close closes this client.
func (client *Client) Close(reason CloseReason) error {
	return client.close(websocket.StatusNormalClosure, string(reason))
}

func (client *Client) write(value any) error {
	select {
	case <-client.done:
		return errs.New("websocket client is closed")
	default:
	}
	select {
	case client.send <- value:
		client.hub.statsOut(value)
		return nil
	case <-client.done:
		return errs.New("websocket client is closed")
	default:
		_ = client.close(websocket.StatusPolicyViolation, "send buffer full")
		return errs.New("websocket send buffer full")
	}
}

func (client *Client) writeLoop() {
	defer client.close(websocket.StatusGoingAway, "writer stopped")
	var ticker *time.Ticker
	var ticks <-chan time.Time
	if client.hub.config.PingInterval > 0 {
		ticker = time.NewTicker(client.hub.config.PingInterval)
		ticks = ticker.C
		defer ticker.Stop()
	}
	for {
		select {
		case value := <-client.send:
			ctx, cancel := context.WithTimeout(context.Background(), client.hub.config.WriteTimeout)
			err := wsjson.Write(ctx, client.conn, value)
			cancel()
			if err != nil {
				return
			}
		case <-ticks:
			ctx, cancel := context.WithTimeout(context.Background(), client.hub.config.PongTimeout)
			err := client.conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}
			client.touch()
		case <-client.done:
			return
		}
	}
}

func (client *Client) close(status websocket.StatusCode, reason string) error {
	client.once.Do(func() {
		close(client.done)
		client.hub.unregister(client)
		if client.conn != nil {
			client.conn.CloseNow()
		}
	})
	_ = status
	_ = reason
	return nil
}

func (client *Client) touch() {
	client.mu.Lock()
	client.lastSeen = core.Now()
	info := client.infoLocked()
	client.mu.Unlock()
	_ = client.hub.config.Presence.Set(context.Background(), info)
}

func (client *Client) info() ClientInfo {
	client.mu.RLock()
	info := client.infoLocked()
	client.mu.RUnlock()
	return info
}

func (client *Client) infoLocked() ClientInfo {
	channels := sortedChannels(client.channels)
	return ClientInfo{
		ID:        client.id,
		Name:      client.identity.Name,
		Node:      client.hub.config.Node,
		Channels:  channels,
		Connected: client.connected,
		LastSeen:  client.lastSeen,
		IP:        client.ip,
		UserAgent: client.userAgent,
		Meta:      core.CloneMap(client.identity.Meta),
	}
}
